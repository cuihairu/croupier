package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示 Croupier Ingest 版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		PrintVersionInfo()
	},
}

// PrintVersionInfo prints ingest version metadata.
func PrintVersionInfo() {
	fmt.Printf("Croupier Ingest %s\n", Version)
	if GitCommit != "unknown" {
		fmt.Printf("Git commit: %s\n", GitCommit)
	}
	if BuildTime != "" {
		fmt.Printf("Build time: %s\n", BuildTime)
	}
}
