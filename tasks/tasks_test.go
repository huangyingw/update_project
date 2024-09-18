package tasks

import (
	"os"
	"testing"
)

func TestGenerateFileIndex(t *testing.T) {
	// 创建临时的 files.proj
	err := os.WriteFile("files.proj", []byte("\"testfile.go\"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create files.proj: %v", err)
	}
	defer os.Remove("files.proj")
	defer os.Remove("files.proj.bak")

	err = GenerateFileIndex()
	if err != nil {
		t.Errorf("GenerateFileIndex error: %v", err)
	}
}

func TestGenerateRsyncFiles(t *testing.T) {
	// 创建临时的 files.proj
	err := os.WriteFile("files.proj", []byte("\"testfile.go\"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create files.proj: %v", err)
	}
	defer os.Remove("files.proj")
	defer os.Remove("rsync.files")

	err = GenerateRsyncFiles()
	if err != nil {
		t.Errorf("GenerateRsyncFiles error: %v", err)
	}
}

func TestRunCscope(t *testing.T) {
	// 创建临时的 files.proj
	err := os.WriteFile("files.proj", []byte("\"testfile.go\"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create files.proj: %v", err)
	}
	defer os.Remove("files.proj")
	defer os.Remove("cscope.out")
	defer os.Remove("cscope.in.out")
	defer os.Remove("cscope.po.out")

	err = RunCscope()
	if err != nil {
		t.Errorf("RunCscope error: %v", err)
	}
}
