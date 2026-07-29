package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证配置文件",
	Long:  `验证指定配置文件的语法和完整性，确保所有必需的配置项都已设置。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfigFile(cfgFile)
		if err != nil {
			return err
		}

		fmt.Printf("✓ 配置文件验证通过: %s\n", cfgFile)
		fmt.Printf("  - 服务地址: %s:%d\n", c.Server.Host, c.Server.Port)
		fmt.Printf("  - 运行模式: %s\n", c.Server.Mode)
		fmt.Printf("  - 数据库: %s\n", c.Database.Driver)
		if c.Database.MultiGame {
			fmt.Printf("  - 多游戏分库: 启用 (database-per-game)\n")
		}
		fmt.Printf("  - JWT密钥: %s\n", maskIfSet(c.Auth.JWTSecret))

		return nil
	},
}

// maskIfSet 敏感信息掩码显示
func maskIfSet(s string) string {
	if s == "" {
		return "未设置"
	}
	if len(s) <= 8 {
		return "已设置"
	}
	return "已设置 (***" + s[len(s)-4:] + ")"
}
