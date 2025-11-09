package main

import (
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
	"google.golang.org/grpc/keepalive"

	appagent "github.com/cuihairu/croupier/internal/app/agent"
	common "github.com/cuihairu/croupier/internal/cli/common"
	_ "github.com/cuihairu/croupier/internal/transport/jsoncodec"
	gin "github.com/gin-gonic/gin"
)

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
				if err := viper.ReadInConfig(); err != nil { slog.Warn("read config", "error", err) } else { slog.Info("config loaded", "file", viper.ConfigFileUsed()) }
			}
			common.MergeLogSection(viper.GetViper())
			if viper.IsSet("log.output") { _ = os.Setenv("CROUPIER_LOG_OUTPUT", viper.GetString("log.output")) }
			common.SetupLoggerWithFile(
				viper.GetString("log.level"), viper.GetString("log.format"), viper.GetString("log.file"),
				viper.GetInt("log.max_size"), viper.GetInt("log.max_backups"), viper.GetInt("log.max_age"), viper.GetBool("log.compress"),
			)

			localAddr := viper.GetString("local_addr")
			httpAddr := viper.GetString("http_addr")
			insecureLocal := viper.GetBool("insecure_local")

			lis, err := net.Listen("tcp", localAddr)
			if err != nil { slog.Error("listen local", "error", err); os.Exit(1) }

			var srv *grpc.Server
			if insecureLocal { srv = grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{})) } else { slog.Error("secure local server not implemented; run with --insecure_local"); os.Exit(1) }

			// Register stub services via app/agent
			app := appagent.New()
			app.RegisterGRPC(srv)

			var httpSrv *http.Server
			go func() {
				r := gin.New()
				r.Use(func(c *gin.Context) {
					w := c.Writer; r0 := c.Request
					w.Header().Set("Access-Control-Allow-Origin", "*")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					if r0.Method == http.MethodOptions { c.Status(http.StatusNoContent); c.Abort(); return }
					start := time.Now(); c.Next(); dur := time.Since(start)
					lvl := slog.LevelInfo; st := c.Writer.Status(); if st >= 500 { lvl = slog.LevelError } else if st >= 400 { lvl = slog.LevelWarn }
					slog.Log(c, lvl, "agent_http", "method", r0.Method, "path", r0.URL.Path, "status", st, "dur_ms", dur.Milliseconds())
				})
				r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
				r.GET("/metrics", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
				slog.Info("agent http listening", "addr", httpAddr)
				httpSrv = &http.Server{Addr: httpAddr, Handler: r}
				_ = httpSrv.ListenAndServe()
			}()

			slog.Info("croupier-agent listening", "local", localAddr)
			go func() {
				c := make(chan os.Signal, 1)
				signal.Notify(c, os.Interrupt, syscall.SIGTERM)
				<-c
				slog.Info("agent shutting down")
				if httpSrv != nil { _ = httpSrv.Shutdown(nil) }
				srv.GracefulStop()
			}()
			if err := srv.Serve(lis); err != nil { slog.Error("serve local", "error", err); os.Exit(1) }
			return nil
		},
	}
	root.Flags().StringVar(&cfgFile, "config", "", "config file")
	if err := root.Execute(); err != nil { os.Exit(1) }
}
