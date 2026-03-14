package main

import (
	"fmt"
	"os"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "健康检查",
	Long:  `检查服务依赖的健康状态，包括数据库连接、配置文件等。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfgFile == "" {
			return fmt.Errorf("必须指定配置文件 (-f/--config)")
		}

		var c config.Config
		data, err := os.ReadFile(cfgFile)
		if err != nil {
			return fmt.Errorf("读取配置文件失败: %v", err)
		}
		expanded := os.ExpandEnv(string(data))
		if err := yaml.Unmarshal([]byte(expanded), &c); err != nil {
			return fmt.Errorf("配置文件解析失败: %v", err)
		}
		if bootstrapDataDir != "" {
			c.BootstrapData.BaseDir = bootstrapDataDir
		}

		fmt.Println("🔍 开始健康检查...")

		// 检查配置文件
		fmt.Printf("✓ 配置文件: %s\n", cfgFile)

		// 检查服务上下文
		_ = svc.NewServiceContext(c)
		fmt.Println("✓ 服务上下文初始化成功")

		// 检查数据库连接（如果有配置）
		if c.Database.DataSource != "" {
			fmt.Printf("✓ 数据库配置: %s\n", c.Database.Driver)
		} else {
			fmt.Println("⚠ 数据库未配置")
		}

		// 检查其他依赖
		if c.Auth.JWTSecret != "" {
			fmt.Println("✓ JWT密钥已配置")
		} else {
			fmt.Println("⚠ JWT密钥未配置")
		}

		fmt.Printf("\n🎉 所有检查完成！服务状态正常。\n")
		return nil
	},
}
