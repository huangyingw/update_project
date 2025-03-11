package utils

import (
	"os/exec"
	"strings"
)

// GetCurrentBranch 返回当前Git仓库的分支名
func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// 去除输出中的换行符
	branch := strings.TrimSpace(string(output))
	return branch, nil
}
