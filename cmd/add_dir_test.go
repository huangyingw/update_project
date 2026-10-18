package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	// 不让用户的全局 hook(post-commit 自动 push 等)介入测试
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestAddDirsCommits(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")

	project := filepath.Join(root, "proj")
	sibling := filepath.Join(root, "sibling")
	other := filepath.Join(root, "other")
	for _, d := range []string{project, sibling, other} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(project, "files.proj"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	// 顺手放一个已暂存的无关文件, 验证只提交 .gitconfig
	if err := os.WriteFile(filepath.Join(project, "unrelated.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "init", "-q")
	gitRun(t, project, "add", "unrelated.txt")

	wd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	// 一个绝对路径 + 一个相对路径
	if err := addDirs([]string{sibling, "../other"}, true); err != nil {
		t.Fatalf("addDirs: %v", err)
	}

	cfg := gitRun(t, project, "config", "--file", ".gitconfig", "--get-all", "proj.extra")
	if got := strings.Fields(cfg); strings.Join(got, ",") != "../sibling,../other" {
		t.Errorf(".gitconfig 内容不符: %q", got)
	}

	// 提交里只有 .gitconfig
	files := strings.Fields(gitRun(t, project, "show", "--name-only", "--format=", "HEAD"))
	if strings.Join(files, ",") != ".gitconfig" {
		t.Errorf("提交应只含 .gitconfig, 实际 %v", files)
	}
	if !strings.Contains(gitRun(t, project, "log", "-1", "--format=%s"), "../sibling, ../other") {
		t.Errorf("提交说明应列出登记的目录")
	}
	// 无关文件仍只是暂存, 没被带进提交
	if !strings.Contains(gitRun(t, project, "status", "--short"), "A  unrelated.txt") {
		t.Errorf("unrelated.txt 不应被提交")
	}

	// 重复添加: 不新增、不再提交
	before := gitRun(t, project, "rev-parse", "HEAD")
	if err := addDirs([]string{"../sibling"}, true); err != nil {
		t.Fatalf("重复 addDirs: %v", err)
	}
	if after := gitRun(t, project, "rev-parse", "HEAD"); after != before {
		t.Errorf("重复添加不应产生新提交")
	}

	// 项目内目录被拒
	if err := addDirs([]string{"."}, true); err == nil {
		t.Errorf("项目根本身应被拒")
	}
}

func TestAddDirsOutsideGitRepo(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	sibling := filepath.Join(root, "sibling")
	for _, d := range []string{project, sibling} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(project, "files.proj"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	// 临时目录可能位于某个外层 git 仓库之内, 用 GIT_CEILING_DIRECTORIES 截断向上查找
	t.Setenv("GIT_CEILING_DIRECTORIES", root)
	wd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if err := addDirs([]string{sibling}, true); err != nil {
		t.Fatalf("非 git 仓库应只写配置不报错: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, ".gitconfig")); err != nil {
		t.Errorf(".gitconfig 应已写入: %v", err)
	}
}
