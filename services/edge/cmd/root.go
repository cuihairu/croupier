package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/services/edge/internal/config"
	"github.com/cuihairu/croupier/services/edge/internal/handler"
	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/spf13/cobra"
	"github.com/zeromicro/go-zero/core/conf"
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
	Use:   "croupier-edge",
	Short: "Croupier Edge 服务入口",
	Long: `Croupier Edge 负责边缘节点接入、隧道管理以及玩家请求路由，
可通过命令行参数覆盖默认配置。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEdge()
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

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "f", "etc/edge.yaml", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&mode, "mode", "m", "dev", "运行模式 (dev|prod|test)")
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 0, "覆盖配置文件中的端口 (0=使用配置)")
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "覆盖监听主机")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "启用调试模式")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "edge",
		Short: "Run edge (alias)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEdge()
		},
	})
	rootCmd.AddCommand(versionCmd)
}

func runEdge() error {
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

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	grpcAddr := c.Server.InternalAddr
	if grpcAddr == "" {
		grpcAddr = ":18888"
	}

	var grpcOpts []grpc.ServerOption
	if c.TLS.Enabled {
		certFile := strings.TrimSpace(c.TLS.CertFile)
		keyFile := strings.TrimSpace(c.TLS.KeyFile)
		caFile := strings.TrimSpace(c.TLS.CAFile)
		if certFile == "" {
			certFile = strings.TrimSpace(c.Server.TLSCertFile)
		}
		if keyFile == "" {
			keyFile = strings.TrimSpace(c.Server.TLSKeyFile)
		}
		if certFile == "" || keyFile == "" {
			return fmt.Errorf("TLS enabled but missing cert/key (TLS.CertFile/TLS.KeyFile)")
		}

		creds, err := tlsutil.ServerTLS(certFile, keyFile, caFile, caFile != "")
		if err != nil {
			return fmt.Errorf("failed to create gRPC TLS credentials: %w", err)
		}
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
	}

	rpcConf := zrpc.RpcServerConf{
		ListenOn: grpcAddr,
	}
	rpcConf.Name = "croupier-edge-grpc"
	rpcServer := zrpc.MustNewServer(rpcConf, func(s *grpc.Server) {
		ctx.EdgeApp.RegisterGRPC(s)
	})
	rpcServer.AddOptions(grpcOpts...)
	go func() {
		fmt.Printf("Edge gRPC listening at %s\n", grpcAddr)
		rpcServer.Start()
	}()

	fmt.Printf("Starting Croupier Edge at %s:%d (mode: %s, debug: %v)...\n",
		c.RestConf.Host, c.RestConf.Port, c.RestConf.Mode, debug)

	server.Start()
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
