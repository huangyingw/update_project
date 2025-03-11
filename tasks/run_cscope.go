package tasks

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

var cscopeLockFile = filepath.Join(os.TempDir(), "cscope_update.lck")

func RunCscope() error {
	// 使用文件锁确保只有一个进程在更新cscope索引
	lockFile, err := os.OpenFile(cscopeLockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("无法创建cscope锁文件: %w", err)
	}
	defer lockFile.Close()
	defer os.Remove(cscopeLockFile)

	// 尝试获取文件锁，非阻塞模式
	err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("另一个进程正在更新cscope索引")
		}
		return fmt.Errorf("获取cscope锁失败: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

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

	// 复制 files.proj
	err = copyFile("files.proj", sourceFile)
	if err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	// 替换特殊字符
	err = replaceInFile(sourceFile, `\\ `, ` `)
	if err != nil {
		return fmt.Errorf("替换文件内容失败: %w", err)
	}

	// 运行 cscope 生成临时索引文件
	cmd := exec.Command("cscope", "-bq", "-i", sourceFile, "-f", tempCscopeOut)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cscope 命令执行失败: %w, 输出: %s", err, string(output))
	}

	fmt.Println("已创建临时cscope索引文件")

	// 检查临时文件是否生成
	if _, err := os.Stat(tempCscopeOut); os.IsNotExist(err) {
		return fmt.Errorf("临时索引文件未生成: %s", tempCscopeOut)
	}

	// 原子地替换索引文件以不影响搜索
	if err := safeReplaceFile(tempCscopeOut, "cscope.out"); err != nil {
		return fmt.Errorf("替换cscope.out失败: %w", err)
	}

	// 检查并替换其他cscope文件（如果存在）
	if _, err := os.Stat(tempCscopeIn); !os.IsNotExist(err) {
		if err := safeReplaceFile(tempCscopeIn, "cscope.in.out"); err != nil {
			return fmt.Errorf("替换cscope.in.out失败: %w", err)
		}
	}

	if _, err := os.Stat(tempCscopePo); !os.IsNotExist(err) {
		if err := safeReplaceFile(tempCscopePo, "cscope.po.out"); err != nil {
			return fmt.Errorf("替换cscope.po.out失败: %w", err)
		}
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

	_, err = io.Copy(dstFile, srcFile)
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

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	err = os.WriteFile(dst, input, 0644)
	return err
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
