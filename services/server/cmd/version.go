package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示 Croupier Server 版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		PrintVersionInfo()
	},
}

// PrintVersionInfo 打印版本信息
func PrintVersionInfo() {
	fmt.Printf("Croupier Server %s\n", Version)
	if GitCommit != "unknown" {
		fmt.Printf("Git commit: %s\n", GitCommit)
	}
	if BuildTime != "" {
		fmt.Printf("Build time: %s\n", BuildTime)
	}
}
