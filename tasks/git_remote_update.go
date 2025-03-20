package tasks

import (
	"fmt"
	"os/exec"
)

func GitRemoteUpdate() error {
	cmd := exec.Command("git", "remote", "update")
	err := cmd.Run()
	if err != nil {
		fmt.Printf("警告: git remote update 失败: %v\n", err)
		return nil
	}
	return nil
}
