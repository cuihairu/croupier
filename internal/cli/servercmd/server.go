package servercmd

import (
    "fmt"
    "net"
    "strings"
    "os"
    "log/slog"
    "sync"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "google.golang.org/grpc"
    "google.golang.org/grpc/keepalive"

    controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
    controlserver "github.com/cuihairu/croupier/internal/server/control"
    functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
    functionserver "github.com/cuihairu/croupier/internal/server/function"
    httpserver "github.com/cuihairu/croupier/internal/server/http"
    // register json codec
    _ "github.com/cuihairu/croupier/internal/transport/jsoncodec"
    auditchain "github.com/cuihairu/croupier/internal/audit/chain"
    rbac "github.com/cuihairu/croupier/internal/auth/rbac"
    
    jwt "github.com/cuihairu/croupier/internal/auth/token"
    "github.com/cuihairu/croupier/internal/devcert"
    "github.com/cuihairu/croupier/internal/loadbalancer"
    "github.com/cuihairu/croupier/internal/connpool"
    common "github.com/cuihairu/croupier/internal/cli/common"
    tlsutil "github.com/cuihairu/croupier/internal/tlsutil"
)

// loadServerTLS is kept here to avoid leaking into other packages.
// Deprecated: replaced by tlsutil.ServerTLS

// New returns the `croupier server` command.
func New() *cobra.Command {
    var cfgFile string
    var includes []string
    var profile string
    var perFn bool
    var perGameDenies bool
    cmd := &cobra.Command{
        Use:   "server",
        Short: "Run Croupier Server",
        RunE: func(cmd *cobra.Command, args []string) error {
            // load base + includes
            v, err := common.LoadWithIncludes(cfgFile, includes)
            if err != nil { return err }
            // env overlay
            v.SetEnvPrefix("CROUPIER_SERVER")
            v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
            v.AutomaticEnv()
            // apply section+profile
            if v, err = common.ApplySectionAndProfile(v, "server", profile); err != nil { return err }
            common.MergeLogSection(v)
            // metrics options toggles
            perFn = v.GetBool("metrics.per_function")
            perGameDenies = v.GetBool("metrics.per_game_denies")
            httpserver.SetMetricsOptions(perFn, perGameDenies)

            // logging setup
            common.SetupLoggerWithFile(
                v.GetString("log.level"),
                v.GetString("log.format"),
                v.GetString("log.file"),
                v.GetInt("log.max_size"),
                v.GetInt("log.max_backups"),
                v.GetInt("log.max_age"),
                v.GetBool("log.compress"),
            )

            // DB/Storage config → env bridge
            if v.IsSet("db.driver") { _ = os.Setenv("DB_DRIVER", v.GetString("db.driver")) }
            if v.IsSet("db.dsn") { _ = os.Setenv("DATABASE_URL", v.GetString("db.dsn")) }
            if vv := v.GetString("storage.driver"); vv != "" { _ = os.Setenv("STORAGE_DRIVER", vv) }
            if vv := v.GetString("storage.bucket"); vv != "" { _ = os.Setenv("STORAGE_BUCKET", vv) }
            if vv := v.GetString("storage.region"); vv != "" { _ = os.Setenv("STORAGE_REGION", vv) }
            if vv := v.GetString("storage.endpoint"); vv != "" { _ = os.Setenv("STORAGE_ENDPOINT", vv) }
            if vv := v.GetString("storage.access_key"); vv != "" { _ = os.Setenv("STORAGE_ACCESS_KEY", vv); _ = os.Setenv("AWS_ACCESS_KEY_ID", vv) }
            if vv := v.GetString("storage.secret_key"); vv != "" { _ = os.Setenv("STORAGE_SECRET_KEY", vv); _ = os.Setenv("AWS_SECRET_ACCESS_KEY", vv) }
            if v.IsSet("storage.force_path_style") { _ = os.Setenv("STORAGE_FORCE_PATH_STYLE", fmt.Sprintf("%v", v.GetBool("storage.force_path_style"))) }
            if vv := v.GetString("storage.base_dir"); vv != "" { _ = os.Setenv("STORAGE_BASE_DIR", vv) }
            if vv := v.GetString("storage.signed_url_ttl"); vv != "" { _ = os.Setenv("STORAGE_SIGNED_URL_TTL", vv) }

            // config validation (non-strict: allow devcert fallback)
            if err := common.ValidateServerConfig(v, false); err != nil { return fmt.Errorf("config invalid: %w", err) }

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
                if err != nil { return fmt.Errorf("generate dev CA: %w", err) }
                srvCrt, srvKey, err := devcert.EnsureServerCert(out, caCrt, caKey, []string{"localhost", "127.0.0.1"})
                if err != nil { return fmt.Errorf("generate dev server cert: %w", err) }
                cert, key, ca = srvCrt, srvKey, caCrt
                slog.Info("devcert generated", "dir", out)
            }

            creds, err := tlsutil.ServerTLS(cert, key, ca, true)
            if err != nil { return fmt.Errorf("load TLS: %w", err) }

            lis, err := net.Listen("tcp", addr)
            if err != nil { return fmt.Errorf("listen: %w", err) }

            s := grpc.NewServer(
                grpc.Creds(creds),
                grpc.KeepaliveParams(keepalive.ServerParameters{}),
            )

            // DB-backed games; ignore legacy gamesPath and use empty allowlist for control server
            _ = gamesPath
            ctrl := controlserver.NewServer(nil)
            controlv1.RegisterControlServiceServer(s, ctrl)
            var invoker httpserver.FunctionInvoker
            var locator interface{ GetJobAddr(string) (string, bool) }
            if edgeAddr != "" {
                fwd := functionserver.NewForwarder(edgeAddr)
                functionv1.RegisterFunctionServiceServer(s, fwd)
                invoker = functionserver.NewForwarderInvoker(fwd)
                locator = nil
            } else {
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
                if err := s.Serve(lis); err != nil { slog.Error("serve grpc", "error", err); os.Exit(1) }
            }()
            go func() {
                defer wg.Done()
                aw, err := auditchain.NewWriter("logs/audit.log")
                if err != nil { slog.Error("audit", "error", err); os.Exit(1) }
                defer aw.Close()
                var pol *rbac.Policy
                if p, err := rbac.LoadPolicy(rbacPath); err == nil { pol = p } else { pol = rbac.NewPolicy(); pol.Grant("user:dev", "*"); pol.Grant("user:dev", "job:cancel"); pol.Grant("role:admin", "*") }
                // DB-backed users; ignore legacy usersPath
                _ = usersPath
                jm := jwt.NewManager(jwtSecret)
                var statsProv interface{ GetStats() map[string]*loadbalancer.AgentStats; GetPoolStats() *connpool.PoolStats }
                if sp, ok := invoker.(interface{ GetStats() map[string]*loadbalancer.AgentStats; GetPoolStats() *connpool.PoolStats }); ok { statsProv = sp }
                httpSrv, err := httpserver.NewServer("gen/croupier", invoker, aw, pol, ctrl.Store(), jm, locator, statsProv)
                if err != nil { slog.Error("http server", "error", err); os.Exit(1) }
                if err := httpSrv.ListenAndServe(httpAddr); err != nil { slog.Error("serve http", "error", err); os.Exit(1) }
            }()
            wg.Wait()
            return nil
        },
    }
    // Flags and config binding
    cmd.Flags().StringVar(&cfgFile, "config", "", "config file (yaml), supports top-level 'server:' section")
    cmd.Flags().StringSliceVar(&includes, "config-include", nil, "additional config files to merge in order (overrides base)")
    cmd.Flags().StringVar(&profile, "profile", "", "profile name under 'profiles:' to overlay")
    cmd.Flags().String("addr", ":8443", "grpc listen address")
    cmd.Flags().String("http_addr", ":8080", "http api listen address")
    cmd.Flags().String("edge_addr", "", "optional edge address for forwarding function calls (DEV PoC)")
    cmd.Flags().String("rbac_config", "configs/rbac.json", "rbac policy json path")
    cmd.Flags().String("cert", "", "server cert file")
    cmd.Flags().String("key", "", "server key file")
    cmd.Flags().String("ca", "", "ca cert file for client cert verification")
    cmd.Flags().String("games_config", "configs/games.json", "allowed games config (json)")
    cmd.Flags().String("users_config", "configs/users.json", "users config json")
    cmd.Flags().String("jwt_secret", "dev-secret", "jwt hs256 secret")
    cmd.Flags().String("log.level", "info", "log level: debug|info|warn|error")
    cmd.Flags().String("log.format", "console", "log format: console|json")
    cmd.Flags().String("log.file", "", "log file path (if set, enable rotation)")
    cmd.Flags().Int("log.max_size", 100, "max size of log file in MB before rotation")
    cmd.Flags().Int("log.max_backups", 7, "max number of old log files to retain")
    cmd.Flags().Int("log.max_age", 7, "max age (days) to retain old log files")
    cmd.Flags().Bool("log.compress", true, "compress rotated log files")
    cmd.Flags().Bool("metrics.per_function", true, "export per-function metrics (invocations/errors/latency)")
    cmd.Flags().Bool("metrics.per_game_denies", false, "export RBAC denied counts per game within per-function metrics")
    cmd.Flags().String("db.driver", "auto", "database driver: postgres|mysql|sqlite|auto")
    cmd.Flags().String("db.dsn", "", "database DSN/URL; for sqlite can be file:path.db or :memory:")
    _ = viper.BindPFlags(cmd.Flags())
    return cmd
}
