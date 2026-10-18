package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"projupdater/utils"
)

var addDirNoCommit bool

var addDirCmd = &cobra.Command{
	Use:   "add-dir <目录>...",
	Short: "把一个或多个额外目录登记进当前项目的 .gitconfig, 之后 update_proj 会一并索引",
	Long: `把额外目录登记到当前项目(当前目录须有 files.proj)根下的 .gitconfig:

    [proj]
        extra = ../sibling

无论传入绝对路径还是相对路径, 一律存成相对项目根的路径, 便于跨机器同步;
相对路径按当前工作目录解析。登记后默认把 .gitconfig 提交进仓库(只提交这一个文件)。
之后运行 update_proj, 这些目录里的文本文件会以 "../sibling/..." 的形式并入 files.proj。
查看已登记的目录: git config --file .gitconfig --get-all ` + utils.ExtraDirConfigKey,
	Args: cobra.MinimumNArgs(1),
	// 校验失败(目录不存在 / 在项目内)是用户输入问题, 不打整段用法; 错误由 main 统一打印一次
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return addDirs(args, !addDirNoCommit)
	},
}

func init() {
	addDirCmd.Flags().BoolVar(&addDirNoCommit, "no-commit", false, "只写 .gitconfig, 不提交")
	rootCmd.AddCommand(addDirCmd)
}

func addDirs(inputs []string, commit bool) error {
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "files.proj")); os.IsNotExist(err) {
		return fmt.Errorf("当前目录下不存在 files.proj 文件, 不是项目根目录")
	}

	var added []string
	for _, input := range inputs {
		rel, err := utils.NormalizeExtraDir(projectDir, input)
		if err != nil {
			return err
		}
		ok, err := utils.AddExtraDir(projectDir, rel)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Printf("INFO: %s 已登记为 %s, 跳过\n", input, rel)
			continue
		}
		fmt.Printf("INFO: 已登记 %s → %s (%s)\n", input, rel, utils.ExtraDirConfigFile)
		added = append(added, rel)
	}
	if len(added) == 0 {
		fmt.Println("INFO: 没有新增目录, 不提交")
		return nil
	}
	if !commit {
		return nil
	}
	committed, err := utils.CommitExtraDirConfig(projectDir, added)
	if err != nil {
		return err
	}
	if !committed {
		fmt.Printf("WARN: %s 不是 git 仓库, 已写入 %s 但未提交\n", projectDir, utils.ExtraDirConfigFile)
		return nil
	}
	fmt.Printf("INFO: 已提交 %s: %s\n", utils.ExtraDirConfigFile, strings.Join(added, ", "))
	return nil
}
