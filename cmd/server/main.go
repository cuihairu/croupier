package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	httpserver "github.com/cuihairu/croupier/internal/app/server/http"
	controlserver "github.com/cuihairu/croupier/internal/platform/control"
	controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	// register json codec
	_ "github.com/cuihairu/croupier/internal/transport/jsoncodec"

	common "github.com/cuihairu/croupier/internal/cli/common"
	"github.com/cuihairu/croupier/internal/connpool"
	"github.com/cuihairu/croupier/internal/devcert"
	tlsutil "github.com/cuihairu/croupier/internal/platform/tlsutil"
	"strings"
)

// noopInvoker implements httpserver.FunctionInvoker with no-op behavior so that
// the HTTP server can run non-function endpoints without wiring FunctionService.
type noopInvoker struct{}

func (noopInvoker) Invoke(ctx context.Context, in *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	return &functionv1.InvokeResponse{Payload: nil}, nil
}
func (noopInvoker) StartJob(ctx context.Context, in *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	return &functionv1.StartJobResponse{JobId: ""}, nil
}
func (noopInvoker) StreamJob(ctx context.Context, in *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error) {
	return nil, fmt.Errorf("stream not available")
}
func (noopInvoker) CancelJob(ctx context.Context, in *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
	return &functionv1.StartJobResponse{JobId: in.GetJobId()}, nil
}

// edgeForwarder implements FunctionInvoker by forwarding to an Edge instance over gRPC.
type edgeForwarder struct {
	cc    *grpc.ClientConn
	cli   functionv1.FunctionServiceClient
	mu    sync.Mutex
	stats httpserver.AgentStats
	// sliding QPS window (60s)
	buckets [60]int64
	lastSec int64
}

func newEdgeForwarder(addr string, creds credentials.TransportCredentials) (*edgeForwarder, error) {
	cc, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{}),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")),
	)
	if err != nil {
		return nil, err
	}
	return &edgeForwarder{cc: cc, cli: functionv1.NewFunctionServiceClient(cc)}, nil
}

func (e *edgeForwarder) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	var out *functionv1.InvokeResponse
	err := e.withStats(func(ctx context.Context) error {
		var err error
		out, err = e.cli.Invoke(ctx, req)
		return err
	})
	return out, err
}
func (e *edgeForwarder) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	var out *functionv1.StartJobResponse
	err := e.withStats(func(ctx context.Context) error {
		var err error
		out, err = e.cli.StartJob(ctx, req)
		return err
	})
	return out, err
}
func (e *edgeForwarder) StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error) {
	atomic.AddInt64(&e.stats.ActiveConns, 1)
	start := time.Now()
	stream, err := e.cli.StreamJob(ctx, req)
	dur := time.Since(start)
	atomic.AddInt64(&e.stats.ActiveConns, -1)
	e.finishSample(dur, err)
	return stream, err
}
func (e *edgeForwarder) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
	var out *functionv1.StartJobResponse
	err := e.withStats(func(ctx context.Context) error {
		var err error
		out, err = e.cli.CancelJob(ctx, req)
		return err
	})
	return out, err
}

// withStats wraps a unary RPC with timeout + one retry on transient error, and updates stats.
func (e *edgeForwarder) withStats(call func(ctx context.Context) error) error {
	atomic.AddInt64(&e.stats.ActiveConns, 1)
	defer atomic.AddInt64(&e.stats.ActiveConns, -1)
	// per-call timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	start := time.Now()
	err := call(ctx)
	if shouldRetry(err) {
		// brief backoff
		time.Sleep(200 * time.Millisecond)
		start = time.Now()
		// renew timeout for retry
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		err = call(ctx2)
		cancel2()
	}
	e.finishSample(time.Since(start), err)
	return err
}

func (e *edgeForwarder) finishSample(dur time.Duration, err error) {
	// total
	atomic.AddInt64(&e.stats.TotalRequests, 1)
	if err != nil {
		atomic.AddInt64(&e.stats.FailedRequests, 1)
	}
	// avg response time (EMA)
	e.mu.Lock()
	if e.stats.AvgResponseTime == 0 {
		e.stats.AvgResponseTime = dur
	} else {
		e.stats.AvgResponseTime = (e.stats.AvgResponseTime*9 + dur) / 10
	}
	// QPS window
	now := time.Now().Unix()
	sec := now % 60
	if e.lastSec == 0 {
		e.lastSec = now
	}
	if now != e.lastSec {
		if now-e.lastSec >= 60 {
			for i := 0; i < 60; i++ {
				e.buckets[i] = 0
			}
		} else {
			for t := e.lastSec + 1; t <= now; t++ {
				e.buckets[t%60] = 0
			}
		}
		e.lastSec = now
	}
	e.buckets[sec]++
	var sum int64
	for i := 0; i < 60; i++ {
		sum += e.buckets[i]
	}
	e.stats.QPS1m = float64(sum) / 60.0
	e.stats.LastSeen = time.Now()
	e.mu.Unlock()
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unavailable") || strings.Contains(s, "deadline") || strings.Contains(s, "timeout")
}

// Stats provider for HTTP layer
func (e *edgeForwarder) GetStats() map[string]*httpserver.AgentStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := e.stats
	return map[string]*httpserver.AgentStats{"edge": &cp}
}
func (e *edgeForwarder) GetPoolStats() *connpool.PoolStats { return nil }

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
			// FunctionService invoker: forward to Edge when provided, else use a no-op.
			var invoker httpserver.FunctionInvoker = noopInvoker{}
			if edgeAddr != "" {
				// Build mTLS creds for dialing edge (reuse server certs/CA).
				// Use SNI from host part when possible.
				sni := ""
				host := edgeAddr
				if i := strings.Index(host, "://"); i >= 0 {
					host = host[i+3:]
				}
				if i := strings.LastIndex(host, ":"); i >= 0 {
					host = host[:i]
				}
				if host != "" && host != ":" {
					sni = host
				}
				creds, err := tlsutil.ClientTLS(cert, key, ca, sni)
				if err != nil {
					slog.Error("edge dial tls", "error", err)
					os.Exit(1)
				}
				fwd, err := newEdgeForwarder(edgeAddr, creds)
				if err != nil {
					slog.Error("edge dial", "error", err)
					os.Exit(1)
				}
				invoker = fwd
			}
			var locator interface{ GetJobAddr(string) (string, bool) } = nil

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
				_ = usersPath // legacy users.json ignored; DB-backed users in http server
				// Export RBAC/JWT config to env for InitServerAppAuto providers
				if rbacPath != "" {
					_ = os.Setenv("RBAC_CONFIG", rbacPath)
				}
				if jwtSecret != "" {
					_ = os.Setenv("JWT_SECRET", jwtSecret)
				}
				var statsProv interface {
					GetStats() map[string]*httpserver.AgentStats
					GetPoolStats() *connpool.PoolStats
				}
				if ef, ok := invoker.(*edgeForwarder); ok {
					statsProv = ef
				}
				httpSrv, err = httpserver.InitServerAppAuto("gen/croupier", invoker, ctrl.Store(), locator, statsProv)
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
