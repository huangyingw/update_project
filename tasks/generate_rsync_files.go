package tasks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"projupdater/utils"
)

func GenerateRsyncFiles() error {
	// 检查 files.proj 是否存在
	if _, err := os.Stat("files.proj"); os.IsNotExist(err) {
		return fmt.Errorf("当前目录下不存在 files.proj 文件")
	}

	// 定义文件路径
	rsyncFilesTmp := "rsync.files.tmp"
	prunefixFile := "prunefix.rsync"
	includeFile := "includefile.rsync"

	// 读取files.proj文件
	filesProj, err := utils.ReadFilesProj("files.proj")
	if err != nil {
		return err
	}

	// 将处理后的files.proj写入临时文件
	err = utils.WriteLinesToFile(rsyncFilesTmp, filesProj)
	if err != nil {
		return err
	}

	// 确保prunefix.rsync文件存在
	if _, err := os.Stat(prunefixFile); os.IsNotExist(err) {
		_, err = os.Create(prunefixFile)
		if err != nil {
			return err
		}
	}

	// 读取prunefix.rsync文件中的排除规则
	prunePatterns, err := utils.ReadFileLines(prunefixFile)
	if err != nil {
		return err
	}

	// 构建find命令参数以排除文件
	if len(prunePatterns) > 0 {
		// 创建临时文件用于存储diff结果
		rsyncFilesDiff := rsyncFilesTmp + ".diff"

		// 构建find命令来查找符合排除规则的文件
		args := []string{"."}

		// 添加路径模式
		if len(prunePatterns) > 0 {
			args = append(args, "(")
			for i, pattern := range prunePatterns {
				pattern = strings.Trim(pattern, "\"")
				if i > 0 {
					args = append(args, "-o")
				}
				args = append(args, "-path", pattern)
			}
			args = append(args, ")")
		}

		// 添加类型和大小限制
		args = append(args, "-type", "f", "-size", "-9000k")

		// 执行find命令
		cmd := exec.Command("find", args...)
		diffOutput, err := cmd.Output()
		if err != nil {
			fmt.Printf("警告: find命令执行失败: %v\n", err)
		} else {
			// 写入diff文件
			err = os.WriteFile(rsyncFilesDiff, diffOutput, 0644)
			if err != nil {
				return err
			}

			// 执行comm命令比较文件
			rsyncFilesTmp2 := rsyncFilesTmp + ".tmp"
			commCmd := exec.Command("bash", "-c",
				"comm -23 <(sort \""+rsyncFilesTmp+"\") <(sort \""+rsyncFilesDiff+"\")")
			commCmd.Env = append(os.Environ(), "SHELL=bash")
			commCmd.Dir = "."
			commCmd.Stderr = os.Stderr
			commOutput, err := commCmd.Output()

			if err != nil {
				return fmt.Errorf("comm命令执行失败: %v", err)
			}

			// 写入输出并复制回原临时文件
			err = os.WriteFile(rsyncFilesTmp2, commOutput, 0644)
			if err != nil {
				return err
			}

			err = utils.CopyFile(rsyncFilesTmp2, rsyncFilesTmp)
			if err != nil {
				return err
			}

			// 如果存在files.rev，也执行相同的比较
			if _, err := os.Stat("files.rev"); !os.IsNotExist(err) {
				commCmd := exec.Command("bash", "-c",
					"comm -23 <(sort \""+rsyncFilesTmp+"\") <(sort \"files.rev\")")
				commCmd.Env = append(os.Environ(), "SHELL=bash")
				commCmd.Dir = "."
				commCmd.Stderr = os.Stderr
				commOutput, err := commCmd.Output()

				if err != nil {
					return fmt.Errorf("comm命令执行失败: %v", err)
				}

				// 写入输出并复制回原临时文件
				err = os.WriteFile(rsyncFilesTmp2, commOutput, 0644)
				if err != nil {
					return err
				}

				err = utils.CopyFile(rsyncFilesTmp2, rsyncFilesTmp)
				if err != nil {
					return err
				}
			}

			// 清理临时文件
			os.Remove(rsyncFilesDiff)
			os.Remove(rsyncFilesTmp2)
		}
	}

	// 读取includefile.rsync文件中的包含规则
	includePatterns, err := utils.ReadFileLines(includeFile)
	if err != nil {
		return err
	}

	// 处理包含规则
	if len(includePatterns) > 0 {
		// 构建find命令来查找符合包含规则的文件
		args := []string{"."}

		// 添加路径模式
		if len(includePatterns) > 0 {
			args = append(args, "(")
			for i, pattern := range includePatterns {
				pattern = strings.Trim(pattern, "\"")
				if i > 0 {
					args = append(args, "-o")
				}
				args = append(args, "-path", pattern)
			}
			args = append(args, ")")
		}

		// 添加类型限制
		args = append(args, "-type", "f")

		// 执行find命令
		cmd := exec.Command("find", args...)
		includeOutput, err := cmd.Output()
		if err != nil {
			fmt.Printf("警告: find命令执行失败: %v\n", err)
		} else {
			// 读取现有的rsync.files.tmp内容
			existingContent, err := os.ReadFile(rsyncFilesTmp)
			if err != nil {
				return err
			}

			// 合并内容
			allContent := string(existingContent) + string(includeOutput)

			// 写回临时文件
			err = os.WriteFile(rsyncFilesTmp, []byte(allContent), 0644)
			if err != nil {
				return err
			}

			// 排序并去重
			sortCmd := exec.Command("sort", "-u", rsyncFilesTmp, "-o", rsyncFilesTmp)
			err = sortCmd.Run()
			if err != nil {
				return fmt.Errorf("sort命令执行失败: %v", err)
			}
		}
	}

	// 最后，复制临时文件为正式文件
	err = utils.CopyFile(rsyncFilesTmp, "rsync.files")
	if err != nil {
		return err
	}

	// 清理临时文件
	os.Remove(rsyncFilesTmp)
	os.Remove("rsync.files.tmp")

	fmt.Println("成功生成 rsync.files")
	return nil
}
