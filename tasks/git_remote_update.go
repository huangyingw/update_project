package tasks

import (
	"os/exec"
)

func GitRemoteUpdate() error {
	cmd := exec.Command("git", "remote", "update")
	return cmd.Run()
}
