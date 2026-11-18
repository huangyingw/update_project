package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"projupdater/utils"
)

var addFileCmd = &cobra.Command{
	Use:   "add-file <文件或通配>...",
	Short: "把项目外的单个文件登记进当前检出的 .proj.local(只属于本机, 不提交), 之后 update_proj 会一并索引",
	Long: `把项目外的单个文件登记到当前项目(当前目录须有 files.proj)根下的 .proj.local:

    [proj]
        file = /home/me/.cache/app/report.md
        file = /home/me/.cache/app/todo-*.md

与 add-dir 不同: 只收点名的文件, 不扫整个目录; 存绝对路径(也可手写 ~/ 开头),
.proj.local 只属于这一个检出(本机), 不提交——并会把 /.proj.local 写进仓库的 info/exclude。
含通配(* ? [)的值原样存, 每次 update_proj 时展开, 新出现的同名规律文件自动收进来。
之后运行 update_proj, 这些文件以绝对路径并入 files.proj(不进 rsync.files)。
查看: git config --file .proj.local --get-all ` + utils.ExtraFileConfigKey + `
删除: git config --file .proj.local --unset ` + utils.ExtraFileConfigKey + ` '<原值>'`,
	Args:          cobra.MinimumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return addFiles(args)
	},
}

func init() {
	rootCmd.AddCommand(addFileCmd)
}

func addFiles(inputs []string) error {
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "files.proj")); os.IsNotExist(err) {
		return fmt.Errorf("当前目录下不存在 files.proj 文件, 不是项目根目录")
	}
	// 先全部校验, 有一个不合法就整体不写
	var values []string
	for _, input := range inputs {
		v, err := utils.NormalizeExtraFile(input)
		if err != nil {
			return err
		}
		values = append(values, v)
	}
	for i, v := range values {
		ok, err := utils.AddExtraFile(projectDir, v)
		if err != nil {
			return err
		}
		if ok {
			fmt.Printf("INFO: 已登记 %s → %s (%s)\n", inputs[i], v, utils.LocalConfigFile)
		} else {
			fmt.Printf("INFO: %s 已登记, 跳过\n", v)
		}
	}
	excluded, err := utils.EnsureLocalConfigExcluded(projectDir)
	if err != nil {
		fmt.Printf("WARN: 把 /%s 写进 info/exclude 失败: %v(它不该被提交)\n", utils.LocalConfigFile, err)
	} else if excluded {
		fmt.Printf("INFO: 已把 /%s 写进仓库的 info/exclude\n", utils.LocalConfigFile)
	}
	return nil
}
