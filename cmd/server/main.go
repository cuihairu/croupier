package main

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "io/ioutil"
    "log/slog"
    "os"
    "net"
    "sync"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
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
    "github.com/cuihairu/croupier/internal/server/games"
    users "github.com/cuihairu/croupier/internal/auth/users"
    jwt "github.com/cuihairu/croupier/internal/auth/token"
    "github.com/cuihairu/croupier/internal/devcert"
    "github.com/cuihairu/croupier/internal/loadbalancer"
    "github.com/cuihairu/croupier/internal/connpool"
    "strings"
    common "github.com/cuihairu/croupier/internal/cli/common"
)

// loadServerTLS builds a tls.Config for mTLS if caFile is provided.
func loadServerTLS(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, err
    }
    cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
    // Enforce mTLS: require CA for client verification
    if caFile == "" { return nil, fmt.Errorf("ca certificate required for mTLS: provide --ca") }
    caPEM, err := ioutil.ReadFile(caFile)
    if err != nil { return nil, err }
    pool := x509.NewCertPool()
    if !pool.AppendCertsFromPEM(caPEM) { return nil, fmt.Errorf("failed to append CA") }
    cfg.ClientCAs = pool
    cfg.ClientAuth = tls.RequireAndVerifyClientCert
    return credentials.NewTLS(cfg), nil
}

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
            // set up logger (stdout by default); honor log.* if present
            common.MergeLogSection(viper.GetViper())
            if viper.IsSet("log.output") {
                // pass to logger via env bridge
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

            addr := viper.GetString("addr")
            httpAddr := viper.GetString("http_addr")
            edgeAddr := viper.GetString("edge_addr")
            rbacPath := viper.GetString("rbac_config")
            cert := viper.GetString("cert")
            key := viper.GetString("key")
            ca := viper.GetString("ca")
            gamesPath := viper.GetString("games_config")
            usersPath := viper.GetString("users_config")
            jwtSecret := viper.GetString("jwt_secret")

            // Auto-generate dev certs when not provided (DEV ONLY)
            if cert == "" || key == "" || ca == "" {
                out := "configs/dev"
                caCrt, caKey, err := devcert.EnsureDevCA(out)
                if err != nil { slog.Error("generate dev CA", "error", err); os.Exit(1) }
                // include common localhost hosts for dev
                srvCrt, srvKey, err := devcert.EnsureServerCert(out, caCrt, caKey, []string{"localhost", "127.0.0.1"})
                if err != nil { slog.Error("generate dev server cert", "error", err); os.Exit(1) }
                // set to generated paths
                cert, key, ca = srvCrt, srvKey, caCrt
                slog.Info("devcert generated", "dir", out)
            }

            creds, err := loadServerTLS(cert, key, ca)
            if err != nil { slog.Error("load TLS", "error", err); os.Exit(1) }

            lis, err := net.Listen("tcp", addr)
            if err != nil { slog.Error("listen", "error", err); os.Exit(1) }

            s := grpc.NewServer(
                grpc.Creds(creds),
                grpc.KeepaliveParams(keepalive.ServerParameters{}),
            )

            // Register services
            // Allowed games store
            gstore := games.NewStore(gamesPath)
            if err := gstore.Load(); err != nil { slog.Error("load games", "error", err); os.Exit(1) }
            ctrl := controlserver.NewServer(gstore)
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
                if err := s.Serve(lis); err != nil { slog.Error("serve grpc", "error", err); os.Exit(1) }
            }()
            go func() {
                defer wg.Done()
                aw, err := auditchain.NewWriter("logs/audit.log")
                if err != nil { slog.Error("audit", "error", err); os.Exit(1) }
                defer aw.Close()
                var pol *rbac.Policy
                if p, err := rbac.LoadPolicy(rbacPath); err == nil { pol = p } else { pol = rbac.NewPolicy(); pol.Grant("user:dev", "*"); pol.Grant("user:dev", "job:cancel"); pol.Grant("role:admin", "*") }
                var us *users.Store
                if s, err := users.Load(usersPath); err == nil { us = s } else { slog.Warn("users load failed", "error", err) }
                jm := jwt.NewManager(jwtSecret)
                var statsProv interface{ GetStats() map[string]*loadbalancer.AgentStats; GetPoolStats() *connpool.PoolStats }
                if sp, ok := invoker.(interface{ GetStats() map[string]*loadbalancer.AgentStats; GetPoolStats() *connpool.PoolStats }); ok { statsProv = sp }
                httpSrv, err := httpserver.NewServer("gen/croupier", invoker, aw, pol, gstore, ctrl.Store(), us, jm, locator, statsProv)
                if err != nil { slog.Error("http server", "error", err); os.Exit(1) }
                if err := httpSrv.ListenAndServe(httpAddr); err != nil { slog.Error("serve http", "error", err); os.Exit(1) }
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
    _ = viper.BindPFlags(root.Flags())

    if err := root.Execute(); err != nil { slog.Error("server exit", "error", err); os.Exit(1) }
}
