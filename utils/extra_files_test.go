package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveExtraFiles(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	project := filepath.Join(root, "proj")
	cache := filepath.Join(home, ".cache", "app")
	writeFile(t, filepath.Join(cache, "reminders.md"), "r\n")
	writeFile(t, filepath.Join(cache, "todo-a.md"), "a\n")
	writeFile(t, filepath.Join(cache, "todo-b.md"), "b\n")
	writeFile(t, filepath.Join(cache, "todo.state.json"), "{}\n")
	writeFile(t, filepath.Join(cache, "empty.md"), "")
	writeFile(t, filepath.Join(cache, "bin.md"), "a\x00b")
	writeFile(t, filepath.Join(project, "files.proj"), "")

	// 没有 .proj.local → 空, 不报错
	if got, err := ResolveExtraFiles(project); err != nil || len(got) != 0 {
		t.Fatalf("无配置时 got %v, %v", got, err)
	}

	for _, v := range []string{
		"~/.cache/app/reminders.md",
		filepath.Join(cache, "todo-*.md"),
		filepath.Join(cache, "todo-a.md"), // 与通配重复, 只收一次
		filepath.Join(cache, "gone.md"),   // 不存在
		filepath.Join(cache, "nomatch-*.md"),
		filepath.Join(cache, "empty.md"),
		filepath.Join(cache, "bin.md"),
		cache,           // 目录
		"relative/x.md", // 相对路径
	} {
		if _, err := AddExtraFile(project, v); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ResolveExtraFiles(project)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(cache, "reminders.md"),
		filepath.Join(cache, "todo-a.md"),
		filepath.Join(cache, "todo-b.md"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestAddExtraFileDedupe(t *testing.T) {
	project := t.TempDir()
	for i, wantAdded := range []bool{true, false} {
		added, err := AddExtraFile(project, "/x/a.md")
		if err != nil || added != wantAdded {
			t.Fatalf("第 %d 次: added=%v err=%v", i+1, added, err)
		}
	}
	vals, _ := ReadExtraFiles(project)
	if !reflect.DeepEqual(vals, []string{"/x/a.md"}) {
		t.Errorf("got %v", vals)
	}
}

func TestNormalizeExtraFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	writeFile(t, filepath.Join(root, "a.md"), "a\n")

	if got, err := NormalizeExtraFile("~/a.md"); err != nil || got != filepath.Join(root, "a.md") {
		t.Errorf("~/a.md → %q, %v", got, err)
	}
	if got, err := NormalizeExtraFile(filepath.Join(root, "*.md")); err != nil || got != filepath.Join(root, "*.md") {
		t.Errorf("通配应原样存: %q, %v", got, err)
	}
	if _, err := NormalizeExtraFile(filepath.Join(root, "missing.md")); err == nil {
		t.Error("不存在的文件应报错")
	}
	if _, err := NormalizeExtraFile(root); err == nil {
		t.Error("目录应报错")
	}
	if _, err := NormalizeExtraFile(filepath.Join(root, "[.md")); err == nil {
		t.Error("坏通配应报错")
	}
}

func TestEnsureLocalConfigExcluded(t *testing.T) {
	// 非 git 目录: 什么都不做
	if added, err := EnsureLocalConfigExcluded(t.TempDir()); added || err != nil {
		t.Fatalf("非 git 目录 added=%v err=%v", added, err)
	}

	repo := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	writeFile(t, excludePath, "# 原有内容, 末尾无换行")
	for i, want := range []bool{true, false} {
		added, err := EnsureLocalConfigExcluded(repo)
		if err != nil || added != want {
			t.Fatalf("第 %d 次 added=%v err=%v", i+1, added, err)
		}
	}
	content, _ := os.ReadFile(excludePath)
	if string(content) != "# 原有内容, 末尾无换行\n/.proj.local\n" {
		t.Errorf("exclude 内容不符: %q", content)
	}
	writeFile(t, filepath.Join(repo, LocalConfigFile), "[proj]\n")
	out, _ := exec.Command("git", "-C", repo, "status", "--porcelain").Output()
	if strings.Contains(string(out), LocalConfigFile) {
		t.Errorf("%s 应被忽略, git status: %s", LocalConfigFile, out)
	}
}

// 入库 .gitconfig: [proj] file 所有检出都收; [proj "tower"] file 只有当前分支是 tower 的检出收;
// .proj.local 与之叠加去重。
func TestResolveExtraFilesTrackedBranchScoped(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	cache := filepath.Join(home, ".cache", "tower")
	for _, f := range []string{"shared.md", "reminders.md", "todo-self.md", "todo-action.md", "local.md"} {
		writeFile(t, filepath.Join(cache, f), f+"\n")
	}
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "tower")
	cfg := filepath.Join(repo, ExtraDirConfigFile)
	for _, kv := range [][2]string{
		{ExtraFileConfigKey, "~/.cache/tower/shared.md"},
		{BranchExtraFileConfigKey("tower"), "~/.cache/tower/reminders.md"},
		{BranchExtraFileConfigKey("tower"), "~/.cache/tower/todo-*.md"},
		{BranchExtraFileConfigKey("feature/x"), "~/.cache/tower/local.md"},
	} {
		if out, err := exec.Command("git", "config", "--file", cfg, "--add", kv[0], kv[1]).CombinedOutput(); err != nil {
			t.Fatalf("写 .gitconfig: %v %s", err, out)
		}
	}
	if _, err := AddExtraFile(repo, filepath.Join(cache, "local.md")); err != nil {
		t.Fatal(err)
	}
	p := func(f string) string { return filepath.Join(cache, f) }

	got, err := ResolveExtraFiles(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{p("local.md"), p("reminders.md"), p("shared.md"), p("todo-action.md"), p("todo-self.md")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tower 分支:\n got %v\nwant %v", got, want)
	}

	// 换到别的分支: tower 子节不收, feature/x 子节收
	git("symbolic-ref", "HEAD", "refs/heads/feature/x")
	got, err = ResolveExtraFiles(repo)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{p("local.md"), p("shared.md")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("feature/x 分支:\n got %v\nwant %v", got, want)
	}

	// 不是 git 仓库: 只收不带分支的与 .proj.local
	plain := filepath.Join(root, "plain")
	writeFile(t, filepath.Join(plain, ExtraDirConfigFile), "[proj \"tower\"]\n\tfile = ~/.cache/tower/reminders.md\n[proj]\n\tfile = ~/.cache/tower/shared.md\n")
	got, err = ResolveExtraFiles(plain)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{p("shared.md")}) {
		t.Errorf("非 git 目录 got %v", got)
	}
}
