package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func RunOnce(name string, fn func() error) error {
	lockFile := fmt.Sprintf("%s.lck", name)
	lockPath := filepath.Join(os.TempDir(), lockFile)

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("另一个实例正在运行")
		}
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	// 运行实际的函数
	return fn()
}
