package utils

import (
	"os"
	"path/filepath"
	"testing"
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
	err := RunOnce("test_lock", func() error {
		// Do nothing
		return nil
	})
	if err != nil {
		t.Errorf("RunOnce error: %v", err)
	}

	// 尝试再次运行，应该返回错误
	err = RunOnce("test_lock", func() error {
		// Do nothing
		return nil
	})
	if err == nil {
		t.Errorf("Expected error when running second instance")
	}
}
