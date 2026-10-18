package tasks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 额外目录(.gitconfig 的 proj.extra)里的文本文件以 "../sibling/..." 并入 files.proj,
// 主目录条目保持 "./..." 形式; rsync.files 不收 ../ 条目。
func TestGenerateFileIndexWithExtraDirs(t *testing.T) {
	root := t.TempDir()
	// 隔离 ~/all.proj 与配置模板副作用
	t.Setenv("HOME", filepath.Join(root, "home"))

	project := filepath.Join(root, "proj")
	sibling := filepath.Join(root, "sibling")
	nested := filepath.Join(sibling, "nested")
	missing := filepath.Join(root, "missing")
	for _, d := range []string{project, nested, filepath.Join(root, "home")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(project, "files.proj"), "")
	write(filepath.Join(project, "main.go"), "package main\n")
	write(filepath.Join(project, "skip.log"), "log\n")
	write(filepath.Join(project, "prunefix.conf"), "./*.log\n")
	write(filepath.Join(project, "prunefile.conf"), "\"./files.proj\"\n\"./prunefix.conf\"\n\"./prunefile.conf\"\n\"./includefile.conf\"\n\"./.gitconfig\"\n")
	write(filepath.Join(project, "includefile.conf"), "")
	write(filepath.Join(sibling, "lib.go"), "package lib\n")
	write(filepath.Join(sibling, "noise.log"), "log\n")
	write(filepath.Join(nested, "deep.txt"), "deep\n")

	for _, rel := range []string{"../sibling", "../missing"} {
		cmd := exec.Command("git", "config", "--file", filepath.Join(project, ".gitconfig"), "--add", "proj.extra", rel)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("写 .gitconfig 失败: %v %s", err, out)
		}
	}
	_ = missing

	wd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if err := GenerateFileIndex(); err != nil {
		t.Fatalf("GenerateFileIndex: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(project, "files.proj"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(content)), "\n")
	want := []string{
		`"../sibling/lib.go"`,
		`"../sibling/nested/deep.txt"`,
		`"./main.go"`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("files.proj 内容不符:\n got: %q\nwant: %q", got, want)
	}

	if err := GenerateRsyncFiles(); err != nil {
		t.Fatalf("GenerateRsyncFiles: %v", err)
	}
	rsync, err := os.ReadFile(filepath.Join(project, "rsync.files"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rsync), "../") {
		t.Errorf("rsync.files 不应包含项目外条目:\n%s", rsync)
	}
	if !strings.Contains(string(rsync), "./main.go") {
		t.Errorf("rsync.files 应包含主目录条目:\n%s", rsync)
	}
}
