package main

import (
	"flag"
	"log/slog"
	"net"
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
	gin "github.com/gin-gonic/gin"
	"net/http"
)

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
			slog.Log(c, lvl, "edge_http", "method", r0.Method, "path", r0.URL.Path, "status", st, "dur_ms", dur.Milliseconds())
		})
		r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
		r.GET("/metrics", func(c *gin.Context) { c.JSON(http.StatusOK, app.MetricsMap()) })
		slog.Info("edge http listening", "addr", *httpAddr)
		httpSrv = &http.Server{Addr: *httpAddr, Handler: r}
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
