package tasks

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"projupdater/utils"
)

const (
	// MaxCscopeFiles 是 cscope 允许处理的最大文件数量
	MaxCscopeFiles = 100000
	// CscopeTimeout 是 cscope 命令的超时时间
	CscopeTimeout = 10 * time.Minute
)

func RunCscope() error {
	sourceFile := "cscopesourcefile.bak"
	tempCscopeOut := "cscope.out.bak"
	tempCscopeIn := "cscope.out.bak.in"
	tempCscopePo := "cscope.out.bak.po"

	// 创建清理函数
	cleanup := func() {
		os.Remove(sourceFile + ".bak")
		os.Remove(tempCscopeOut)
		os.Remove(tempCscopeIn)
		os.Remove(tempCscopePo)
	}

	// 确保在函数退出时执行清理
	defer cleanup()

	// 复制 files.proj 并处理特殊字符 - 这两步合并成一个操作提高性能
	err := utils.CopyFileAndReplace("files.proj", sourceFile, `\\ `, ` `)
	if err != nil {
		return fmt.Errorf("复制并处理文件失败: %w", err)
	}

	// 检查文件数量，超过阈值则跳过 cscope
	lineCount, err := countFileLines(sourceFile)
	if err != nil {
		fmt.Printf("WARN: 无法统计文件数量: %v，继续执行\n", err)
	} else if lineCount > MaxCscopeFiles {
		return fmt.Errorf("files.proj 包含 %d 个文件，超过cscope阈值(%d)，跳过索引生成以避免耗尽系统资源", lineCount, MaxCscopeFiles)
	} else {
		fmt.Printf("INFO: files.proj 包含 %d 个文件，在阈值(%d)内\n", lineCount, MaxCscopeFiles)
	}

	// 运行 cscope 生成临时索引文件，使用-k选项以提高性能（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), CscopeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "cscope", "-bkq", "-i", sourceFile, "-f", tempCscopeOut)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("cscope 命令执行超时(%v)，文件过多，已终止", CscopeTimeout)
		}
		return fmt.Errorf("cscope 命令执行失败: %w, 输出: %s", err, string(output))
	}

	fmt.Println("已创建临时cscope索引文件")

	// 检查临时文件是否生成
	if _, err := os.Stat(tempCscopeOut); os.IsNotExist(err) {
		return fmt.Errorf("临时索引文件未生成: %s", tempCscopeOut)
	}

	// 并行替换所有cscope文件
	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	// 替换主索引文件
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := safeReplaceFile(tempCscopeOut, "cscope.out"); err != nil {
			errChan <- fmt.Errorf("替换cscope.out失败: %w", err)
		}
	}()

	// 检查并替换其他cscope文件（如果存在）
	if _, err := os.Stat(tempCscopeIn); !os.IsNotExist(err) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := safeReplaceFile(tempCscopeIn, "cscope.in.out"); err != nil {
				errChan <- fmt.Errorf("替换cscope.in.out失败: %w", err)
			}
		}()
	}

	if _, err := os.Stat(tempCscopePo); !os.IsNotExist(err) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := safeReplaceFile(tempCscopePo, "cscope.po.out"); err != nil {
				errChan <- fmt.Errorf("替换cscope.po.out失败: %w", err)
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// 检查是否有错误
	for err := range errChan {
		return err // 返回第一个遇到的错误
	}

	fmt.Println("成功更新 cscope 索引文件")
	return nil
}

// 安全地替换文件：先复制到临时文件，再原子重命名
func safeReplaceFile(src, dst string) error {
	// 确保源文件存在
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("源文件不存在: %s", src)
	}

	// 创建一个临时目标文件
	tempDst := dst + ".tmp"

	// 如果存在旧的临时文件，先删除
	os.Remove(tempDst)

	// 复制内容
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(tempDst)
	if err != nil {
		return err
	}

	// 使用defer加Close()，但需要检查错误
	defer func() {
		cerr := dstFile.Close()
		if err == nil {
			err = cerr
		}
	}()

	// 使用较大的缓冲区提高复制性能
	buf := make([]byte, 1024*1024) // 1MB缓冲区
	_, err = io.CopyBuffer(dstFile, srcFile, buf)
	if err != nil {
		os.Remove(tempDst) // 清理临时文件
		return err
	}

	// 确保所有数据都写入磁盘
	if err := dstFile.Sync(); err != nil {
		os.Remove(tempDst)
		return err
	}

	// 显式关闭文件，确保不会在重命名时出现问题
	if err := dstFile.Close(); err != nil {
		os.Remove(tempDst)
		return err
	}

	// 原子地重命名临时文件为目标文件
	// 在Unix系统上，这是原子操作
	return os.Rename(tempDst, dst)
}

// 复制文件并替换内容 - 合并两个操作提高性能
func copyFileAndReplace(src, dst, oldStr, newStr string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// 替换内容
	output := strings.ReplaceAll(string(input), oldStr, newStr)

	// 写入到目标文件
	return os.WriteFile(dst, []byte(output), 0644)
}

// countFileLines 统计文件的行数
func countFileLines(filename string) (int, error) {
	f, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() != "" {
			count++
		}
	}
	return count, scanner.Err()
}

func replaceInFile(filename, oldStr, newStr string) error {
	input, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	output := strings.ReplaceAll(string(input), oldStr, newStr)
	err = os.WriteFile(filename, []byte(output), 0644)
	return err
}
