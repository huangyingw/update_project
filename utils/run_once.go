package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// LockInfo 保存锁的相关信息
type LockInfo struct {
	PID       int       `json:"pid"`
	StartTime time.Time `json:"start_time"`
	Name      string    `json:"name"`
}

// 进程内的锁映射，用于确保同一进程内也能锁定
var (
	processLocks      = make(map[string]bool)
	processLocksMutex sync.Mutex
)

// RunOnce 通过文件锁确保同一时刻只有一个进程运行指定的函数
func RunOnce(name string, fn func() error) error {
	// 获取当前工作目录，将其作为锁的一部分
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("无法获取当前工作目录: %w", err)
	}

	// 使用当前工作目录创建一个唯一的锁名
	// 将路径转换为适合作为锁名的字符串
	safeCwd := filepath.ToSlash(cwd)
	// 为每个目录创建唯一的锁名
	lockName := fmt.Sprintf("%s_%s", name, safeCwd)

	// 首先检查进程内锁
	processLocksMutex.Lock()
	fmt.Printf("检查进程内锁 %s: 当前状态 = %v\n", lockName, processLocks[lockName])
	if processLocks[lockName] {
		processLocksMutex.Unlock()
		pid := os.Getpid()
		fmt.Printf("进程内锁检测到另一个实例正在运行: %s, PID: %d\n", lockName, pid)
		return fmt.Errorf("另一个实例正在运行，当前进程ID: %d", pid)
	}
	processLocks[lockName] = true
	fmt.Printf("设置进程内锁 %s = true\n", lockName)
	processLocksMutex.Unlock()

	// 确保在函数返回时释放进程内锁
	defer func() {
		processLocksMutex.Lock()
		delete(processLocks, lockName)
		fmt.Printf("清除进程内锁 %s\n", lockName)
		processLocksMutex.Unlock()
	}()

	// 创建目录特定的锁文件名
	lockFile := fmt.Sprintf("%s_%s.lck", name, filepath.Base(cwd))
	lockPath := filepath.Join(os.TempDir(), lockFile)
	lockInfoPath := fmt.Sprintf("%s.info", lockPath)

	fmt.Printf("尝试获取锁: %s\n", lockPath)

	// 检查锁文件是否存在，如果存在，检查对应的进程是否还活着
	if _, err := os.Stat(lockPath); err == nil {
		// 锁文件存在，尝试读取进程信息
		if lockInfo, err := readLockInfo(lockInfoPath); err == nil {
			// 检查该进程是否还存在
			if !isProcessRunning(lockInfo.PID) {
				fmt.Printf("检测到孤儿锁：PID %d 不存在，删除锁文件\n", lockInfo.PID)
				os.Remove(lockPath)
				os.Remove(lockInfoPath)
			}
		}
	}

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

	// 获取锁后，写入锁信息文件
	lockInfo := LockInfo{
		PID:       os.Getpid(),
		StartTime: time.Now(),
		Name:      lockName,
	}
	if err := writeLockInfo(lockInfoPath, lockInfo); err != nil {
		fmt.Printf("警告：写入锁信息失败: %v\n", err)
	}

	// 确保在函数返回时释放锁，并删除锁信息文件
	defer func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		os.Remove(lockInfoPath)
	}()

	// 记录启动时间用于性能跟踪
	startTime := time.Now()
	fmt.Printf("成功获取锁，开始执行任务: %s\n", lockName)

	// 运行实际的函数
	err = fn()

	// 记录执行时间
	elapsed := time.Since(startTime)
	if err != nil {
		fmt.Printf("任务执行失败: %s, 耗时: %v, 错误: %v\n", lockName, elapsed, err)
	} else {
		fmt.Printf("任务执行成功: %s, 耗时: %v\n", lockName, elapsed)
	}

	return err
}

// isProcessRunning 检查进程是否正在运行
func isProcessRunning(pid int) bool {
	// 对于Darwin/MacOS系统
	if _, err := os.FindProcess(pid); err != nil {
		return false
	}

	// 在UNIX系统中，os.FindProcess几乎总是成功的，需要发送信号0来检查进程是否真的存在
	cmd := exec.Command("kill", "-0", strconv.Itoa(pid))
	return cmd.Run() == nil
}

// writeLockInfo 写入锁信息到文件
func writeLockInfo(path string, info LockInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// readLockInfo 从文件读取锁信息
func readLockInfo(path string) (LockInfo, error) {
	var info LockInfo
	data, err := os.ReadFile(path)
	if err != nil {
		return info, err
	}
	err = json.Unmarshal(data, &info)
	return info, err
}
