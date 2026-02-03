// Package cmd implements the croupier-server CLI using Cobra
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/cli/common"
	"github.com/cuihairu/croupier/internal/nng"
	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/handler"
	"github.com/cuihairu/croupier/services/server/internal/logic/ops"
	"github.com/cuihairu/croupier/services/server/internal/middleware"
	"github.com/cuihairu/croupier/services/server/internal/runtime"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/spf13/cobra"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var (
	Version   = "dev"     // 版本信息，通过 ldflags 设置
	GitCommit = "unknown" // Git 提交哈希
	BuildTime = ""        // 构建时间

	cfgFile          string
	mode             string
	port             int
	debug            bool
	host             string
	logLevel         string
	bootstrapDataDir string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "croupier-server",
	Short: "Croupier 游戏管理服务器",
	Long: `Croupier 是一个三层分布式游戏管理后端系统

- 权限控制层：独立于游戏逻辑的 RBAC/ABAC 系统
- 游戏控制层：函数注册驱动的游戏操作
- 可观察展示层：描述符驱动的 UI 生成

支持多游戏租户、函数注册、负载均衡、审计链、审批工作流等功能。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// 设置版本信息
	rootCmd.Version = Version

	// 全局标志
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "f", "etc/server.yaml", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&mode, "mode", "m", "dev", "运行模式 (dev|prod|test)")
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 0, "覆盖配置文件中的端口 (0=使用配置)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "启用调试模式")
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "覆盖监听主机")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "日志级别 (debug|info|warn|error)")
	defaultBootstrapDir := runtime.DefaultBootstrapDataDir()
	rootCmd.PersistentFlags().StringVar(&bootstrapDataDir, "bootstrap-data-dir", defaultBootstrapDir, "引导数据目录（如 configs），用于加载默认管理员/游戏等 JSON 文件")

	// 版本标志
	rootCmd.Flags().BoolP("version", "V", false, "显示版本信息")
	rootCmd.SetVersionTemplate("{{printf \"%s %s\\n\" .Name .Version}}")

	// 添加子命令
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(versionCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "server",
		Short: "Run server (alias)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer()
		},
	})
}

func runServer() error {
	var c config.Config

	// 加载配置文件
	if cfgFile != "" {
		// Allow ${VAR} expansion in YAML via environment variables.
		conf.MustLoad(cfgFile, &c, conf.UseEnv())
	} else {
		return fmt.Errorf("配置文件是必需的")
	}

	// 覆盖配置文件设置
	if port > 0 {
		c.RestConf.Port = port
	}
	if host != "" {
		c.RestConf.Host = host
	}

	// 根据模式调整配置
	switch mode {
	case "prod":
		c.RestConf.Mode = "prod"
	case "test":
		c.RestConf.Mode = "test"
	default:
		c.RestConf.Mode = "dev"
	}

	// 调试模式设置
	if debug {
		fmt.Println("Debug mode enabled")
		c.RestConf.Mode = "dev"
		if logLevel == "" {
			logLevel = "debug"
		}
	}

	// 日志级别设置
	if logLevel != "" {
		if c.CroupierLog.Level == "" {
			c.CroupierLog.Level = logLevel
		}
	}

	// 初始化日志系统
	if c.CroupierLog.Level == "" {
		c.CroupierLog.Level = "info"
	}
	if c.CroupierLog.Format == "" {
		c.CroupierLog.Format = "console"
	}
	common.SetupLoggerWithFile(
		c.CroupierLog.Level,
		c.CroupierLog.Format,
		c.CroupierLog.Output,
		c.CroupierLog.MaxSize,
		c.CroupierLog.MaxBackups,
		c.CroupierLog.MaxAge,
		c.CroupierLog.Compress,
	)

	// 引导数据目录
	if bootstrapDataDir != "" {
		c.BootstrapData.BaseDir = bootstrapDataDir
	}

	applyRuntimeDefaults(&c)

	// 创建服务上下文
	ctx := svc.NewServiceContext(c)

	// 初始化 Agent Ops 客户端
	ops.InitAgentOpsClient(ctx.RegistryStore)

	// 检查数据库连接
	dbHealth := middleware.NewDBHealth(ctx)
	if err := dbHealth.Ping(); err != nil {
		fmt.Printf("警告: 数据库连接失败: %v\n", err)
		fmt.Println("某些功能可能无法正常工作，请检查数据库配置")
	}

	// 启动 NNG 控制服务器（替代 gRPC）
	go startNNGControlServer(&c, ctx)

	// 创建 REST 服务器
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// 添加认证中间件
	authMiddleware := middleware.NewAuthMiddleware(ctx)
	server.Use(authMiddleware.Handle)

	// 注册路由
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting Croupier Server at %s:%d (mode: %s, debug: %v)...\n",
		c.RestConf.Host, c.RestConf.Port, mode, debug)

	server.Start()
	return nil
}

// startNNGControlServer 启动 NNG 控制服务器（替代 gRPC）
func startNNGControlServer(c *config.Config, svcCtx *svc.ServiceContext) {
	// 解析 NNG 监听地址
	addr := c.GRPC.Addr
	if addr == "" {
		addr = ":19090" // 默认 NNG ControlService 端口（与 SDK 保持一致）
	}
	if addr[0] == ':' {
		addr = "0.0.0.0" + addr
	}

	// 创建 NNG 控制服务器
	nngServer := nng.NewServer(addr, svcCtx.RegistryStore)

	// 启动服务器
	if err := nngServer.Start(); err != nil {
		fmt.Printf("Failed to start NNG Control server: %v\n", err)
		return
	}

	// 获取实际监听地址
	localAddr, _ := nngServer.GetLocalAddr()
	fmt.Printf("Starting NNG ControlService on %s (SDK/Agent registration)...\n", localAddr)
}

func applyRuntimeDefaults(c *config.Config) {
	if c == nil {
		return
	}

	// Allow long-lived connections (SSE, streaming) by keeping HTTP timeout generous.
	if c.RestConf.Timeout == 0 {
		// default go-zero timeout is 3s; bump to 10 minutes for streaming endpoints.
		c.RestConf.Timeout = 600000
	}

	// ✅ Auto-adjust timeout based on SSE configuration to prevent premature disconnection
	validateAndAdjustTimeout(c)

	if strings.TrimSpace(c.Components.DataDir) == "" {
		c.Components.DataDir = "data"
	}

	if strings.EqualFold(strings.TrimSpace(c.Storage.Driver), "file") {
		if strings.TrimSpace(c.Storage.BaseDir) == "" {
			c.Storage.BaseDir = filepath.Join("data", "uploads")
		}
	}
}

// validateAndAdjustTimeout 确保 go-zero timeout > SSE intervals
func validateAndAdjustTimeout(c *config.Config) {
	// SSE 配置默认值（秒）
	updateInterval := 2     // 默认 2 秒
	keepAliveInterval := 30 // 默认 30 秒

	// 读取配置值
	if c.SSE.UpdateInterval > 0 {
		updateInterval = c.SSE.UpdateInterval
	}
	if c.SSE.KeepAliveInterval > 0 {
		keepAliveInterval = c.SSE.KeepAliveInterval
	}

	// 计算最小安全超时：至少 3 倍的 keep-alive 间隔
	// 这样允许至少 2 次 keep-alive + 容错余量
	minSafeTimeout := keepAliveInterval * 3
	currentTimeoutSec := c.RestConf.Timeout / 1000 // 毫秒转秒

	// 如果当前超时小于安全值，自动调整并警告
	if currentTimeoutSec < int64(minSafeTimeout) {
		fmt.Printf("⚠️  警告: go-zero Timeout (%d秒) 小于 SSE KeepAliveInterval (%d秒) 的 3 倍\n",
			currentTimeoutSec, keepAliveInterval)
		fmt.Printf("   自动调整 Timeout 为 %d 秒以防止 SSE 连接过早断开\n", minSafeTimeout)

		c.RestConf.Timeout = int64(minSafeTimeout) * 1000 // 秒转毫秒
	} else {
		// 验证通过，显示配置信息
		fmt.Printf("✅ SSE 配置验证通过:\n")
		fmt.Printf("   - go-zero Timeout: %d 秒\n", currentTimeoutSec)
		fmt.Printf("   - SSE UpdateInterval: %d 秒\n", updateInterval)
		fmt.Printf("   - SSE KeepAliveInterval: %d 秒\n", keepAliveInterval)
		fmt.Printf("   - 安全系数: %.1fx (超时 / KeepAlive)\n", float64(currentTimeoutSec)/float64(keepAliveInterval))
	}
}
