package edgecmd

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	common "github.com/cuihairu/croupier/internal/cli/common"
	functionserver "github.com/cuihairu/croupier/internal/edge/function"
	jobserver "github.com/cuihairu/croupier/internal/edge/job"
	tunnelsrv "github.com/cuihairu/croupier/internal/edge/tunnel"
	controlserver "github.com/cuihairu/croupier/internal/server/control"
	"github.com/cuihairu/croupier/internal/server/games"
	tlsutil "github.com/cuihairu/croupier/internal/tlsutil"
	controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
	jobv1 "github.com/cuihairu/croupier/pkg/pb/croupier/edge/job/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	tunnelv1 "github.com/cuihairu/croupier/pkg/pb/croupier/tunnel/v1"
	gin "github.com/gin-gonic/gin"
)

// New returns `croupier edge` command.
func New() *cobra.Command {
	var cfgFile string
	cmd := &cobra.Command{Use: "edge", Short: "Run Croupier Edge (forwarder)",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()
			v.SetEnvPrefix("CROUPIER_EDGE")
			v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
			v.AutomaticEnv()
			if cfgFile != "" {
				v.SetConfigFile(cfgFile)
				_ = v.ReadInConfig()
			}
			common.MergeLogSection(v)
			common.SetupLoggerWithFile(
				v.GetString("log.level"),
				v.GetString("log.format"),
				v.GetString("log.file"),
				v.GetInt("log.max_size"),
				v.GetInt("log.max_backups"),
				v.GetInt("log.max_age"),
				v.GetBool("log.compress"),
			)

			addr := v.GetString("addr")
			httpAddr := v.GetString("http_addr")
			cert := v.GetString("cert")
			key := v.GetString("key")
			ca := v.GetString("ca")
			gamesPath := v.GetString("games_config")

			if err := common.ValidateAddr(addr); err != nil {
				return fmt.Errorf("addr: %w", err)
			}
			if err := common.ValidateAddr(httpAddr); err != nil {
				return fmt.Errorf("http_addr: %w", err)
			}
			if err := common.ValidateTLS(cert, key, ca, true); err != nil {
				return err
			}

			creds, err := tlsutil.ServerTLS(cert, key, ca, true)
			if err != nil {
				return fmt.Errorf("load TLS: %w", err)
			}

			lis, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen: %w", err)
			}
			s := grpc.NewServer(grpc.Creds(creds), grpc.KeepaliveParams(keepalive.ServerParameters{}))

			gstore := games.NewStore(gamesPath)
			_ = gstore.Load()
			ctrl := controlserver.NewServer(gstore)
			controlv1.RegisterControlServiceServer(s, ctrl)
			tun := tunnelsrv.NewServer()
			tunnelv1.RegisterTunnelServiceServer(s, tun)
			fn := functionserver.NewEdgeServer(ctrl.Store(), tun)
			functionv1.RegisterFunctionServiceServer(s, fn)
			jobv1.RegisterJobServiceServer(s, jobserver.New(tun))

			var httpSrv *http.Server
			// Edge meta report (optional): report_url + token -> server
			if u := strings.TrimSpace(os.Getenv("EDGE_REPORT_URL")); u != "" {
				tok := strings.TrimSpace(os.Getenv("EDGE_REPORT_TOKEN"))
				if tok == "" {
					tok = strings.TrimSpace(os.Getenv("AGENT_META_TOKEN"))
				}
				interval := 30 * time.Second
				if v := strings.TrimSpace(os.Getenv("EDGE_REPORT_INTERVAL_SEC")); v != "" {
					if n, err := strconv.Atoi(v); err == nil && n > 0 {
						interval = time.Duration(n) * time.Second
					}
				}
				go func() {
					for {
						func() {
							body := strings.NewReader(fmt.Sprintf(`{"type":"edge","id":"%s","addr":"%s","http_addr":"%s","version":"%s"}`, "edge-1", addr, httpAddr, v.GetString("version")))
							req, _ := http.NewRequest(http.MethodPost, strings.TrimRight(u, "/")+"/api/ops/nodes/meta", body)
							req.Header.Set("Content-Type", "application/json")
							if tok != "" {
								req.Header.Set("X-Agent-Token", tok)
							}
							cli := &http.Client{Timeout: 2 * time.Second}
							if resp, err := cli.Do(req); err == nil {
								if resp.Body != nil {
									resp.Body.Close()
								}
							}
						}()
						time.Sleep(interval)
					}
				}()
			}
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
				r.GET("/metrics", func(c *gin.Context) {
					m := tun.MetricsMap()
					m["logs"] = common.GetLogCounters()
					c.JSON(http.StatusOK, m)
				})
				r.GET("/metrics.prom", func(c *gin.Context) {
					w := c.Writer
					w.Header().Set("Content-Type", "text/plain; version=0.0.4")
					lc := common.GetLogCounters()
					fmt.Fprintf(w, "# TYPE croupier_logs_total counter\n")
					fmt.Fprintf(w, "croupier_logs_total{level=\"debug\"} %d\n", lc["debug"])
					fmt.Fprintf(w, "croupier_logs_total{level=\"info\"} %d\n", lc["info"])
					fmt.Fprintf(w, "croupier_logs_total{level=\"warn\"} %d\n", lc["warn"])
					fmt.Fprintf(w, "croupier_logs_total{level=\"error\"} %d\n", lc["error"])
				})
				slog.Info("edge http listening", "addr", httpAddr)
				httpSrv = &http.Server{Addr: httpAddr, Handler: r}
				_ = httpSrv.ListenAndServe()
			}()
			slog.Info("edge listening", "addr", addr)
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
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cfgFile, "config", "", "config file (yaml)")
	cmd.Flags().String("addr", ":9443", "edge grpc listen")
	cmd.Flags().String("http_addr", ":9080", "edge http listen")
	cmd.Flags().String("cert", "", "TLS cert file")
	cmd.Flags().String("key", "", "TLS key file")
	cmd.Flags().String("ca", "", "CA cert file (client verify)")
	cmd.Flags().String("games_config", "configs/games.json", "allowed games config (json)")
	cmd.Flags().String("log.level", "info", "log level: debug|info|warn|error")
	cmd.Flags().String("log.format", "console", "log format: console|json")
	cmd.Flags().String("log.file", "", "log file path (if set, enable rotation)")
	cmd.Flags().Int("log.max_size", 100, "max size of log file in MB before rotation")
	cmd.Flags().Int("log.max_backups", 7, "max number of old log files to retain")
	cmd.Flags().Int("log.max_age", 7, "max age (days) to retain old log files")
	cmd.Flags().Bool("log.compress", true, "compress rotated log files")
	_ = viper.BindPFlags(cmd.Flags())
	return cmd
}
