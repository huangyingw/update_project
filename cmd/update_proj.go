package cmd

import (
	"fmt"
	"os"
	"projupdater/utils"
)

func UpdateProj() error {
	// 检查 files.proj 是否存在
	if _, err := os.Stat("files.proj"); os.IsNotExist(err) {
		return fmt.Errorf("当前目录下不存在 files.proj 文件，无法更新项目")
	}

	// 检查是否在远程挂载的文件系统上
	mounted, err := utils.IsRemoteMounted(".")
	if err != nil {
		return err
	}
	if mounted {
		return fmt.Errorf("在远程挂载的文件系统上运行会非常慢，已退出")
	}

	// 运行更新逻辑
	err = utils.RunOnce("do_update_proj", func() error {
		return DoUpdateProj()
	})
	if err != nil {
		return err
	}

	return nil
}
