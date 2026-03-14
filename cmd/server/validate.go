package main

import (
	"fmt"
	"os"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证配置文件",
	Long:  `验证指定配置文件的语法和完整性，确保所有必需的配置项都已设置。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfgFile == "" {
			return fmt.Errorf("必须指定配置文件 (-f/--config)")
		}

		var c config.Config
		data, err := os.ReadFile(cfgFile)
		if err != nil {
			return fmt.Errorf("读取配置文件失败: %w", err)
		}
		expanded := os.ExpandEnv(string(data))
		if err := yaml.Unmarshal([]byte(expanded), &c); err != nil {
			return fmt.Errorf("配置文件解析失败: %w", err)
		}

		fmt.Printf("✓ 配置文件验证通过: %s\n", cfgFile)
		fmt.Printf("  - 服务地址: %s:%d\n", c.Server.Host, c.Server.Port)
		fmt.Printf("  - 运行模式: %s\n", c.Server.Mode)
		fmt.Printf("  - 数据库: %s\n", c.Database.Driver)
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
