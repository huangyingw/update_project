package utils

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestIsRemoteMounted(t *testing.T) {
	mounted, err := IsRemoteMounted(".")
	if err != nil {
		t.Errorf("IsRemoteMounted error: %v", err)
	}
	t.Logf("IsRemoteMounted: %v", mounted)
}

func TestCopyFile(t *testing.T) {
	src := "test_src.txt"
	dst := "test_dst.txt"
	content := []byte("Hello, world!")
	err := os.WriteFile(src, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}
	defer os.Remove(src)
	defer os.Remove(dst)

	err = CopyFile(src, dst)
	if err != nil {
		t.Errorf("CopyFile error: %v", err)
	}

	dstContent, err := os.ReadFile(dst)
	if err != nil {
		t.Errorf("Failed to read destination file: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Errorf("Content mismatch: got %s, want %s", dstContent, content)
	}
}

func TestRunOnce(t *testing.T) {
	// 手动删除可能存在的锁文件
	lockFile := filepath.Join(os.TempDir(), "test_lock.lck")
	os.Remove(lockFile)

	// 确保测试结束后清理锁文件
	defer os.Remove(lockFile)

	// 第一个goroutine启动并获取锁
	var wg sync.WaitGroup
	wg.Add(1)

	var firstFuncExecuted bool
	var secondFuncExecuted bool
	var firstError error
	var secondError error

	// 第一个goroutine获取锁并持有10毫秒
	go func() {
		defer wg.Done()

		t.Log("开始第一个goroutine调用...")
		firstError = RunOnce("test_lock", func() error {
			t.Log("第一个goroutine的函数执行中，持有锁10毫秒")
			firstFuncExecuted = true
			time.Sleep(10 * time.Millisecond) // 持有锁10毫秒
			return nil
		})
		t.Log("第一个goroutine调用完成")
	}()

	// 等待一小段时间确保第一个goroutine启动
	time.Sleep(5 * time.Millisecond)

	// 第二个goroutine尝试获取锁，应该失败
	t.Log("开始第二个goroutine调用...")
	secondError = RunOnce("test_lock", func() error {
		t.Log("第二个goroutine的函数被执行 - 这不应该发生")
		secondFuncExecuted = true
		return nil
	})
	t.Log("第二个goroutine调用完成")

	// 等待第一个goroutine完成
	wg.Wait()

	// 检查结果
	if firstError != nil {
		t.Errorf("第一个goroutine调用出错: %v", firstError)
	}

	if !firstFuncExecuted {
		t.Errorf("第一个goroutine的函数应该被执行")
	}

	if secondError == nil {
		t.Errorf("第二个goroutine调用应该返回错误，但没有")
	} else {
		t.Logf("正确接收到第二个goroutine错误: %v", secondError)
	}

	if secondFuncExecuted {
		t.Errorf("第二个goroutine的函数不应该被执行")
	}

	t.Log("测试完成")
}
