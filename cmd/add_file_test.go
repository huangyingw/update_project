package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"projupdater/utils"
)

func TestAddFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	project := filepath.Join(root, "proj")
	cache := filepath.Join(root, ".cache", "tower")
	for _, d := range []string{project, cache} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, project, "init", "-q")
	for _, f := range []string{filepath.Join(project, "files.proj"), filepath.Join(cache, "a.md")} {
		if err := os.WriteFile(f, []byte("x\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	// 有一个不合法 → 整体不写
	if err := addFiles([]string{"~/.cache/tower/a.md", "~/.cache/tower/missing.md"}); err == nil {
		t.Fatal("不存在的文件应报错")
	}
	if _, err := os.Stat(filepath.Join(project, utils.LocalConfigFile)); !os.IsNotExist(err) {
		t.Fatalf("校验失败时不应写出 %s", utils.LocalConfigFile)
	}

	if err := addFiles([]string{"~/.cache/tower/a.md", filepath.Join(cache, "todo-*.md"), "~/.cache/tower/a.md"}); err != nil {
		t.Fatal(err)
	}
	vals, err := utils.ReadExtraFiles(project)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(cache, "a.md"), filepath.Join(cache, "todo-*.md")}
	if strings.Join(vals, "\n") != strings.Join(want, "\n") {
		t.Errorf("登记值 got %v want %v", vals, want)
	}
	if status := gitRun(t, project, "status", "--porcelain"); strings.Contains(status, utils.LocalConfigFile) {
		t.Errorf("%s 应被 info/exclude 忽略:\n%s", utils.LocalConfigFile, status)
	}
}
