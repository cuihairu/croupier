package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	edgeapp "github.com/cuihairu/croupier/internal/app/edge"
	common "github.com/cuihairu/croupier/internal/cli/common"
	registry "github.com/cuihairu/croupier/internal/platform/registry"
	tlsutil "github.com/cuihairu/croupier/internal/platform/tlsutil"
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
	// initialize logger (stdout, console) before prints; can be overridden by env LOG_OUTPUT or config in other modes
	common.SetupLoggerWithFile("info", "console", "", 0, 0, 0, false)
	// Ports can be the same; FunctionService and ControlService share one listener.
	addr := flag.String("addr", ":9443", "edge grpc listen")
	cert := flag.String("cert", "", "TLS cert file")
	key := flag.String("key", "", "TLS key file")
	ca := flag.String("ca", "", "CA cert file (client verify)")
	_ = flag.String("games_config", "", "(deprecated) allowed games config")
	httpAddr := flag.String("http_addr", ":9080", "edge http listen for health/metrics")
	flag.Parse()

	if *cert == "" || *key == "" {
		slog.Error("TLS cert/key required")
		os.Exit(1)
	}
	creds, err := tlsutil.ServerTLS(*cert, *key, *ca, true)
	if err != nil {
		slog.Error("load TLS", "error", err)
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		slog.Error("listen", "error", err)
		os.Exit(1)
	}
	s := grpc.NewServer(grpc.Creds(creds), grpc.KeepaliveParams(keepalive.ServerParameters{}))

	reg := registry.NewStore()
	app := edgeapp.New(reg)
	app.RegisterGRPC(s)

	// HTTP health/metrics
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
			data, _ := json.Marshal(app.MetricsMap())
			w.Write(data)
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
			slog.Log(context.Background(), lvl, "edge_http", "method", r.Method, "path", r.URL.Path, "status", rec.statusCode, "dur_ms", dur.Milliseconds())
		})

		slog.Info("edge http listening", "addr", *httpAddr)
		httpSrv = &http.Server{Addr: *httpAddr, Handler: handler}
		_ = httpSrv.ListenAndServe()
	}()
	slog.Info("edge listening", "addr", *addr)
	// graceful shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		slog.Info("edge shutting down")
		if httpSrv != nil {
			_ = httpSrv.Shutdown(nil)
		}
		s.GracefulStop()
	}()
	if err := s.Serve(lis); err != nil {
		slog.Error("serve", "error", err)
		os.Exit(1)
	}
}
