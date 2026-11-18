package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// LocalConfigFile 是项目根下只属于本机检出的配置文件(git config 格式, 不提交; add-file 会把它写进 info/exclude)
	LocalConfigFile = ".proj.local"
	// ExtraFileConfigKey 是 .proj.local 里的多值键: [proj] file = <绝对路径 / ~/路径 / 通配>
	ExtraFileConfigKey = "proj.file"
)

// ReadExtraFiles 读取项目根 .proj.local 里登记的单个文件(原样返回, 不展开不校验)。
// 配置文件或键不存在 → 空切片, 不报错。
func ReadExtraFiles(projectDir string) ([]string, error) {
	configPath := filepath.Join(projectDir, LocalConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}
	out, err := exec.Command("git", "config", "--file", configPath, "--get-all", ExtraFileConfigKey).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 %s 的 %s 失败: %w", configPath, ExtraFileConfigKey, err)
	}
	var values []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			values = append(values, line)
		}
	}
	return values, nil
}

// expandHome 把开头的 ~ 换成 $HOME
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		return filepath.Join(os.Getenv("HOME"), strings.TrimPrefix(p, "~"))
	}
	return p
}

func hasGlobMeta(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

// isTextFile 与主目录扫描(find -size +0 … grep -Il "")口径一致: 非空、前 8000 字节里没有 NUL
func isTextFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8000)
	n, err := io.ReadFull(f, buf)
	if n == 0 {
		return false
	}
	if err != nil && err != io.ErrUnexpectedEOF {
		return false
	}
	return !bytes.Contains(buf[:n], []byte{0})
}

// ResolveExtraFiles 把 .proj.local 登记的值展开成文件绝对路径(展开 ~ 与通配), 去重排序。
// 不存在 / 通配没匹配 / 不是普通文件 / 不是文本文件的条目打 WARN 后跳过, 不报错——
// 登记的多是别的程序定时重写的文件, 偶尔不在不应让整个索引失败。
func ResolveExtraFiles(projectDir string) ([]string, error) {
	values, err := ReadExtraFiles(projectDir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var files []string
	for _, v := range values {
		p := expandHome(v)
		if !filepath.IsAbs(p) {
			fmt.Printf("WARN: %s 里的 %s = %s 不是绝对路径(或 ~/ 开头), 跳过\n", LocalConfigFile, ExtraFileConfigKey, v)
			continue
		}
		p = filepath.Clean(p)
		matches := []string{p}
		if hasGlobMeta(p) {
			if matches, err = filepath.Glob(p); err != nil {
				fmt.Printf("WARN: 通配 %s 写法有误: %v, 跳过\n", v, err)
				continue
			}
			if len(matches) == 0 {
				fmt.Printf("WARN: 通配 %s 没有匹配到文件, 跳过\n", v)
				continue
			}
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil {
				fmt.Printf("WARN: 登记的文件 %s 不存在, 跳过(改名了就用 update_proj add-file 登记新路径并删掉旧条目)\n", m)
				continue
			}
			if !info.Mode().IsRegular() {
				fmt.Printf("WARN: 登记的 %s 不是普通文件, 跳过\n", m)
				continue
			}
			if !isTextFile(m) {
				fmt.Printf("WARN: 登记的 %s 为空或不是文本文件, 跳过\n", m)
				continue
			}
			if !seen[m] {
				seen[m] = true
				files = append(files, m)
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

// NormalizeExtraFile 把用户输入转成存进 .proj.local 的值: 绝对、清理过的路径(相对路径按 cwd 解析, ~ 展开)。
// 不含通配的路径必须是已存在的普通文件; 含通配的原样存, 由每次索引时展开。
func NormalizeExtraFile(input string) (string, error) {
	p, err := filepath.Abs(expandHome(input))
	if err != nil {
		return "", err
	}
	if hasGlobMeta(p) {
		if _, err := filepath.Match(p, ""); err != nil {
			return "", fmt.Errorf("通配 %s 写法有误: %w", input, err)
		}
		return p, nil
	}
	info, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("文件 %s 不存在: %w", input, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s 不是普通文件(整个目录用 add-dir 登记)", input)
	}
	return p, nil
}

// AddExtraFile 把值追加进 .proj.local(已存在则不重复写), 返回是否新增
func AddExtraFile(projectDir, value string) (bool, error) {
	existing, err := ReadExtraFiles(projectDir)
	if err != nil {
		return false, err
	}
	for _, e := range existing {
		if e == value {
			return false, nil
		}
	}
	configPath := filepath.Join(projectDir, LocalConfigFile)
	if out, err := exec.Command("git", "config", "--file", configPath, "--add", ExtraFileConfigKey, value).CombinedOutput(); err != nil {
		return false, fmt.Errorf("写入 %s 失败: %v %s", configPath, err, out)
	}
	return true, nil
}

// EnsureLocalConfigExcluded 把 /.proj.local 写进仓库的 info/exclude(各 worktree 共用、本身不入库),
// 免得它出现在 git status 里被误提交。不是 git 仓库则什么都不做。返回是否新写入。
func EnsureLocalConfigExcluded(projectDir string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-path", "info/exclude")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	excludePath := strings.TrimSpace(string(out))
	pattern := "/" + LocalConfigFile
	content, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == pattern {
			return false, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		return false, err
	}
	prefix := ""
	if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
		prefix = "\n"
	}
	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.WriteString(prefix + pattern + "\n"); err != nil {
		return false, err
	}
	return true, nil
}
