package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"projupdater/utils"
)

// .proj.local 登记的本机文件以绝对路径并入 files.proj, 同目录里没登记的文件不进;
// rsync.files 不收它们。
func TestGenerateFileIndexWithExtraFiles(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)

	project := filepath.Join(root, "proj")
	cache := filepath.Join(home, ".cache", "tower")
	for _, d := range []string{project, cache} {
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
	write(filepath.Join(project, "prunefix.conf"), "")
	write(filepath.Join(project, "prunefile.conf"), "\"./files.proj\"\n\"./prunefix.conf\"\n\"./prunefile.conf\"\n\"./includefile.conf\"\n\"./.proj.local\"\n")
	write(filepath.Join(project, "includefile.conf"), "")
	write(filepath.Join(cache, "reminders.md"), "r\n")
	write(filepath.Join(cache, "todo-self.md"), "s\n")
	write(filepath.Join(cache, "handoffs.md"), "h\n") // 同目录但没登记
	write(filepath.Join(cache, "todo.state.json"), "{}\n")

	for _, v := range []string{"~/.cache/tower/reminders.md", filepath.Join(cache, "todo-*.md")} {
		if _, err := utils.AddExtraFile(project, v); err != nil {
			t.Fatal(err)
		}
	}

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
		`"./main.go"`,
		`"` + filepath.Join(cache, "reminders.md") + `"`,
		`"` + filepath.Join(cache, "todo-self.md") + `"`,
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
	if strings.Contains(string(rsync), cache) {
		t.Errorf("rsync.files 不应包含登记的项目外文件:\n%s", rsync)
	}
}
