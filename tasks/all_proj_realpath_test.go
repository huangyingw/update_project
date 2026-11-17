package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 从符号链接路径(如 ~/myproject → /Volumes/workdata/myproject)进入项目跑 update_proj 时,
// ~/all.proj 记实际路径; 旧版本按链接写法追加过的同一项目条目被换掉, 其余条目原样保留。
func TestGenerateFileIndexRecordsRealPathInAllProj(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)

	volume := filepath.Join(root, "volume", "myproject")
	project := filepath.Join(volume, "git", "proj")
	for _, d := range []string{project, home} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(home, "myproject")
	if err := os.Symlink(volume, link); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(project, "files.proj"), "")
	write(filepath.Join(project, "main.go"), "package main\n")
	for _, c := range []string{"prunefix.conf", "prunefile.conf", "includefile.conf"} {
		write(filepath.Join(project, c), "")
	}

	linkProject := filepath.Join(link, "git", "proj")
	realProject, err := filepath.EvalSymlinks(project) // macOS 的 TempDir 本身在 /var → /private/var 链接下
	if err != nil {
		t.Fatal(err)
	}
	linkEntry := `"` + linkProject + `/files.proj"`
	realEntry := `"` + realProject + `/files.proj"`
	otherEntry := `"/elsewhere/other/files.proj"`
	allProj := filepath.Join(home, "all.proj")
	write(allProj, otherEntry+"\n"+linkEntry+"\n")

	wd, _ := os.Getwd()
	if err := os.Chdir(linkProject); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	// shell 里 cd 进链接路径后 $PWD 就是链接写法, os.Getwd 会照它返回
	t.Setenv("PWD", linkProject)
	if got, _ := os.Getwd(); got != linkProject {
		t.Fatalf("前置条件不成立: os.Getwd()=%q, 期望链接写法 %q", got, linkProject)
	}

	if err := GenerateFileIndex(); err != nil {
		t.Fatalf("GenerateFileIndex: %v", err)
	}

	content, err := os.ReadFile(allProj)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(content)), "\n")
	want := []string{otherEntry, realEntry}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("all.proj 内容不符:\n got: %q\nwant: %q", got, want)
	}
}

func TestAppendUniqueLine(t *testing.T) {
	read := func(t *testing.T, p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	t.Run("新建文件", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "sub", "all.proj")
		if err := appendUniqueLine(p, "b"); err != nil {
			t.Fatal(err)
		}
		if got := read(t, p); got != "b\n" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("已存在且无旧行时不改写文件", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "all.proj")
		orig := "z\nb\n" // 故意不排序: 不改写就保持原样
		if err := os.WriteFile(p, []byte(orig), 0644); err != nil {
			t.Fatal(err)
		}
		if err := appendUniqueLine(p, "b", "absent"); err != nil {
			t.Fatal(err)
		}
		if got := read(t, p); got != orig {
			t.Errorf("got %q, want %q", got, orig)
		}
	})

	t.Run("已存在但有旧行时删旧行", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "all.proj")
		if err := os.WriteFile(p, []byte("c\nold\nb\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := appendUniqueLine(p, "b", "old"); err != nil {
			t.Fatal(err)
		}
		if got := read(t, p); got != "b\nc\n" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("旧行与新行相同时不删", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "all.proj")
		if err := appendUniqueLine(p, "b", "b"); err != nil {
			t.Fatal(err)
		}
		if got := read(t, p); got != "b\n" {
			t.Errorf("got %q", got)
		}
	})
}
