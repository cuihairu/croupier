package agentcmd

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net"
    "net/http"
    "strings"
    "time"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/keepalive"

    controlclient "github.com/cuihairu/croupier/internal/agent/control"
    controlv1 "github.com/cuihairu/croupier/gen/go/croupier/control/v1"
    functionv1 "github.com/cuihairu/croupier/gen/go/croupier/function/v1"
    agentfunc "github.com/cuihairu/croupier/internal/agent/function"
    localv1 "github.com/cuihairu/croupier/gen/go/croupier/agent/local/v1"
    locallib "github.com/cuihairu/croupier/internal/agent/local"
    localreg "github.com/cuihairu/croupier/internal/agent/registry"
    "github.com/cuihairu/croupier/internal/agent/jobs"
    tunn "github.com/cuihairu/croupier/internal/agent/tunnel"
    _ "github.com/cuihairu/croupier/internal/transport/jsoncodec"
    "github.com/cuihairu/croupier/internal/devcert"
    common "github.com/cuihairu/croupier/internal/cli/common"
)

func loadClientTLS(certFile, keyFile, caFile string, serverName string) (credentials.TransportCredentials, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil { return nil, err }
    caPEM, err := ioutil.ReadFile(caFile)
    if err != nil { return nil, err }
    pool := x509.NewCertPool()
    if !pool.AppendCertsFromPEM(caPEM) { return nil, err }
    cfg := &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: pool, ServerName: serverName}
    return credentials.NewTLS(cfg), nil
}

// New returns the `croupier agent` command.
func New() *cobra.Command {
    var cfgFile string
    cmd := &cobra.Command{
        Use:   "agent",
        Short: "Run Croupier Agent",
        RunE: func(cmd *cobra.Command, args []string) error {
            v := viper.GetViper()
            v.SetEnvPrefix("CROUPIER_AGENT")
            v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
            v.AutomaticEnv()
            if cfgFile != "" {
                v.SetConfigFile(cfgFile)
                if err := v.ReadInConfig(); err == nil {
                    log.Printf("[config] using %s", v.ConfigFileUsed())
                } else { log.Printf("[warn] read config: %v", err) }
                if sub := v.Sub("agent"); sub != nil { v = sub }
            }
            common.MergeLogSection(v)

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

            localAddr := v.GetString("local_addr")
            serverAddr := v.GetString("server_addr")
            coreAddr := v.GetString("core_addr")
            serverName := v.GetString("server_name")
            cert := v.GetString("cert")
            key := v.GetString("key")
            ca := v.GetString("ca")
            insecureLocal := v.GetBool("insecure_local")
            agentID := v.GetString("agent_id")
            agentVersion := v.GetString("agent_version")
            gameID := v.GetString("game_id")
            env := v.GetString("env")
            httpAddr := v.GetString("http_addr")

            if serverAddr != "" {
                if coreAddr != "" && coreAddr != "127.0.0.1:8443" {
                    log.Printf("[warn] both --server_addr and --core_addr provided; using --server_addr=%s", serverAddr)
                }
                coreAddr = serverAddr
            } else if coreAddr != "" {
                log.Printf("[warn] --core_addr is deprecated; please use --server_addr")
            }

            // Validate config (non-strict) then auto-generate dev certs when not provided (DEV ONLY)
            if err := common.ValidateAgentConfig(v, false); err != nil { return err }
            // Auto-generate dev certs when not provided (DEV ONLY)
            if (cert == "" || key == "" || ca == "") && coreAddr != "" {
                out := "configs/dev"
                caCrt, caKey, err := devcert.EnsureDevCA(out)
                if err != nil { return err }
                agCrt, agKey, err := devcert.EnsureAgentCert(out, caCrt, caKey, agentID)
                if err != nil { return err }
                cert, key, ca = agCrt, agKey, caCrt
                log.Printf("[devcert] generated dev mTLS certs under %s (DEV ONLY)", out)
            }

            // Connect to Server with mTLS
            var dialOpt grpc.DialOption
            if cert != "" && key != "" && ca != "" {
                sni := serverName
                if sni == "" {
                    host := coreAddr
                    if i := strings.Index(host, "://"); i >= 0 { host = host[i+3:] }
                    if i := strings.LastIndex(host, ":"); i >= 0 { host = host[:i] }
                    sni = host
                }
                creds, err := loadClientTLS(cert, key, ca, sni)
                if err != nil { return err }
                dialOpt = grpc.WithTransportCredentials(creds)
            } else {
                return fmt.Errorf("missing TLS cert/key/ca; provide --cert/--key/--ca or set Insecure for dev")
            }

            coreConn, err := grpc.Dial(coreAddr, dialOpt, grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second}), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
            if err != nil { return err }
            defer coreConn.Close()

            // Bootstrap register/heartbeat
            go func() {
                cc := controlclient.NewClient(coreConn)
                fns := []*controlv1.FunctionDescriptor{}
                ctx := context.Background()
                cc.RegisterAndHeartbeat(ctx, agentID, agentVersion, localAddr, gameID, env, fns)
            }()

            // Local gRPC for game servers
            lis, err := net.Listen("tcp", localAddr)
            if err != nil { return err }

            var srv *grpc.Server
            if insecureLocal { srv = grpc.NewServer() } else { return fmt.Errorf("secure local server not implemented; set --insecure_local") }

            lstore := localreg.NewLocalStore()
            exec := jobs.NewExecutor()
            functionv1.RegisterFunctionServiceServer(srv, agentfunc.NewServer(lstore, exec))
            localv1.RegisterLocalControlServiceServer(srv, locallib.NewServer(lstore, controlv1.NewControlServiceClient(coreConn), agentID, agentVersion, localAddr, gameID, env, exec))

            // Tunnel to Server/Edge
            go func(){
                t := tunn.NewClient(coreAddr, agentID, gameID, env, localAddr)
                backoff := time.Second
                for {
                    if err := t.Start(context.Background()); err != nil { log.Printf("tunnel disconnected: %v", err) }
                    time.Sleep(backoff)
                    if backoff < 30*time.Second { backoff *= 2 }
                    tunn.IncReconnect()
                }
            }()

            // HTTP health/metrics
            go func(){
                mux := http.NewServeMux()
                mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.WriteHeader(http.StatusOK); _,_ = w.Write([]byte("ok")) })
                mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request){
                    mp := lstore.List(); total := 0; fns := 0
                    for _, arr := range mp { fns++; total += len(arr) }
                    _ = json.NewEncoder(w).Encode(map[string]any{
                        "functions": fns,
                        "instances": total,
                        "tunnel_reconnects": tunn.Reconnects(),
                        "logs": common.GetLogCounters(),
                    })
                })
                log.Printf("agent http listening on %s", httpAddr)
                _ = http.ListenAndServe(httpAddr, mux)
            }()
            log.Printf("croupier-agent listening on %s; connected to server %s", localAddr, coreAddr)
            if err := srv.Serve(lis); err != nil { return err }
            return nil
        },
    }
    cmd.Flags().StringVar(&cfgFile, "config", "", "config file (yaml), supports top-level 'agent:' section")
    cmd.Flags().String("local_addr", ":19090", "local gRPC listen for game servers")
    cmd.Flags().String("server_addr", "", "server grpc address (alias for --core_addr)")
    cmd.Flags().String("core_addr", "127.0.0.1:8443", "server grpc address (deprecated)")
    cmd.Flags().String("server_name", "", "tls server name (SNI)")
    cmd.Flags().String("cert", "", "client mTLS cert file")
    cmd.Flags().String("key", "", "client mTLS key file")
    cmd.Flags().String("ca", "", "ca cert file to verify server")
    cmd.Flags().Bool("insecure_local", true, "use insecure for local listener (development)")
    cmd.Flags().String("agent_id", "agent-1", "agent id")
    cmd.Flags().String("agent_version", "0.1.0", "agent version")
    cmd.Flags().String("game_id", "", "game id (required if server enforces whitelist)")
    cmd.Flags().String("env", "", "environment (optional) e.g. prod/stage/test")
    cmd.Flags().String("http_addr", ":19091", "agent http listen for health/metrics")
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
