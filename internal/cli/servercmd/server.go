package servercmd

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "io/ioutil"
    "log"
    "net"
    "strings"
    "sync"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/keepalive"

    controlv1 "github.com/cuihairu/croupier/gen/go/croupier/control/v1"
    controlserver "github.com/cuihairu/croupier/internal/server/control"
    functionv1 "github.com/cuihairu/croupier/gen/go/croupier/function/v1"
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
)

// loadServerTLS is kept here to avoid leaking into other packages.
func loadServerTLS(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil { return nil, err }
    cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
    if caFile == "" { return nil, fmt.Errorf("ca certificate required for mTLS: provide --ca") }
    caPEM, err := ioutil.ReadFile(caFile)
    if err != nil { return nil, err }
    pool := x509.NewCertPool()
    if !pool.AppendCertsFromPEM(caPEM) { return nil, fmt.Errorf("failed to append CA") }
    cfg.ClientCAs = pool
    cfg.ClientAuth = tls.RequireAndVerifyClientCert
    return credentials.NewTLS(cfg), nil
}

// New returns the `croupier server` command.
func New() *cobra.Command {
    var cfgFile string
    cmd := &cobra.Command{
        Use:   "server",
        Short: "Run Croupier Server",
        RunE: func(cmd *cobra.Command, args []string) error {
            v := viper.GetViper()
            v.SetEnvPrefix("CROUPIER_SERVER")
            v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
            v.AutomaticEnv()
            if cfgFile != "" {
                v.SetConfigFile(cfgFile)
                if err := v.ReadInConfig(); err == nil {
                    log.Printf("[config] using %s", v.ConfigFileUsed())
                } else {
                    log.Printf("[warn] read config: %v", err)
                }
                if sub := v.Sub("server"); sub != nil {
                    v = sub // prefer sectioned config
                }
            }

            // logging setup
            common.SetupLogger(v.GetString("log.level"), v.GetString("log.format"))

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
                log.Printf("[devcert] generated dev TLS certs under %s (DEV ONLY)", out)
            }

            creds, err := loadServerTLS(cert, key, ca)
            if err != nil { return fmt.Errorf("load TLS: %w", err) }

            lis, err := net.Listen("tcp", addr)
            if err != nil { return fmt.Errorf("listen: %w", err) }

            s := grpc.NewServer(
                grpc.Creds(creds),
                grpc.KeepaliveParams(keepalive.ServerParameters{}),
            )

            // Allowed games store
            gstore := games.NewStore(gamesPath)
            if err := gstore.Load(); err != nil { return fmt.Errorf("load games: %w", err) }
            ctrl := controlserver.NewServer(gstore)
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
                log.Printf("croupier-server (grpc) listening on %s", addr)
                if err := s.Serve(lis); err != nil { log.Fatalf("serve grpc: %v", err) }
            }()
            go func() {
                defer wg.Done()
                aw, err := auditchain.NewWriter("logs/audit.log")
                if err != nil { log.Fatalf("audit: %v", err) }
                defer aw.Close()
                var pol *rbac.Policy
                if p, err := rbac.LoadPolicy(rbacPath); err == nil { pol = p } else { pol = rbac.NewPolicy(); pol.Grant("user:dev", "*"); pol.Grant("user:dev", "job:cancel"); pol.Grant("role:admin", "*") }
                var us *users.Store
                if s, err := users.Load(usersPath); err == nil { us = s } else { log.Printf("users load failed: %v", err) }
                jm := jwt.NewManager(jwtSecret)
                var statsProv interface{ GetStats() map[string]*loadbalancer.AgentStats; GetPoolStats() *connpool.PoolStats }
                if sp, ok := invoker.(interface{ GetStats() map[string]*loadbalancer.AgentStats; GetPoolStats() *connpool.PoolStats }); ok { statsProv = sp }
                httpSrv, err := httpserver.NewServer("descriptors", invoker, aw, pol, gstore, ctrl.Store(), us, jm, locator, statsProv)
                if err != nil { log.Fatalf("http server: %v", err) }
                if err := httpSrv.ListenAndServe(httpAddr); err != nil { log.Fatalf("serve http: %v", err) }
            }()
            wg.Wait()
            return nil
        },
    }
    // Flags and config binding
    cmd.Flags().StringVar(&cfgFile, "config", "", "config file (yaml), supports top-level 'server:' section")
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
    _ = viper.BindPFlags(cmd.Flags())
    return cmd
}
