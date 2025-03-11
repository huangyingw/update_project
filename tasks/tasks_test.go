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
	// 确保测试前清理可能存在的锁文件
	os.Remove(cscopeLockFile)

	// 清理可能存在的临时文件
	cleanupFiles := []string{
		"files.proj", "cscope.out", "cscope.in.out", "cscope.po.out",
		"cscopesourcefile.bak", "cscopesourcefile.bak.bak",
		"cscope.out.bak", "cscope.out.bak.in", "cscope.out.bak.po",
		"cscope.out.tmp", "cscope.in.out.tmp", "cscope.po.out.tmp",
	}

	for _, file := range cleanupFiles {
		os.Remove(file)
	}

	// 创建临时文件用于测试
	testfile := "testfile.go"
	err := os.WriteFile(testfile, []byte("package main\n\nfunc main() {\n}\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testfile)

	// 创建临时的 files.proj
	err = os.WriteFile("files.proj", []byte("\""+testfile+"\"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create files.proj: %v", err)
	}

	// 确保清理所有临时文件
	defer func() {
		for _, file := range cleanupFiles {
			os.Remove(file)
		}
		os.Remove(cscopeLockFile)
	}()

	err = RunCscope()
	if err != nil {
		t.Errorf("RunCscope error: %v", err)
	}

	// 验证文件是否正确生成
	if _, err := os.Stat("cscope.out"); os.IsNotExist(err) {
		t.Errorf("cscope.out 文件未生成")
	}
}
