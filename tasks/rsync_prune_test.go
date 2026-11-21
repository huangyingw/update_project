package tasks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// 建一棵覆盖各种边界的小树, 返回项目根与 files.proj 风格的条目(已去引号)
func buildRsyncPruneTree(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("keep.go", "package main\n")
	write("prunefix.conf", "x\n")
	write("sub/prunefix.conf", "x\n")
	write("sub/deep/a.gdio.diff", "x\n")
	write("sub/name with space.cursorrules", "x\n")
	write(".cursorignore", "x\n")
	write("worktrees/feature/x/prunefix.conf", "x\n") // 不在 files.proj 里, 不影响结果
	write("dir.cursorrules/inner.txt", "x\n")
	// 超过 9000k 的文件: find -size -9000k 不收, 所以不剔除
	big := filepath.Join(root, "sub/big/prunefix.conf")
	if err := os.MkdirAll(filepath.Dir(big), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(10 * 1024 * 1024); err != nil {
		t.Fatal(err)
	}
	f.Close()
	// 符号链接: -type f 不收
	if err := os.Symlink("keep.go", filepath.Join(root, "link.cursorrules")); err != nil {
		t.Fatal(err)
	}

	files := []string{
		"./keep.go",
		"./prunefix.conf",
		"./sub/prunefix.conf",
		"./sub/deep/a.gdio.diff",
		"./sub/name with space.cursorrules",
		"./.cursorignore",
		"./dir.cursorrules", // 目录
		"./dir.cursorrules/inner.txt",
		"./sub/big/prunefix.conf",
		"./link.cursorrules",
		"./gone/prunefix.conf",     // 已删除的文件
		"sub/prunefix.conf",        // 非 "./" 开头, 整树遍历的输出不会是这种写法
		"./sub/../prunefix.conf",   // 非规范写法
		"../sibling/prunefix.conf", // 项目外
	}
	return root, files
}

var rsyncPrunePatterns = []string{
	"*/.cursorignore",
	"*/.cursorrules",
	"*/prunefix.conf",
	"\"*.cursorrules\"",
	"*/update_proj.gdio.diff",
	".*gdio\\.diff$",
	"*gdio.diff",
}

func sortedLines(b []byte) []string {
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	sort.Strings(lines)
	return lines
}

// 旧做法: 从 "." 遍历整棵树, 再与 files 求交——comm -23 实际只会用到这部分
func treeWalkMatches(t *testing.T, root string, files, patterns []string) []string {
	t.Helper()
	args := []string{".", "("}
	for i, p := range patterns {
		if i > 0 {
			args = append(args, "-o")
		}
		args = append(args, "-path", strings.Trim(p, "\""))
	}
	args = append(args, ")", "-type", "f", "-size", "-9000k")
	cmd := exec.Command("find", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("整树 find 失败: %v", err)
	}
	listed := map[string]bool{}
	for _, f := range files {
		listed[f] = true
	}
	var hit []string
	for _, l := range sortedLines(out) {
		if listed[l] {
			hit = append(hit, l)
		}
	}
	return hit
}

func TestFindListedMatchesEqualsTreeWalk(t *testing.T) {
	root, files := buildRsyncPruneTree(t)
	wd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	out, err := findListedMatches(files, rsyncPrunePatterns)
	if err != nil {
		t.Fatalf("findListedMatches: %v", err)
	}
	got := sortedLines(out)
	want := treeWalkMatches(t, root, files, rsyncPrunePatterns)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("结果与整树遍历不一致\n got: %q\nwant: %q", got, want)
	}
	// 防止两边一起退化成空结果
	for _, must := range []string{"./prunefix.conf", "./sub/prunefix.conf", "./sub/deep/a.gdio.diff", "./sub/name with space.cursorrules", "./.cursorignore"} {
		found := false
		for _, g := range got {
			if g == must {
				found = true
			}
		}
		if !found {
			t.Errorf("应剔除 %s, 实际结果 %q", must, got)
		}
	}
}

// 条目多到要分批调用 find 时, 每批的结果都收齐
func TestFindListedMatchesBatches(t *testing.T) {
	root := t.TempDir()
	wd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	var files []string
	want := 0
	pad := strings.Repeat("p", 200)
	for i := 0; i < 1500; i++ { // 约 300 KB 参数, 至少分 3 批
		name := filepath.Join("d", fmt.Sprintf("%s_%04d", pad, i))
		if i%3 == 0 {
			name += ".skip"
			want++
		}
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		files = append(files, "./"+name)
	}
	out, err := findListedMatches(files, []string{"*.skip"})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(sortedLines(out)); got != want {
		t.Fatalf("分批后命中 %d 条, 应为 %d", got, want)
	}
}

// 端到端: rsync.files 剔掉 prunefix.rsync 命中的条目与 ../ 条目, worktrees/ 下不在 files.proj 的文件不受影响
func TestGenerateRsyncFilesPrunesListedOnly(t *testing.T) {
	root, files := buildRsyncPruneTree(t)
	var proj []string
	for _, f := range files {
		proj = append(proj, "\""+f+"\"")
	}
	if err := os.WriteFile(filepath.Join(root, "files.proj"), []byte(strings.Join(proj, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "prunefix.rsync"), []byte(strings.Join(rsyncPrunePatterns, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if err := GenerateRsyncFiles(); err != nil {
		t.Fatalf("GenerateRsyncFiles: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "rsync.files"))
	if err != nil {
		t.Fatal(err)
	}
	got := sortedLines(content)
	want := []string{
		"./dir.cursorrules",
		"./dir.cursorrules/inner.txt",
		"./gone/prunefix.conf",
		"./keep.go",
		"./link.cursorrules",
		"./sub/../prunefix.conf",
		"./sub/big/prunefix.conf",
		"sub/prunefix.conf",
	}
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rsync.files 不对\n got: %q\nwant: %q", got, want)
	}
}
