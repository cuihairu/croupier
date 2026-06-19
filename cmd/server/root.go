// Package cmd implements the croupier-server CLI using Cobra
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cuihairu/croupier/internal/cli/common"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/handler"
	"github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/runtime"
	"github.com/cuihairu/croupier/internal/server"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/tasks"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

支持单公司多游戏多环境作用域、函数注册、负载均衡、审计链、审批工作流等功能。`,
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

// main is the entry point for the croupier-server CLI
func main() {
	Execute()
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

	// 初始化服务管理命令
	initServiceCommand()
}

func runServer() error {
	var c config.Config

	// 加载配置文件
	if cfgFile == "" {
		return fmt.Errorf("配置文件是必需的")
	}
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	expanded := os.ExpandEnv(string(data))
	if err := yaml.Unmarshal([]byte(expanded), &c); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 覆盖配置文件设置
	if port > 0 {
		c.Server.Port = port
	}
	if host != "" {
		c.Server.Host = host
	}

	// 根据模式调整配置
	switch mode {
	case "prod":
		c.Server.Mode = "prod"
	case "test":
		c.Server.Mode = "test"
	default:
		c.Server.Mode = "dev"
	}

	// 调试模式设置
	if debug {
		fmt.Println("Debug mode enabled")
		c.Server.Mode = "dev"
		if logLevel == "" {
			logLevel = "debug"
		}
	}

	// 日志级别设置
	if logLevel != "" {
		if c.Logging.Level == "" {
			c.Logging.Level = logLevel
		}
	}

	// 初始化日志系统
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "console"
	}
	common.SetupLoggerWithFile(
		c.Logging.Level,
		c.Logging.Format,
		c.Logging.Output,
		c.Logging.MaxSize,
		c.Logging.MaxBackups,
		c.Logging.MaxAge,
		c.Logging.Compress,
	)

	// 引导数据目录
	if bootstrapDataDir != "" {
		c.BootstrapData.BaseDir = bootstrapDataDir
	}

	applyRuntimeDefaults(&c)

	// 创建服务上下文
	svcCtx := svc.NewServiceContext(c)

	// 创建 AgentSessionStore 用于管理 Agent TCP session
	sessionStore := server.NewAgentSessionStore()

	// 初始化 Agent Ops 客户端并注入 session resolver
	ops.InitAgentOpsClient()
	opsClient := ops.GetAgentOpsClient()
	opsClient.SetSessionResolver(server.NewSessionResolverAdapter(sessionStore))

	// 检查数据库连接
	dbHealth := svc.NewDBHealth(svcCtx)
	if err := dbHealth.Ping(); err != nil {
		fmt.Printf("警告: 数据库连接失败: %v\n", err)
		fmt.Println("某些功能可能无法正常工作，请检查数据库配置")
	}

	// 将 session resolver 注入到 Dispatcher
	if svcCtx.Dispatcher != nil {
		svcCtx.Dispatcher.SetSessionResolver(server.NewSessionResolverAdapter(sessionStore))
		// 将 task event query 注入到 Dispatcher（用于 StreamTask 查询）
		taskQuery := dispatch.NewTaskEventQueryAdapter(
			model.NewTaskEventModel(svcCtx.DB),
			model.NewTaskRunModel(svcCtx.DB),
		)
		svcCtx.Dispatcher.SetTaskEventQuery(taskQuery)
	}

	// 启动控制服务器（TCP）
	go startControlServer(&c, svcCtx, sessionStore)

	// 启动 Registry 清理任务（定期删除过期的 AgentSession）
	go startRegistryCleanup(svcCtx)

	// 设置 Gin 模式
	switch c.Server.Mode {
	case "prod":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	// 创建 REST 服务器
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors.Default())

	// 禁用自动重定向 trailing slash（避免 /api/v1/profile → /api/v1/profile/ 的 301 重定向）
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false

	// 添加认证中间件
	authMiddleware := svc.NewAuthMiddleware(svcCtx)
	r.Use(authMiddleware)

	// 添加 Game DB 路由中间件（database-per-game 架构下根据 X-Game-ID/X-Env
	// 解析对应的游戏数据库并注入到 request context）
	r.Use(svc.GameDBMiddleware(svcCtx))

	// 注册路由
	handler.RegisterHandlers(r, svcCtx)

	addr := fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
	fmt.Printf("Starting Croupier Server at %s (mode: %s, debug: %v)...\n",
		addr, c.Server.Mode, debug)

	// 配置 HTTP 服务器（支持 SSE 长连接）
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	// 启动 HTTP 服务（在 goroutine 中）
	go func() {
		fmt.Printf("Starting Croupier Server at %s (mode: %s, debug: %v)...\n",
			addr, c.Server.Mode, debug)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// 等待中断信号（SIGINT / SIGTERM）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("\nShutting down server...")

	// 给在途请求 15 秒优雅退出
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Server forced to shutdown: %v\n", err)
	}

	// 关闭 database-per-game Router 缓存的所有游戏数据库连接
	if svcCtx.Router != nil {
		if err := svcCtx.Router.Close(); err != nil {
			slog.Default().Error("Failed to close game database router", "error", err)
		}
	}

	fmt.Println("Server exited")
	return nil
}

// startControlServer 启动控制服务器（TCP）
func startControlServer(c *config.Config, svcCtx *svc.ServiceContext, sessionStore *server.AgentSessionStore) {
	// 解析监听地址
	addr := c.Control.Addr
	if addr == "" {
		addr = ":19090" // 默认 ControlService 端口
	}
	if addr[0] == ':' {
		addr = "0.0.0.0" + addr
	}

	// 创建 ControlService
	controlService := server.NewControlService(svcCtx.RegistryStore, svcCtx.AgentSessionModel)
	controlService.SetTaskStore(tasks.NewStore(
		model.NewTaskRunModel(svcCtx.DB),
		model.NewTaskEventModel(svcCtx.DB),
	))
	controlService.StartBackgroundTasks()

	// 创建 TCPListener (管理 Agent session)
	// 如果配置了 TLS 证书，启用 TLS；否则使用 insecure 模式
	hasTLS := c.Control.Cert != "" && c.Control.Key != ""
	listenerConfig := &server.TCPListenerConfig{
		Address:     addr,
		Insecure:    !hasTLS,
		CertFile:    c.Control.Cert,
		KeyFile:     c.Control.Key,
		CAFile:      c.Control.CA,
		RecvTimeout: time.Second,
		SendTimeout: 10 * time.Second,
	}
	tcpListener, err := server.NewTCPListener(listenerConfig, sessionStore, svcCtx.RegistryStore, slog.Default())
	if err != nil {
		fmt.Printf("Failed to create TCP listener: %v\n", err)
		return
	}

	// 设置 control handler
	tcpListener.SetHandler(controlService)

	// 启动 session 清理任务（定期删除过时的 session）
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			pruned := sessionStore.PruneStale(5 * time.Minute)
			if pruned > 0 {
				fmt.Printf("[control] Pruned %d stale sessions\n", pruned)
			}
		}
	}()

	fmt.Printf("Starting TCP ControlService on %s (SDK/Agent registration with session management)...\n", addr)
	if err := tcpListener.Serve(context.Background()); err != nil && err != context.Canceled {
		fmt.Printf("TCP Control server stopped: %v\n", err)
	}
}

// startRegistryCleanup 启动后台清理任务，定期删除过期的 AgentSession
func startRegistryCleanup(svcCtx *svc.ServiceContext) {
	store := svcCtx.RegistryStore
	if store == nil {
		fmt.Println("RegistryStore is nil, skipping cleanup routine")
		return
	}

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动清理任务，默认每分钟执行一次
	store.StartCleanupRoutine(ctx, 1*time.Minute)
}

func applyRuntimeDefaults(c *config.Config) {
	if c == nil {
		return
	}

	// Allow long-lived connections (SSE, streaming) by keeping HTTP timeout generous.
	if c.Server.Timeout == 0 {
		// default timeout 3s; bump to 10 minutes for streaming endpoints.
		c.Server.Timeout = 600000
	}

	// ✅ Auto-adjust timeout based on SSE configuration to prevent premature disconnection
	validateAndAdjustTimeout(c)

	if strings.EqualFold(strings.TrimSpace(c.Storage.Driver), "file") {
		if strings.TrimSpace(c.Storage.BaseDir) == "" {
			c.Storage.BaseDir = filepath.Join("data", "uploads")
		}
	}
}

// validateAndAdjustTimeout 确保 Timeout > SSE intervals
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
	currentTimeoutSec := c.Server.Timeout / 1000 // 毫秒转秒

	// 如果当前超时小于安全值，自动调整并警告
	if currentTimeoutSec < int64(minSafeTimeout) {
		fmt.Printf("⚠️  警告: Timeout (%d秒) 小于 SSE KeepAliveInterval (%d秒) 的 3 倍\n",
			currentTimeoutSec, keepAliveInterval)
		fmt.Printf("   自动调整 Timeout 为 %d 秒以防止 SSE 连接过早断开\n", minSafeTimeout)

		c.Server.Timeout = int64(minSafeTimeout) * 1000 // 秒转毫秒
	} else {
		// 验证通过，显示配置信息
		fmt.Printf("✅ SSE 配置验证通过:\n")
		fmt.Printf("   - HTTP WriteTimeout: 10 分钟\n")
		fmt.Printf("   - SSE UpdateInterval: %d 秒\n", updateInterval)
		fmt.Printf("   - SSE KeepAliveInterval: %d 秒\n", keepAliveInterval)
		fmt.Printf("   - 安全系数: %.1fx (超时 / KeepAlive)\n", float64(currentTimeoutSec)/float64(keepAliveInterval))
	}
}
