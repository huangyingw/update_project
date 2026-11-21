package tasks

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
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
	// 额外目录(update_proj add-dir)的条目在项目根之外("../..."), rsync.files 只描述本项目,
	// 不收它们——否则 --files-from 会把项目外的文件一并同步走
	filesProj = utils.FilterOutsideProject(filesProj)

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

		// 找出 files.proj 里符合排除规则的文件(只检查已有条目, 不遍历整棵树, 见 findListedMatches)
		diffOutput, err := findListedMatches(filesProj, prunePatterns)
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

// findArgsBatchBytes 是一次 find 调用里起点参数的总字节上限, 远低于 ARG_MAX(macOS 1 MB, 含环境变量)
const findArgsBatchBytes = 128 * 1024

// findListedMatches 返回 files 中满足 `find . ( -path p1 -o -path p2 ... ) -type f -size -9000k`
// 的条目, 每行一个, 写法与该 find 的输出相同("./a/b")。
//
// 结果只用来从 files 里剔除条目(comm -23), 树里不在 files 中的文件不影响结果, 所以把 files 本身
// 作为 find 的起点(-maxdepth 0)逐个检查, 而不是从 "." 遍历整棵树——主检出 ~/loadrc 下有 59 个
// worktree 和 .git, 整树遍历一次 14 秒、69 万个条目, 而 files.proj 只有几千条(2026-11-20)。
// 匹配、类型、大小判断仍交给 find, 语义与整树遍历一致。
//
// 不是 "./" 开头的规范相对路径的条目跳过: 整树遍历的输出不会有这种写法, comm 本来就剔不掉它们。
// 已不存在的条目 find 会报错并以非零退出, 这里照常采用它对其余起点的输出。
func findListedMatches(files []string, patterns []string) ([]byte, error) {
	expr := []string{"-maxdepth", "0", "("}
	for i, pattern := range patterns {
		if i > 0 {
			expr = append(expr, "-o")
		}
		expr = append(expr, "-path", strings.Trim(pattern, "\""))
	}
	expr = append(expr, ")", "-type", "f", "-size", "-9000k")

	var out bytes.Buffer
	run := func(batch []string) error {
		cmd := exec.Command("find", append(batch, expr...)...)
		cmd.Stdout = &out
		err := cmd.Run()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil // 某些起点不存在等, 其余起点的输出照用
		}
		return err
	}

	var batch []string
	size := 0
	for _, f := range files {
		if !strings.HasPrefix(f, "./") || strings.HasPrefix(f, "./../") || "./"+path.Clean(f) != f {
			continue
		}
		if size+len(f)+1 > findArgsBatchBytes && len(batch) > 0 {
			if err := run(batch); err != nil {
				return nil, err
			}
			batch, size = nil, 0
		}
		batch = append(batch, f)
		size += len(f) + 1
	}
	if len(batch) > 0 {
		if err := run(batch); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}
