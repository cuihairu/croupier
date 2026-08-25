package main

import (
	"fmt"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/spf13/cobra"
)

// dbCmd groups database maintenance subcommands (versioned migrations).
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "数据库维护命令",
}

var dbFanoutDryRun bool

var dbFanoutCmd = &cobra.Command{
	Use:   "fanout",
	Short: "批量滚动版本化迁移（meta + 所有 game_envs 注册的游戏库）",
	Long: `按 game_envs 注册表逐库追平版本化迁移并输出报告（阶段 5，
见 docs/architecture/database-migration-strategy.md）。

先迁移 meta 库，再逐个打开已注册的游戏库执行追平。注册表引用但
物理不存在的库报告为 missing-database（运行时懒建，不算错误）。

--dry-run 仅报告各库当前版本，不执行任何 DDL。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfigFile(cfgFile)
		if err != nil {
			return err
		}
		reports, err := svc.RunMigrationFanout(cmd.Context(), cfg, dbFanoutDryRun)
		fmt.Print(svc.FormatFanoutReports(reports))
		if err != nil {
			return err
		}
		for _, r := range reports {
			if r.Status == svc.FanoutStatusError {
				return svc.ErrFanoutFailures
			}
		}
		return nil
	},
}

func init() {
	dbFanoutCmd.Flags().BoolVar(&dbFanoutDryRun, "dry-run", false, "仅报告版本，不执行迁移")
	dbCmd.AddCommand(dbFanoutCmd)
	rootCmd.AddCommand(dbCmd)
}
