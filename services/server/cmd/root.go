// Package cmd implements the croupier-server CLI using Cobra
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/platform/control"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	serverv1 "github.com/cuihairu/croupier/pkg/pb/croupier/server/v1"
	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/handler"
	"github.com/cuihairu/croupier/services/server/internal/logic/ops"
	"github.com/cuihairu/croupier/services/server/internal/middleware"
	"github.com/cuihairu/croupier/services/server/internal/runtime"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/spf13/cobra"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
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

	// 启动 gRPC 服务器
	go startGRPCServer(&c, ctx)

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

// startGRPCServer 启动 gRPC 服务器
func startGRPCServer(c *config.Config, ctx *svc.ServiceContext) {
	// 解析 gRPC 地址
	addr := c.GRPC.Addr
	if addr == "" {
		addr = ":18443" // 默认地址
	}
	if addr[0] == ':' {
		addr = "0.0.0.0" + addr
	}

	// 创建 gRPC 服务器选项
	var opts []grpc.ServerOption

	// 配置 TLS (自动生成或使用配置的证书)
	tlsOpt, err := tlsutil.EnsureServerTLSCredentials(tlsutil.ServerTLSConfig{
		CertFile:  strings.TrimSpace(c.GRPC.Cert),
		KeyFile:   strings.TrimSpace(c.GRPC.Key),
		CAFile:    strings.TrimSpace(c.GRPC.CA),
		AutoGen:   true, // 自动生成开发证书
		ConfigDir: cfgFile,
	})
	if err != nil {
		fmt.Printf("Failed to setup TLS: %v\n", err)
		return
	}
	if tlsOpt != nil {
		opts = append(opts, tlsOpt)
	}

	rpcConf := zrpc.RpcServerConf{
		ListenOn: addr,
	}
	rpcConf.Name = "croupier-server-grpc"

	grpcServer := zrpc.MustNewServer(rpcConf, func(s *grpc.Server) {
		controlServer := control.NewServer(ctx.RegistryStore)
		serverv1.RegisterControlServiceServer(s, controlServer)
	})
	grpcServer.AddOptions(opts...)

	fmt.Printf("Starting gRPC ControlService on %s...\n", addr)
	grpcServer.Start()
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

	if strings.TrimSpace(c.Components.DataDir) == "" {
		c.Components.DataDir = "data"
	}

	if strings.EqualFold(strings.TrimSpace(c.Storage.Driver), "file") {
		if strings.TrimSpace(c.Storage.BaseDir) == "" {
			c.Storage.BaseDir = filepath.Join("data", "uploads")
		}
	}
}
