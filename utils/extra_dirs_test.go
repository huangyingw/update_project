package utils

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeExtraDir(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	sibling := filepath.Join(root, "sibling")
	inside := filepath.Join(project, "sub")
	for _, d := range []string{project, sibling, inside} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "afile"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	// 相对路径按当前工作目录解析: 切到 project 下, 用相对写法
	wd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"绝对路径", sibling, "../sibling"},
		{"相对路径", "../sibling", "../sibling"},
		{"带尾斜杠", sibling + "/", "../sibling"},
		{"父目录", root, ".."},
	}
	for _, c := range cases {
		got, err := NormalizeExtraDir(project, c.input)
		if err != nil {
			t.Errorf("%s: 不应报错: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}

	bad := []struct {
		name  string
		input string
	}{
		{"项目根本身", project},
		{"项目内子目录", inside},
		{"不存在", filepath.Join(root, "nope")},
		{"不是目录", filepath.Join(root, "afile")},
	}
	for _, c := range bad {
		if _, err := NormalizeExtraDir(project, c.input); err == nil {
			t.Errorf("%s: 应报错", c.name)
		}
	}
}

func TestReadAddResolveExtraDirs(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	sibling := filepath.Join(root, "sibling")
	for _, d := range []string{project, sibling} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// 没有 .gitconfig → 空
	got, err := ReadExtraDirs(project)
	if err != nil || len(got) != 0 {
		t.Fatalf("无配置时应为空: %v %v", got, err)
	}

	added, err := AddExtraDir(project, "../sibling")
	if err != nil || !added {
		t.Fatalf("首次添加应成功: added=%v err=%v", added, err)
	}
	added, err = AddExtraDir(project, "../sibling")
	if err != nil || added {
		t.Fatalf("重复添加应跳过: added=%v err=%v", added, err)
	}
	added, err = AddExtraDir(project, "../missing")
	if err != nil || !added {
		t.Fatalf("添加第二项应成功: added=%v err=%v", added, err)
	}

	got, err = ReadExtraDirs(project)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"../sibling", "../missing"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ReadExtraDirs got %v, want %v", got, want)
	}

	// 不存在的目录被跳过
	resolved, err := ResolveExtraDirs(project)
	if err != nil {
		t.Fatal(err)
	}
	siblingReal, _ := filepath.EvalSymlinks(sibling) // macOS 的 /var 是 /private/var 的符号链接
	if len(resolved) != 1 || resolved[0].Rel != "../sibling" || resolved[0].Abs != siblingReal {
		t.Errorf("ResolveExtraDirs got %+v", resolved)
	}
}

func TestFilterOutsideProject(t *testing.T) {
	in := []string{"./a.go", "..", "../sibling/b.go", "..hidden/c.go", "sub/../d.go"}
	want := []string{"./a.go", "..hidden/c.go", "sub/../d.go"}
	if got := FilterOutsideProject(in); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
