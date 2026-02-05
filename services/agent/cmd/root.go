package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
	"github.com/cuihairu/croupier/internal/cli/common"
	"github.com/cuihairu/croupier/internal/devcert"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/services/agent/internal/config"
	"github.com/cuihairu/croupier/services/agent/internal/handler"
	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/spf13/cobra"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/rest"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = ""

	cfgFile string
	mode    string
	port    int
	host    string
	debug   bool
)

var rootCmd = &cobra.Command{
	Use:   "croupier-agent",
	Short: "Croupier Agent 服务守护进程",
	Long: `Croupier Agent 用于与游戏实例交互、分发作业以及上报指标。

支持通过配置文件或命令行参数覆盖运行时行为。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgent()
	},
}

// Execute runs the CLI entrypoint.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = shortVersion()
	rootCmd.SetVersionTemplate("{{printf \"%s\\n\" .Version}}")

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "f", "etc/agent.yaml", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&mode, "mode", "m", "dev", "运行模式 (dev|prod|test)")
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 0, "覆盖配置文件中的端口 (0=使用配置)")
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "覆盖监听主机")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "启用调试模式")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "agent",
		Short: "Run agent (alias)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgent()
		},
	})
	rootCmd.AddCommand(versionCmd)
}

func runAgent() error {
	if cfgFile == "" {
		return fmt.Errorf("配置文件是必需的")
	}

	var c config.Config
	conf.MustLoad(cfgFile, &c, conf.UseEnv())

	if port > 0 {
		c.RestConf.Port = port
	}
	if host != "" {
		c.RestConf.Host = host
	}

	switch mode {
	case "prod":
		c.RestConf.Mode = "prod"
	case "test":
		c.RestConf.Mode = "test"
	default:
		c.RestConf.Mode = "dev"
	}

	if debug {
		fmt.Println("Debug mode enabled")
		c.RestConf.Mode = "dev"
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

	runCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	core, nngAddr, err := startAgentCore(runCtx, &c)
	if err != nil {
		return err
	}

	server := rest.MustNewServer(c.RestConf)
	defer func() {
		server.Stop()
		if core != nil {
			core.Stop()
		}
	}()

	svcCtx := svc.NewServiceContext(c, core, nngAddr)
	handler.RegisterHandlers(server, svcCtx)

	fmt.Printf("Starting Croupier Agent at %s:%d (mode: %s, debug: %v)...\n",
		c.RestConf.Host, c.RestConf.Port, c.RestConf.Mode, debug)

	go server.Start()

	slog.Info("agent http server started", "addr", fmt.Sprintf("%s:%d", c.RestConf.Host, c.RestConf.Port))
	slog.Info("agent nng core started", "listen", nngAddr)

	<-runCtx.Done()
	proc.Shutdown()
	return nil
}

func shortVersion() string {
	switch {
	case GitCommit == "unknown" && BuildTime == "":
		return Version
	case BuildTime == "":
		return fmt.Sprintf("%s (%s)", Version, GitCommit)
	default:
		return fmt.Sprintf("%s (%s, built %s)", Version, GitCommit, BuildTime)
	}
}

func startAgentCore(ctx context.Context, c *config.Config) (*agentcore.App, string, error) {
	if c == nil {
		return nil, "", fmt.Errorf("missing config")
	}

	// NNG local service address (for SDK→Agent communication)
	nngHost := strings.TrimSpace(c.ServerControl.Host)
	if nngHost == "" {
		nngHost = "0.0.0.0"
	}
	nngPort := c.ServerControl.Port
	if nngPort == 0 {
		nngPort = 19091 // Default NNG Agent port
	}
	nngAddr := fmt.Sprintf("%s:%d", nngHost, nngPort)

	agentID := strings.TrimSpace(c.Agent.ID)
	if agentID == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "agent"
		}
		agentID = fmt.Sprintf("%s-%d", host, time.Now().Unix())
		c.Agent.ID = agentID
	}

	rpcAddr := strings.TrimSpace(c.Agent.LocalAddr)
	if rpcAddr == "" {
		rpcAddr = nngAddr
	}

	// 收集系统标签
	labels := collectSystemLabels()
	// 合并配置中的 labels（系统信息优先级最高）
	for k, v := range c.Agent.Labels {
		labels[k] = v
	}

	core := agentcore.New(strings.TrimSpace(c.Server.Addr), agentID)
	core.SetNNGAddr(nngAddr)
	core.WithUpstreamMetadata(agentcore.UpstreamMetadata{
		GameID:            strings.TrimSpace(c.Agent.GameID),
		Env:               strings.TrimSpace(c.Agent.Env),
		Version:           Version,
		RPCAddr:           rpcAddr,
		Region:            strings.TrimSpace(c.Agent.Region),
		Zone:              strings.TrimSpace(c.Agent.Zone),
		Labels:            labels,
		DialTimeout:       time.Duration(c.Upstream.Timeout) * time.Millisecond,
		RequestTimeout:    time.Duration(c.Upstream.Timeout) * time.Millisecond,
		HeartbeatInterval: time.Duration(c.Upstream.HeartbeatInterval) * time.Second,
	})

	// Ensure CA cert exists (if needed for Server certificate verification)
	if !c.Server.Insecure && strings.TrimSpace(c.Server.CAFile) != "" {
		caFile := strings.TrimSpace(c.Server.CAFile)
		if _, err := os.Stat(caFile); os.IsNotExist(err) {
			certDir := filepath.Join(filepath.Dir(cfgFile), "certs")
			fmt.Printf("CA cert not found, auto-generating dev certs in %s...\n", certDir)
			if _, _, err := devcert.EnsureDevCA(certDir); err != nil {
				return nil, "", fmt.Errorf("failed to generate CA cert: %w", err)
			}
			fmt.Printf("  CA: %s\n", filepath.Join(certDir, "ca.crt"))
		}
	}

	if !c.Server.Insecure {
		core.WithUpstreamTLSConfig(&tlsutil.ClientTLSConfig{
			CertFile:           strings.TrimSpace(c.Server.TLSCertFile),
			KeyFile:            strings.TrimSpace(c.Server.TLSKeyFile),
			CAFile:             strings.TrimSpace(c.Server.CAFile),
			ServerName:         strings.TrimSpace(c.Server.ServerName),
			InsecureSkipVerify: c.Server.InsecureSkipVerify,
		})
	} else {
		core.WithUpstreamTLSConfig(nil)
	}

	if c.OutboundTLS.Enabled {
		core.WithOutboundTLSConfig(&tlsutil.ClientTLSConfig{
			CertFile:           strings.TrimSpace(c.OutboundTLS.CertFile),
			KeyFile:            strings.TrimSpace(c.OutboundTLS.KeyFile),
			CAFile:             strings.TrimSpace(c.OutboundTLS.CAFile),
			ServerName:         strings.TrimSpace(c.OutboundTLS.ServerName),
			InsecureSkipVerify: c.OutboundTLS.InsecureSkipVerify,
		})
	} else {
		core.WithOutboundTLSConfig(nil)
	}

	// Start the agent (which now starts NNG server internally)
	go func() {
		if err := core.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("agent run failed", "error", err)
		}
	}()

	return core, nngAddr, nil
}

// collectSystemLabels 收集系统信息作为标签
func collectSystemLabels() map[string]string {
	labels := make(map[string]string)

	// 操作系统
	labels["os"] = runtime.GOOS
	// CPU 架构
	labels["arch"] = runtime.GOARCH

	// 主机名
	if hostname, err := os.Hostname(); err == nil {
		labels["hostname"] = hostname
	}

	// 获取第一个非回环 IP 地址
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					labels["ip"] = ipnet.IP.String()
					break
				}
			}
		}
	}

	// CPU 核心数
	labels["cpu_count"] = fmt.Sprintf("%d", runtime.NumCPU())

	// Go 版本
	labels["go_version"] = runtime.Version()

	return labels
}
