package tasks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"projupdater/utils"
	"sort"
	"strings"
	"time"
)

const (
	// MaxFileCount 是允许处理的最大文件数量，超过此数量将跳过索引生成
	MaxFileCount = 100000
	// FindTimeout 是 find 命令的超时时间
	FindTimeout = 5 * time.Minute
)

func GenerateFileIndex() error {
	targetDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %v", err)
	}

	fmt.Printf("INFO: 在目录 %s 中生成文件索引\n", targetDir)

	filesProjPath := filepath.Join(targetDir, "files.proj")
	if _, err := os.Stat(filesProjPath); os.IsNotExist(err) {
		return fmt.Errorf("当前目录下不存在 files.proj 文件，无法生成索引")
	}

	// 复制配置文件模板（如果需要）
	configFiles := []string{"prunefix", "prunefile", "includefile"}
	for _, file := range configFiles {
		configPath := fmt.Sprintf("%s.conf", file)
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			templatePath := filepath.Join(os.Getenv("HOME"), "loadrc", fmt.Sprintf("%s_template.conf", file))
			if err := utils.CopyFile(templatePath, configPath); err != nil {
				fmt.Printf("WARN: 无法复制配置模板 %s: %v\n", file, err)
			} else {
				fmt.Printf("INFO: 已复制 %s 配置模板\n", file)
			}
		} else {
			fmt.Printf("INFO: %s.conf 已存在，跳过\n", file)
		}
	}

	// 创建临时文件
	tempTarget, err := os.CreateTemp("", "files_proj_temp")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %v", err)
	}
	tempTarget.Close()
	defer os.Remove(tempTarget.Name())

	tempError, err := os.CreateTemp("", "files_proj_error")
	if err != nil {
		return fmt.Errorf("创建错误日志文件失败: %v", err)
	}
	tempError.Close()
	defer os.Remove(tempError.Name())

	// 读取配置文件
	pruneParams, err := utils.ReadFileLines("prunefix.conf")
	if err != nil {
		return fmt.Errorf("读取prunefix.conf失败: %v", err)
	}

	includeParams, err := utils.ReadFileLines("includefile.conf")
	if err != nil {
		return fmt.Errorf("读取includefile.conf失败: %v", err)
	}

	// 快速估算文件数量，超过阈值则跳过
	fileCount, err := estimateFileCount(pruneParams)
	if err != nil {
		fmt.Printf("WARN: 估算文件数量失败: %v，继续执行\n", err)
	} else if fileCount > MaxFileCount {
		return fmt.Errorf("目录下文件数量(%d)超过阈值(%d)，跳过索引生成以避免耗尽系统资源", fileCount, MaxFileCount)
	} else {
		fmt.Printf("INFO: 预估文件数量: %d，在阈值(%d)内\n", fileCount, MaxFileCount)
	}

	// 执行find命令（带超时）
	findCmd := buildFindCommand(pruneParams, false, tempTarget.Name(), tempError.Name())
	fmt.Printf("INFO: 执行find命令: %s\n", findCmd)
	ctx, cancel := context.WithTimeout(context.Background(), FindTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", findCmd)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("find命令执行超时(%v)，目录文件过多，已终止", FindTimeout)
		}
		fmt.Printf("WARN: find命令执行出现错误: %v\n", err)
		// 继续执行，不中断流程
	}

	// 如果有包含参数，执行额外的find命令（带超时）
	if len(includeParams) > 0 {
		includeFindCmd := buildFindCommand(includeParams, true, tempTarget.Name(), tempError.Name())
		fmt.Printf("INFO: 执行include find命令: %s\n", includeFindCmd)
		ctx2, cancel2 := context.WithTimeout(context.Background(), FindTimeout)
		defer cancel2()
		cmd = exec.CommandContext(ctx2, "sh", "-c", includeFindCmd)
		if err := cmd.Run(); err != nil {
			if ctx2.Err() == context.DeadlineExceeded {
				return fmt.Errorf("include find命令执行超时(%v)，目录文件过多，已终止", FindTimeout)
			}
			fmt.Printf("WARN: include find命令执行出现错误: %v\n", err)
			// 继续执行，不中断流程
		}
	}

	// 检查错误日志
	if errorInfo, err := os.ReadFile(tempError.Name()); err == nil && len(errorInfo) > 0 {
		fmt.Printf("INFO: find命令执行时有一些错误，查看 %s 获取详情\n", tempError.Name())
	}

	// 处理临时文件路径
	tempSorted, err := os.CreateTemp("", "files_proj_sorted")
	if err != nil {
		return fmt.Errorf("创建排序临时文件失败: %v", err)
	}
	tempSorted.Close()
	defer os.Remove(tempSorted.Name())

	// 使用sed处理文件路径，确保格式正确，并使用LC_ALL=C确保排序一致性
	sedCmd := fmt.Sprintf("LC_ALL=C sed -e 's|^\\./||' -e 's|^|\"./|' -e 's|$|\"|' \"%s\" | LC_ALL=C sort -u > \"%s\"",
		tempTarget.Name(), tempSorted.Name())
	cmd = exec.Command("sh", "-c", sedCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("处理文件路径失败: %v", err)
	}

	// 创建排序的prune文件
	sortedPrune, err := os.CreateTemp("", "files_proj_prune_sorted")
	if err != nil {
		return fmt.Errorf("创建排序prune文件失败: %v", err)
	}
	sortedPrune.Close()
	defer os.Remove(sortedPrune.Name())

	// 对prune文件进行排序，确保使用LC_ALL=C
	sortCmd := fmt.Sprintf("LC_ALL=C sort \"prunefile.conf\" > \"%s\"", sortedPrune.Name())
	cmd = exec.Command("sh", "-c", sortCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("排序prunefile失败: %v", err)
	}

	// 使用comm命令来排除文件（兼容Linux和macOS）
	finalTarget, err := os.CreateTemp("", "files_proj_final")
	if err != nil {
		return fmt.Errorf("创建最终文件失败: %v", err)
	}
	finalTarget.Close()
	defer os.Remove(finalTarget.Name())

	commCmd := fmt.Sprintf("LC_ALL=C comm -23 \"%s\" \"%s\" > \"%s\" 2>/tmp/comm_error.log",
		tempSorted.Name(), sortedPrune.Name(), finalTarget.Name())
	fmt.Printf("INFO: 执行comm命令: %s\n", commCmd)
	cmd = exec.Command("bash", "-c", commCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("comm命令执行失败: %v", err)
	}

	// 移除可能的空行和不正确的条目
	cleanCmd := fmt.Sprintf("sed -e '/^$/d' -e '/^\"\\.\\.\\/\\./d' -e 's/^\"\\./\"\\./g' \"%s\" > \"%s.new\" && mv \"%s.new\" \"%s\"",
		finalTarget.Name(), finalTarget.Name(), finalTarget.Name(), finalTarget.Name())
	cmd = exec.Command("sh", "-c", cleanCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("清理文件失败: %v", err)
	}

	// 验证生成的文件
	finalInfo, err := os.Stat(finalTarget.Name())
	if err != nil || finalInfo.Size() == 0 {
		return fmt.Errorf("生成的文件为空")
	}

	// 备份现有的files.proj
	backupPath := filesProjPath + ".bak"
	if err := utils.CopyFile(filesProjPath, backupPath); err != nil {
		return fmt.Errorf("备份files.proj失败: %v", err)
	}

	// 检查文件是否有变化
	diffCmd := exec.Command("diff", "-q", finalTarget.Name(), filesProjPath)
	if diffCmd.Run() == nil {
		fmt.Println("INFO: files.proj 没有变化，保留原文件")
		return nil
	}

	// 更新files.proj
	newPath := filesProjPath + ".new"
	if err := utils.CopyFile(finalTarget.Name(), newPath); err != nil {
		return fmt.Errorf("创建新files.proj失败: %v", err)
	}

	if err := os.Rename(newPath, filesProjPath); err != nil {
		// 恢复备份
		utils.CopyFile(backupPath, filesProjPath)
		return fmt.Errorf("重命名新files.proj失败: %v", err)
	}

	// 更新~/all.proj
	allProjPath := filepath.Join(os.Getenv("HOME"), "all.proj")
	newEntry := fmt.Sprintf("\"%s/files.proj\"", targetDir)
	if err := appendUniqueLine(allProjPath, newEntry); err != nil {
		return fmt.Errorf("更新all.proj失败: %v", err)
	}

	fmt.Println("INFO: 脚本执行成功")
	return nil
}

// 构建find命令
func buildFindCommand(patterns []string, isInclude bool, outputFile, errorFile string) string {
	var cmd strings.Builder

	cmd.WriteString("find . ")

	if isInclude {
		// 包含模式
		if len(patterns) > 0 {
			cmd.WriteString("\\( ")
			for i, pattern := range patterns {
				if i > 0 {
					cmd.WriteString("-o ")
				}
				// 去除可能的引号
				pattern = strings.Trim(pattern, "\"")
				cmd.WriteString(fmt.Sprintf("-wholename '%s' ", pattern))
			}
			cmd.WriteString("\\) ")
			cmd.WriteString("-type f -size -9000k ")
		} else {
			cmd.WriteString("-type f -size -9000k ")
		}
	} else {
		// 排除模式
		if len(patterns) > 0 {
			cmd.WriteString("\\( ")
			for i, pattern := range patterns {
				if i > 0 {
					cmd.WriteString("-o ")
				}
				// 去除可能的引号
				pattern = strings.Trim(pattern, "\"")
				cmd.WriteString(fmt.Sprintf("-wholename '%s' ", pattern))
			}
			cmd.WriteString("\\) -a -prune -o ")
		}
		cmd.WriteString("-size +0 -type f ")
	}

	// 使用grep过滤文本文件
	cmd.WriteString("-exec grep -Il \"\" {} + 2>>")
	cmd.WriteString(errorFile)

	// 添加输出重定向
	if isInclude {
		cmd.WriteString(fmt.Sprintf(" >>%s || true", outputFile))
	} else {
		cmd.WriteString(fmt.Sprintf(" >%s || true", outputFile))
	}

	return cmd.String()
}

// estimateFileCount 快速估算目录下的文件数量（排除 prune 目录）
// 使用 find 配合 head 快速截断，避免遍历全部文件
func estimateFileCount(pruneParams []string) (int, error) {
	var cmd strings.Builder
	cmd.WriteString("find . ")

	// 加入排除规则
	if len(pruneParams) > 0 {
		cmd.WriteString("\\( ")
		for i, pattern := range pruneParams {
			if i > 0 {
				cmd.WriteString("-o ")
			}
			pattern = strings.Trim(pattern, "\"")
			cmd.WriteString(fmt.Sprintf("-wholename '%s' ", pattern))
		}
		cmd.WriteString("\\) -a -prune -o ")
	}

	// 只计数文件，用 head 限制输出避免遍历过多
	limit := MaxFileCount + 1
	cmdStr := fmt.Sprintf("%s -type f -print | head -n %d | wc -l", cmd.String(), limit)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sh", "-c", cmdStr).Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// 30秒内都无法完成计数，说明文件极多
			return MaxFileCount + 1, nil
		}
		return 0, err
	}

	var count int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	return count, nil
}

// 追加唯一行到文件
func appendUniqueLine(filePath, line string) error {
	// 检查文件是否存在
	var lines []string

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		// 文件存在，读取内容
		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		lines = strings.Split(string(content), "\n")

		// 检查行是否已存在
		for _, existingLine := range lines {
			if existingLine == line {
				return nil // 行已存在，不需要添加
			}
		}
	}

	// 添加新行
	lines = append(lines, line)

	// 过滤空行并排序
	var nonEmptyLines []string
	for _, l := range lines {
		if l != "" {
			nonEmptyLines = append(nonEmptyLines, l)
		}
	}

	// 排序
	sort.Strings(nonEmptyLines)

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(filePath, []byte(strings.Join(nonEmptyLines, "\n")+"\n"), 0644)
}
