package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "projupdater",
	Short: "ProjUpdater 是一个用于更新项目索引的工具",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdate()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func runUpdate() error {
	// 调用更新逻辑
	return UpdateProj()
}
