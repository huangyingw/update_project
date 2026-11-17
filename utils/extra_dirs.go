package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// ExtraDirConfigFile 是项目根下记录额外索引目录的配置文件(随仓库提交, 跨机器同步)
	ExtraDirConfigFile = ".gitconfig"
	// ExtraDirConfigKey 是 .gitconfig 里的多值键: [proj] extra = <相对项目根的路径>
	ExtraDirConfigKey = "proj.extra"
)

// ExtraDir 是一个已解析的额外索引目录
type ExtraDir struct {
	Rel string // 配置里存的值: 相对项目根, 正斜杠, 如 ../sibling
	Abs string // 解析后的绝对路径
}

// ReadExtraDirs 读取项目根 .gitconfig 里登记的额外目录(原样返回相对路径, 不校验存在性)。
// 配置文件或键不存在 → 空切片, 不报错。
func ReadExtraDirs(projectDir string) ([]string, error) {
	configPath := filepath.Join(projectDir, ExtraDirConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}
	cmd := exec.Command("git", "config", "--file", configPath, "--get-all", ExtraDirConfigKey)
	out, err := cmd.Output()
	if err != nil {
		// git config 查不到键时退出码为 1, 视为没有配置
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 %s 的 %s 失败: %w", configPath, ExtraDirConfigKey, err)
	}
	var dirs []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			dirs = append(dirs, line)
		}
	}
	return dirs, nil
}

// ResolveExtraDirs 把配置里的相对路径解析成绝对路径; 不存在或不是目录的条目打 WARN 后跳过。
func ResolveExtraDirs(projectDir string) ([]ExtraDir, error) {
	rels, err := ReadExtraDirs(projectDir)
	if err != nil {
		return nil, err
	}
	// 相对路径是按真实路径(符号链接解析后)算出来的, 解析时也按真实路径拼, 两边口径一致
	projectReal, err := RealPath(projectDir)
	if err != nil {
		return nil, err
	}
	var dirs []ExtraDir
	for _, rel := range rels {
		abs := filepath.Clean(filepath.Join(projectReal, filepath.FromSlash(rel)))
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			fmt.Printf("WARN: 额外目录 %s (%s) 不存在或不是目录, 跳过\n", rel, abs)
			continue
		}
		dirs = append(dirs, ExtraDir{Rel: rel, Abs: abs})
	}
	return dirs, nil
}

// NormalizeExtraDir 把用户给的路径(绝对或相对当前工作目录)归一化成相对项目根的路径。
// 校验: 必须是已存在的目录; 不能是项目根本身或其子目录(那部分本来就会被索引)。
func NormalizeExtraDir(projectDir, input string) (string, error) {
	info, err := os.Stat(input)
	if err != nil {
		return "", fmt.Errorf("目录不存在: %s", input)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("不是目录: %s", input)
	}
	// 两边都解析成真实路径再算相对路径: 用户 cwd 或传入路径经过符号链接(macOS 的 /var → /private/var,
	// ~/Dropbox 之类)时, 单纯的 Abs 会算出一长串 ../, 存进配置就失去意义了
	projectAbs, err := RealPath(projectDir)
	if err != nil {
		return "", err
	}
	dirAbs, err := RealPath(input)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(projectAbs, dirAbs)
	if err != nil {
		return "", fmt.Errorf("无法计算 %s 相对 %s 的路径: %w", dirAbs, projectAbs, err)
	}
	if rel == "." {
		return "", fmt.Errorf("%s 就是项目根目录, 无需添加", input)
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s 在项目根目录之内, 本来就会被索引, 无需添加", input)
	}
	return filepath.ToSlash(rel), nil
}

// AddExtraDir 把相对路径写进项目根 .gitconfig; 已存在则返回 false 不重复写。
func AddExtraDir(projectDir, rel string) (bool, error) {
	existing, err := ReadExtraDirs(projectDir)
	if err != nil {
		return false, err
	}
	for _, e := range existing {
		if e == rel {
			return false, nil
		}
	}
	configPath := filepath.Join(projectDir, ExtraDirConfigFile)
	cmd := exec.Command("git", "config", "--file", configPath, "--add", ExtraDirConfigKey, rel)
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("写入 %s 失败: %w: %s", configPath, err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

// CommitExtraDirConfig 把项目根 .gitconfig 提交进仓库(只提交这一个文件)。
// 项目不是 git 仓库 → 返回 (false, nil), 由调用方提示。
func CommitExtraDirConfig(projectDir string, added []string) (bool, error) {
	check := exec.Command("git", "-C", projectDir, "rev-parse", "--is-inside-work-tree")
	if out, err := check.Output(); err != nil || strings.TrimSpace(string(out)) != "true" {
		return false, nil
	}
	add := exec.Command("git", "-C", projectDir, "add", "-f", "--", ExtraDirConfigFile)
	if out, err := add.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git add %s 失败: %w: %s", ExtraDirConfigFile, err, strings.TrimSpace(string(out)))
	}
	msg := fmt.Sprintf("chore(proj): 登记额外索引目录 %s", strings.Join(added, ", "))
	commit := exec.Command("git", "-C", projectDir, "commit", "-m", msg, "--", ExtraDirConfigFile)
	commit.Stdout = os.Stdout
	commit.Stderr = os.Stderr
	if err := commit.Run(); err != nil {
		return false, fmt.Errorf("git commit %s 失败: %w", ExtraDirConfigFile, err)
	}
	return true, nil
}

// RealPath 返回绝对且解析过符号链接的路径
func RealPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("解析路径 %s 失败: %w", path, err)
	}
	return real, nil
}
