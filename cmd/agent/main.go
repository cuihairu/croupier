package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	appagent "github.com/cuihairu/croupier/internal/app/agent"
	common "github.com/cuihairu/croupier/internal/cli/common"
	"github.com/cuihairu/croupier/internal/devcert"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	_ "github.com/cuihairu/croupier/internal/transport/jsoncodec"
)

// responseRecorder wraps http.ResponseWriter to capture status code
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func main() {
	var cfgFile string
	var root = &cobra.Command{Use: "croupier-agent", Short: "Croupier Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			// logger
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
			common.MergeLogSection(viper.GetViper())
			if viper.IsSet("log.output") {
				_ = os.Setenv("CROUPIER_LOG_OUTPUT", viper.GetString("log.output"))
			}
			common.SetupLoggerWithFile(
				viper.GetString("log.level"), viper.GetString("log.format"), viper.GetString("log.file"),
				viper.GetInt("log.max_size"), viper.GetInt("log.max_backups"), viper.GetInt("log.max_age"), viper.GetBool("log.compress"),
			)

			localAddr := viper.GetString("local_addr")
			httpAddr := viper.GetString("http_addr")
			insecureLocal := viper.GetBool("insecure_local")
			serverAddr := viper.GetString("server_addr")
			agentID := viper.GetString("agent_id")
			certFile := viper.GetString("cert")
			keyFile := viper.GetString("key")
			caFile := viper.GetString("ca")
			if agentID == "" {
				agentID = "agent-" + time.Now().Format("20060102150405")
			}

			lis, err := net.Listen("tcp", localAddr)
			if err != nil {
				slog.Error("listen local", "error", err)
				os.Exit(1)
			}

			var srv *grpc.Server
			if insecureLocal {
				srv = grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{}))
			} else {
				creds, err := ensureLocalTLS(localAddr, certFile, keyFile, caFile)
				if err != nil {
					slog.Error("setup local TLS", "error", err)
					os.Exit(1)
				}
				srv = grpc.NewServer(
					grpc.Creds(creds),
					grpc.KeepaliveParams(keepalive.ServerParameters{}),
				)
			}

			// Register stub services via app/agent
			app := appagent.New(serverAddr, agentID)
			app.RegisterGRPC(srv)

			// Start upstream sync
			go func() {
				if err := app.Run(context.Background()); err != nil {
					slog.Error("upstream agent failed", "error", err)
				}
			}()

			var httpSrv *http.Server
			go func() {
				mux := http.NewServeMux()

				// Health check endpoint
				mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("ok"))
				})

				// Metrics endpoint
				mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"ok":true}`))
				})

				// Logging middleware
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Access-Control-Allow-Origin", "*")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

					if r.Method == http.MethodOptions {
						w.WriteHeader(http.StatusNoContent)
						return
					}

					start := time.Now()
					rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
					mux.ServeHTTP(rec, r)
					dur := time.Since(start)

					lvl := slog.LevelInfo
					if rec.statusCode >= 500 {
						lvl = slog.LevelError
					} else if rec.statusCode >= 400 {
						lvl = slog.LevelWarn
					}
					slog.Log(context.Background(), lvl, "agent_http", "method", r.Method, "path", r.URL.Path, "status", rec.statusCode, "dur_ms", dur.Milliseconds())
				})

				slog.Info("agent http listening", "addr", httpAddr)
				httpSrv = &http.Server{Addr: httpAddr, Handler: handler}
				_ = httpSrv.ListenAndServe()
			}()

			slog.Info("croupier-agent listening", "local", localAddr)
			go func() {
				c := make(chan os.Signal, 1)
				signal.Notify(c, os.Interrupt, syscall.SIGTERM)
				<-c
				slog.Info("agent shutting down")
				if httpSrv != nil {
					_ = httpSrv.Shutdown(nil)
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
	root.Flags().StringVar(&cfgFile, "config", "", "config file")
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func ensureLocalTLS(localAddr, certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	if isMissingTLSFile(certFile) || isMissingTLSFile(keyFile) {
		out := filepath.Join("configs", "dev")
		caCrt, caKey, err := devcert.EnsureDevCA(out)
		if err != nil {
			return nil, err
		}
		hosts := localTLSHosts(localAddr)
		crt, key, err := devcert.EnsureServerCert(out, caCrt, caKey, hosts)
		if err != nil {
			return nil, err
		}
		certFile, keyFile = crt, key
		if isMissingTLSFile(caFile) {
			caFile = caCrt
		}
		slog.Info("generated local TLS certificates", "cert", certFile, "ca", caFile)
	}

	return tlsutil.ServerTLS(certFile, keyFile, caFile, false)
}

func localTLSHosts(addr string) []string {
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		if host != "" && host != "0.0.0.0" && host != "::" && host != "[::]" {
			hosts = append(hosts, host)
		}
	} else if addr != "" {
		hosts = append(hosts, addr)
	}
	return hosts
}

func isMissingTLSFile(path string) bool {
	if path == "" {
		return true
	}
	if _, err := os.Stat(path); err != nil {
		return true
	}
	return false
}
