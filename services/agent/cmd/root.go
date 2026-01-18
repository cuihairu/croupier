package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/services/agent/internal/config"
	"github.com/cuihairu/croupier/services/agent/internal/handler"
	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/spf13/cobra"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
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

	runCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	core, localGRPCAddr, err := startGRPCCore(runCtx, &c)
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

	svcCtx := svc.NewServiceContext(c, core, localGRPCAddr)
	handler.RegisterHandlers(server, svcCtx)

	fmt.Printf("Starting Croupier Agent at %s:%d (mode: %s, debug: %v)...\n",
		c.RestConf.Host, c.RestConf.Port, c.RestConf.Mode, debug)

	go server.Start()

	slog.Info("agent http server started", "addr", fmt.Sprintf("%s:%d", c.RestConf.Host, c.RestConf.Port))
	slog.Info("agent grpc core started", "listen", localGRPCAddr)

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

func startGRPCCore(ctx context.Context, c *config.Config) (*agentcore.App, string, error) {
	if c == nil {
		return nil, "", fmt.Errorf("missing config")
	}
	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(c.GRPC.Host), c.GRPC.Port)
	if strings.TrimSpace(c.GRPC.Host) == "" || c.GRPC.Port == 0 {
		return nil, "", fmt.Errorf("grpc host/port not configured")
	}

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
		rpcAddr = addr
	}

	core := agentcore.New(strings.TrimSpace(c.Server.Addr), agentID)
	core.WithUpstreamMetadata(agentcore.UpstreamMetadata{
		GameID:            strings.TrimSpace(c.Agent.GameID),
		Env:               strings.TrimSpace(c.Agent.Env),
		Version:           Version,
		RPCAddr:           rpcAddr,
		DialTimeout:       time.Duration(c.Upstream.Timeout) * time.Millisecond,
		RequestTimeout:    time.Duration(c.Upstream.Timeout) * time.Millisecond,
		HeartbeatInterval: time.Duration(c.Upstream.HeartbeatInterval) * time.Second,
	})

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

	var grpcOpts []grpc.ServerOption
	if c.TLS.Enabled {
		certFile := strings.TrimSpace(c.TLS.CertFile)
		keyFile := strings.TrimSpace(c.TLS.KeyFile)
		caFile := strings.TrimSpace(c.TLS.CAFile)
		if certFile == "" || keyFile == "" {
			return nil, "", fmt.Errorf("TLS enabled but missing cert/key (TLS.CertFile/TLS.KeyFile)")
		}
		creds, err := tlsutil.ServerTLS(certFile, keyFile, caFile, caFile != "")
		if err != nil {
			return nil, "", fmt.Errorf("failed to create gRPC TLS credentials: %w", err)
		}
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
	}

	rpcConf := zrpc.RpcServerConf{
		ListenOn: addr,
	}
	rpcConf.Name = "croupier-agent-grpc"
	grpcServer := zrpc.MustNewServer(rpcConf, func(s *grpc.Server) {
		core.RegisterGRPC(s)
	})
	grpcServer.AddOptions(grpcOpts...)

	go func() {
		if err := core.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("agent upstream sync failed", "error", err)
		}
	}()

	go func() {
		grpcServer.Start()
	}()

	return core, addr, nil
}
