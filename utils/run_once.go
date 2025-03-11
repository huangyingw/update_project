package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// 进程内的锁映射，用于确保同一进程内也能锁定
var (
	processLocks      = make(map[string]bool)
	processLocksMutex sync.Mutex
)

// RunOnce 通过文件锁确保同一时刻只有一个进程运行指定的函数
func RunOnce(name string, fn func() error) error {
	// 首先检查进程内锁
	processLocksMutex.Lock()
	fmt.Printf("检查进程内锁 %s: 当前状态 = %v\n", name, processLocks[name])
	if processLocks[name] {
		processLocksMutex.Unlock()
		pid := os.Getpid()
		fmt.Printf("进程内锁检测到另一个实例正在运行: %s, PID: %d\n", name, pid)
		return fmt.Errorf("另一个实例正在运行，当前进程ID: %d", pid)
	}
	processLocks[name] = true
	fmt.Printf("设置进程内锁 %s = true\n", name)
	processLocksMutex.Unlock()

	// 确保在函数返回时释放进程内锁
	defer func() {
		processLocksMutex.Lock()
		delete(processLocks, name)
		fmt.Printf("清除进程内锁 %s\n", name)
		processLocksMutex.Unlock()
	}()

	lockFile := fmt.Sprintf("%s.lck", name)
	lockPath := filepath.Join(os.TempDir(), lockFile)

	fmt.Printf("尝试获取锁: %s\n", lockPath)

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("无法创建锁文件: %w", err)
	}
	defer file.Close()

	// 尝试获取文件锁
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if err == syscall.EWOULDBLOCK {
			// 可以添加进程ID信息以便更好地调试
			pid := os.Getpid()
			return fmt.Errorf("另一个实例正在运行，当前进程ID: %d", pid)
		}
		return fmt.Errorf("获取文件锁失败: %w", err)
	}

	// 确保在函数返回时释放锁，但不删除锁文件
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	// 记录启动时间用于性能跟踪
	startTime := time.Now()
	fmt.Printf("成功获取锁，开始执行任务: %s\n", name)

	// 运行实际的函数
	err = fn()

	// 记录执行时间
	elapsed := time.Since(startTime)
	if err != nil {
		fmt.Printf("任务执行失败: %s, 耗时: %v, 错误: %v\n", name, elapsed, err)
	} else {
		fmt.Printf("任务执行成功: %s, 耗时: %v\n", name, elapsed)
	}

	return err
}
