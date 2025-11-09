package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	// "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	controlclient "github.com/cuihairu/croupier/internal/agent/control"
	agentfunc "github.com/cuihairu/croupier/internal/agent/function"
	"github.com/cuihairu/croupier/internal/agent/jobs"
	locallib "github.com/cuihairu/croupier/internal/agent/local"
	localreg "github.com/cuihairu/croupier/internal/agent/registry"
	tunn "github.com/cuihairu/croupier/internal/agent/tunnel"
	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
	controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	// register json codec
	common "github.com/cuihairu/croupier/internal/cli/common"
	"github.com/cuihairu/croupier/internal/devcert"
	tlsutil "github.com/cuihairu/croupier/internal/tlsutil"
	_ "github.com/cuihairu/croupier/internal/transport/jsoncodec"
	gin "github.com/gin-gonic/gin"
)

// Deprecated: local TLS helper replaced by tlsutil.ClientTLS

func main() {
	var cfgFile string
	var root = &cobra.Command{
		Use:   "croupier-agent",
		Short: "Croupier Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			// default logger to stdout for early logs
			common.SetupLoggerWithFile("info", "console", "", 0, 0, 0, false)
			viper.SetEnvPrefix("CROUPIER")
			viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
			viper.AutomaticEnv()
			if cfgFile != "" {
				viper.SetConfigFile(cfgFile)
				if err := viper.ReadInConfig(); err != nil {
					slog.Warn("read config", "error", err)
				} else {
					slog.Info("config loaded", "file", viper.ConfigFileUsed())
				}
			}
			// logging: default to stdout; honor log.* config if provided
			common.MergeLogSection(viper.GetViper())
			if viper.IsSet("log.output") {
				_ = os.Setenv("CROUPIER_LOG_OUTPUT", viper.GetString("log.output"))
			}
			common.SetupLoggerWithFile(
				viper.GetString("log.level"),
				viper.GetString("log.format"),
				viper.GetString("log.file"),
				viper.GetInt("log.max_size"),
				viper.GetInt("log.max_backups"),
				viper.GetInt("log.max_age"),
				viper.GetBool("log.compress"),
			)

			localAddr := viper.GetString("local_addr")
			serverAddr := viper.GetString("server_addr")
			coreAddr := viper.GetString("core_addr")
			serverName := viper.GetString("server_name")
			cert := viper.GetString("cert")
			key := viper.GetString("key")
			ca := viper.GetString("ca")
			insecureLocal := viper.GetBool("insecure_local")
			agentID := viper.GetString("agent_id")
			agentVersion := viper.GetString("agent_version")
			gameID := viper.GetString("game_id")
			env := viper.GetString("env")
			httpAddr := viper.GetString("http_addr")

			// Prefer --server_addr if provided (alias), warn on deprecated --core_addr usage
			if serverAddr != "" {
				if coreAddr != "" && coreAddr != "127.0.0.1:8443" {
					slog.Warn("both server_addr and core_addr provided; using server_addr", "server_addr", serverAddr)
				}
				coreAddr = serverAddr
			} else if coreAddr != "" {
				slog.Warn("--core_addr is deprecated; use --server_addr")
			}

			// Auto-generate dev certs when not provided (DEV ONLY)
			if (cert == "" || key == "" || ca == "") && coreAddr != "" {
				out := "configs/dev"
				caCrt, caKey, err := devcert.EnsureDevCA(out)
				if err != nil {
					slog.Error("generate dev CA", "error", err)
					os.Exit(1)
				}
				agCrt, agKey, err := devcert.EnsureAgentCert(out, caCrt, caKey, agentID)
				if err != nil {
					slog.Error("generate dev agent cert", "error", err)
					os.Exit(1)
				}
				cert, key, ca = agCrt, agKey, caCrt
				slog.Info("devcert generated", "dir", out)
			}

			// Connect to Server with mTLS (required by default)
			var dialOpt grpc.DialOption
			if cert != "" && key != "" && ca != "" {
				// Default SNI from core_addr host if not provided
				sni := serverName
				if sni == "" {
					host := coreAddr
					if i := strings.Index(host, "://"); i >= 0 {
						host = host[i+3:]
					}
					if i := strings.LastIndex(host, ":"); i >= 0 {
						host = host[:i]
					}
					sni = host
				}
				creds, err := tlsutil.ClientTLS(cert, key, ca, sni)
				if err != nil {
					slog.Error("load TLS", "error", err)
					os.Exit(1)
				}
				dialOpt = grpc.WithTransportCredentials(creds)
			} else {
				slog.Error("TLS cert/key/ca required for agent outbound; provide --cert/--key/--ca")
				os.Exit(1)
			}

			coreConn, err := grpc.Dial(coreAddr, dialOpt, grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second}), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
			if err != nil {
				slog.Error("dial server", "error", err)
				os.Exit(1)
			}
			defer coreConn.Close()

			// Bootstrap register/heartbeat (placeholder function list; Local server will update on RegisterLocal)
			go func() {
				cc := controlclient.NewClient(coreConn)
				fns := []*controlv1.FunctionDescriptor{}
				ctx := context.Background()
				cc.RegisterAndHeartbeat(ctx, agentID, agentVersion, localAddr, gameID, env, fns)
			}()

			// Local gRPC for game servers to connect
			lis, err := net.Listen("tcp", localAddr)
			if err != nil {
				slog.Error("listen local", "error", err)
				os.Exit(1)
			}

			var srv *grpc.Server
			if insecureLocal {
				srv = grpc.NewServer()
			} else {
				slog.Error("secure local server not implemented; run with --insecure_local")
				os.Exit(1)
			}

			// Local registry (function id -> local game server endpoint/version)
			lstore := localreg.NewLocalStore()
			exec := jobs.NewExecutor()
			// Register local FunctionService endpoint (routes to local game servers & job executor)
			functionv1.RegisterFunctionServiceServer(srv, agentfunc.NewServer(lstore, exec))
			// Register LocalControl service for SDKs to register themselves
			localv1.RegisterLocalControlServiceServer(srv, locallib.NewServer(lstore, controlv1.NewControlServiceClient(coreConn), agentID, agentVersion, localAddr, gameID, env, exec))
			// Open tunnel to Edge/Server for Invoke proxy
			go func() {
				t := tunn.NewClient(coreAddr, agentID, gameID, env, localAddr)
				backoff := time.Second
				for {
					err := t.Start(context.Background())
					if err != nil {
						slog.Warn("tunnel disconnected", "error", err)
					}
					time.Sleep(backoff)
					if backoff < 30*time.Second {
						backoff *= 2
					}
					tunn.IncReconnect()
				}
			}()

			// HTTP health/metrics (Gin)
			var httpSrv *http.Server
			go func() {
				r := gin.New()
				r.Use(func(c *gin.Context) {
					w := c.Writer
					r0 := c.Request
					w.Header().Set("Access-Control-Allow-Origin", "*")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					if r0.Method == http.MethodOptions {
						c.Status(http.StatusNoContent)
						c.Abort()
						return
					}
					start := time.Now()
					c.Next()
					dur := time.Since(start)
					lvl := slog.LevelInfo
					st := c.Writer.Status()
					if st >= 500 {
						lvl = slog.LevelError
					} else if st >= 400 {
						lvl = slog.LevelWarn
					}
					slog.Log(c, lvl, "agent_http", "method", r0.Method, "path", r0.URL.Path, "status", st, "dur_ms", dur.Milliseconds())
				})
				r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
				r.GET("/metrics", func(c *gin.Context) {
					mp := lstore.List()
					total := 0
					fns := 0
					for _, arr := range mp {
						fns++
						total += len(arr)
					}
					c.JSON(http.StatusOK, gin.H{"functions": fns, "instances": total, "tunnel_reconnects": tunn.Reconnects()})
				})
				slog.Info("agent http listening", "addr", httpAddr)
				httpSrv = &http.Server{Addr: httpAddr, Handler: r}
				_ = httpSrv.ListenAndServe()
			}()
			slog.Info("croupier-agent listening", "local", localAddr, "server", coreAddr)
			// prune stale instances periodically
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for range ticker.C {
					removed := lstore.Prune(60 * time.Second)
					if removed > 0 {
						slog.Info("pruned stale local instances", "count", removed)
					}
				}
			}()
			// graceful shutdown
			go func() {
				c := make(chan os.Signal, 1)
				signal.Notify(c, os.Interrupt, syscall.SIGTERM)
				<-c
				slog.Info("agent shutting down")
				if httpSrv != nil {
					_ = httpSrv.Shutdown(context.Background())
				}
				srv.GracefulStop()
			}()
			if err := srv.Serve(lis); err != nil {
				slog.Error("serve local", "error", err)
				os.Exit(1)
			}
			return nil
		},
	}

	// Flags and config
	root.Flags().StringVar(&cfgFile, "config", "", "config file (yaml), e.g. configs/agent.yaml")
	root.Flags().String("local_addr", ":19090", "local gRPC listen for game servers")
	root.Flags().String("server_addr", "", "server grpc address (alias for --core_addr)")
	root.Flags().String("core_addr", "127.0.0.1:8443", "server grpc address (deprecated)")
	root.Flags().String("server_name", "", "tls server name (SNI)")
	root.Flags().String("cert", "", "client mTLS cert file")
	root.Flags().String("key", "", "client mTLS key file")
	root.Flags().String("ca", "", "ca cert file to verify server")
	root.Flags().Bool("insecure_local", true, "use insecure for local listener (development)")
	root.Flags().String("agent_id", "agent-1", "agent id")
	root.Flags().String("agent_version", "0.1.0", "agent version")
	root.Flags().String("game_id", "", "game id (required if server enforces whitelist)")
	root.Flags().String("env", "", "environment (optional) e.g. prod/stage/test")
	root.Flags().String("http_addr", ":19091", "agent http listen for health/metrics")
	_ = viper.BindPFlags(root.Flags())

	if err := root.Execute(); err != nil {
		slog.Error("agent exit", "error", err)
		os.Exit(1)
	}
}
