package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	controlserver "github.com/cuihairu/croupier/internal/server/control"
	functionserver "github.com/cuihairu/croupier/internal/server/function"
	httpserver "github.com/cuihairu/croupier/internal/server/http"
	controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	// register json codec
	auditchain "github.com/cuihairu/croupier/internal/audit/chain"
	rbac "github.com/cuihairu/croupier/internal/auth/rbac"
	_ "github.com/cuihairu/croupier/internal/transport/jsoncodec"

	jwt "github.com/cuihairu/croupier/internal/auth/token"
	common "github.com/cuihairu/croupier/internal/cli/common"
	"github.com/cuihairu/croupier/internal/connpool"
	"github.com/cuihairu/croupier/internal/devcert"
	"github.com/cuihairu/croupier/internal/loadbalancer"
	tlsutil "github.com/cuihairu/croupier/internal/tlsutil"
	"strings"
)

// loadServerTLS builds a tls.Config for mTLS if caFile is provided.
// Deprecated: inlined TLS helpers replaced by tlsutil

func main() {
	var cfgFile string
	root := &cobra.Command{
		Use:   "croupier-server",
		Short: "Croupier Server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// initialize default logger to stdout to avoid red stderr in early logs
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
			// Extract `server` section if present (YAML friendly)
			v := viper.GetViper()
			if sub := v.Sub("server"); sub != nil {
				v = sub
			}
			// set up logger (stdout by default); honor log.* if present
			common.MergeLogSection(v)
			if v.IsSet("log.output") {
				_ = os.Setenv("CROUPIER_LOG_OUTPUT", v.GetString("log.output"))
			}
			common.SetupLoggerWithFile(
				v.GetString("log.level"),
				v.GetString("log.format"),
				v.GetString("log.file"),
				v.GetInt("log.max_size"),
				v.GetInt("log.max_backups"),
				v.GetInt("log.max_age"),
				v.GetBool("log.compress"),
			)
			// DB config (prefer YAML in server.*)
			if vv := v.GetString("db.driver"); vv != "" {
				_ = os.Setenv("DB_DRIVER", vv)
			}
			if vv := v.GetString("db.dsn"); vv != "" {
				_ = os.Setenv("DATABASE_URL", vv)
			}
			// Storage config (to env bridge for http server)
			if vv := v.GetString("storage.driver"); vv != "" {
				_ = os.Setenv("STORAGE_DRIVER", vv)
			}
			if vv := v.GetString("storage.bucket"); vv != "" {
				_ = os.Setenv("STORAGE_BUCKET", vv)
			}
			if vv := v.GetString("storage.region"); vv != "" {
				_ = os.Setenv("STORAGE_REGION", vv)
			}
			if vv := v.GetString("storage.endpoint"); vv != "" {
				_ = os.Setenv("STORAGE_ENDPOINT", vv)
			}
			if vv := v.GetString("storage.access_key"); vv != "" {
				_ = os.Setenv("STORAGE_ACCESS_KEY", vv)
				_ = os.Setenv("AWS_ACCESS_KEY_ID", vv)
			}
			if vv := v.GetString("storage.secret_key"); vv != "" {
				_ = os.Setenv("STORAGE_SECRET_KEY", vv)
				_ = os.Setenv("AWS_SECRET_ACCESS_KEY", vv)
			}
			if v.IsSet("storage.force_path_style") {
				_ = os.Setenv("STORAGE_FORCE_PATH_STYLE", fmt.Sprintf("%v", v.GetBool("storage.force_path_style")))
			}
			if vv := v.GetString("storage.base_dir"); vv != "" {
				_ = os.Setenv("STORAGE_BASE_DIR", vv)
			}
			if vv := v.GetString("storage.signed_url_ttl"); vv != "" {
				_ = os.Setenv("STORAGE_SIGNED_URL_TTL", vv)
			}

			addr := v.GetString("addr")
			httpAddr := v.GetString("http_addr")
			edgeAddr := v.GetString("edge_addr")
			rbacPath := v.GetString("rbac_config")
			cert := v.GetString("cert")
			key := v.GetString("key")
			ca := v.GetString("ca")
			gamesPath := v.GetString("games_config")
			usersPath := v.GetString("users_config")
			jwtSecret := v.GetString("jwt_secret")

			// Auto-generate dev certs when not provided (DEV ONLY)
			if cert == "" || key == "" || ca == "" {
				out := "configs/dev"
				caCrt, caKey, err := devcert.EnsureDevCA(out)
				if err != nil {
					slog.Error("generate dev CA", "error", err)
					os.Exit(1)
				}
				// include common localhost hosts for dev
				srvCrt, srvKey, err := devcert.EnsureServerCert(out, caCrt, caKey, []string{"localhost", "127.0.0.1"})
				if err != nil {
					slog.Error("generate dev server cert", "error", err)
					os.Exit(1)
				}
				// set to generated paths
				cert, key, ca = srvCrt, srvKey, caCrt
				slog.Info("devcert generated", "dir", out)
			}

			creds, err := tlsutil.ServerTLS(cert, key, ca, true)
			if err != nil {
				slog.Error("load TLS", "error", err)
				os.Exit(1)
			}

			lis, err := net.Listen("tcp", addr)
			if err != nil {
				slog.Error("listen", "error", err)
				os.Exit(1)
			}

			s := grpc.NewServer(
				grpc.Creds(creds),
				grpc.KeepaliveParams(keepalive.ServerParameters{}),
			)

			// Register services
			// DB-backed games; ignore legacy gamesPath
			_ = gamesPath
			ctrl := controlserver.NewServer(nil)
			controlv1.RegisterControlServiceServer(s, ctrl)
			var invoker httpserver.FunctionInvoker
			var locator interface{ GetJobAddr(string) (string, bool) }
			if edgeAddr != "" {
				// Forward all FunctionService calls to Edge
				fwd := functionserver.NewForwarder(edgeAddr)
				functionv1.RegisterFunctionServiceServer(s, fwd)
				invoker = functionserver.NewForwarderInvoker(fwd)
				locator = nil // edge-forward mode: job_result API not available in Server
			} else {
				// Use default function server config when running in-server
				fnsrv := functionserver.NewServer(ctrl.Store(), nil)
				functionv1.RegisterFunctionServiceServer(s, fnsrv)
				invoker = functionserver.NewClientAdapter(fnsrv)
				locator = fnsrv
			}

			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				slog.Info("croupier-server listening", "grpc", addr)
				if err := s.Serve(lis); err != nil {
					slog.Error("serve grpc", "error", err)
					os.Exit(1)
				}
			}()
			var httpSrv *httpserver.Server
			go func() {
				defer wg.Done()
				aw, err := auditchain.NewWriter("logs/audit.log")
				if err != nil {
					slog.Error("audit", "error", err)
					os.Exit(1)
				}
				defer aw.Close()
				var pol rbac.PolicyInterface
				if p, err := rbac.LoadCasbinPolicy(rbacPath); err == nil {
					pol = p
				} else {
					log.Printf("[RBAC] Failed to load Casbin policy, creating fallback policy: %v", err)
					fallback := rbac.NewPolicy()
					fallback.Grant("user:dev", "*")
					fallback.Grant("user:dev", "job:cancel")
					fallback.Grant("role:admin", "*")
					fallback.Grant("user:admin", "*")
					pol = fallback
				}
				_ = usersPath // legacy users.json ignored; DB-backed users in http server
				jm := jwt.NewManager(jwtSecret)
				var statsProv interface {
					GetStats() map[string]*loadbalancer.AgentStats
					GetPoolStats() *connpool.PoolStats
				}
				if sp, ok := invoker.(interface {
					GetStats() map[string]*loadbalancer.AgentStats
					GetPoolStats() *connpool.PoolStats
				}); ok {
					statsProv = sp
				}
				httpSrv, err = httpserver.NewServer("gen/croupier", invoker, aw, pol, ctrl.Store(), jm, locator, statsProv)
				if err != nil {
					slog.Error("http server", "error", err)
					os.Exit(1)
				}
				if err := httpSrv.ListenAndServe(httpAddr); err != nil {
					slog.Error("serve http", "error", err)
					os.Exit(1)
				}
			}()
			// graceful shutdown on SIGINT/SIGTERM
			go func() {
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
				sig := <-sigCh
				slog.Info("shutdown signal", "signal", sig.String())
				// stop HTTP first
				if httpSrv != nil {
					_ = httpSrv.Shutdown(context.Background())
				}
				// then gRPC
				s.GracefulStop()
			}()
			wg.Wait()
			return nil
		},
	}

	// Flags and config
	root.Flags().StringVar(&cfgFile, "config", "", "config file (yaml), e.g. configs/server.yaml")
	root.Flags().String("addr", ":8443", "grpc listen address")
	root.Flags().String("http_addr", ":8080", "http api listen address")
	root.Flags().String("edge_addr", "", "optional edge address for forwarding function calls (DEV PoC)")
	root.Flags().String("rbac_config", "configs/rbac.json", "rbac policy json path")
	root.Flags().String("cert", "", "server cert file")
	root.Flags().String("key", "", "server key file")
	root.Flags().String("ca", "", "ca cert file for client cert verification")
	root.Flags().String("games_config", "configs/games.json", "allowed games config (json)")
	root.Flags().String("users_config", "configs/users.json", "users config json")
	root.Flags().String("jwt_secret", "dev-secret", "jwt hs256 secret")
	root.Flags().String("db.driver", "auto", "database driver: postgres|mysql|sqlite|auto")
	root.Flags().String("db.dsn", "", "database DSN/URL; for sqlite can be file:path.db or :memory:")
	// storage
	root.Flags().String("storage.driver", "", "object storage driver: s3|oss|file")
	root.Flags().String("storage.bucket", "", "object storage bucket")
	root.Flags().String("storage.region", "", "object storage region (s3)")
	root.Flags().String("storage.endpoint", "", "object storage endpoint (s3/cos/minio)")
	root.Flags().String("storage.access_key", "", "object storage access key")
	root.Flags().String("storage.secret_key", "", "object storage secret key")
	root.Flags().Bool("storage.force_path_style", false, "s3 path-style routing")
	root.Flags().String("storage.base_dir", "", "local storage base dir when driver=file (default data/uploads)")
	root.Flags().String("storage.signed_url_ttl", "15m", "signed URL TTL, e.g. 15m,1h")
	_ = viper.BindPFlags(root.Flags())

	if err := root.Execute(); err != nil {
		slog.Error("server exit", "error", err)
		os.Exit(1)
	}
}
