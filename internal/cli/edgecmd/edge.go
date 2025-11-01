package edgecmd

import (
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net"
    "net/http"
    "strings"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/keepalive"

    controlv1 "github.com/cuihairu/croupier/gen/go/croupier/control/v1"
    functionv1 "github.com/cuihairu/croupier/gen/go/croupier/function/v1"
    controlserver "github.com/cuihairu/croupier/internal/server/control"
    functionserver "github.com/cuihairu/croupier/internal/edge/function"
    "github.com/cuihairu/croupier/internal/server/games"
    tunnelsrv "github.com/cuihairu/croupier/internal/edge/tunnel"
    tunnelv1 "github.com/cuihairu/croupier/gen/go/croupier/tunnel/v1"
    jobv1 "github.com/cuihairu/croupier/gen/go/croupier/edge/job/v1"
    jobserver "github.com/cuihairu/croupier/internal/edge/job"
    common "github.com/cuihairu/croupier/internal/cli/common"
)

func loadTLS(certFile, keyFile, caFile string, requireClient bool) (credentials.TransportCredentials, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil { return nil, err }
    cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
    if caFile != "" {
        caPEM, err := ioutil.ReadFile(caFile)
        if err != nil { return nil, err }
        pool := x509.NewCertPool(); pool.AppendCertsFromPEM(caPEM)
        cfg.ClientCAs = pool
        if requireClient { cfg.ClientAuth = tls.RequireAndVerifyClientCert }
    }
    return credentials.NewTLS(cfg), nil
}

// New returns `croupier edge` command.
func New() *cobra.Command {
    var cfgFile string
    cmd := &cobra.Command{ Use: "edge", Short: "Run Croupier Edge (forwarder)",
        RunE: func(cmd *cobra.Command, args []string) error {
            v := viper.GetViper()
            v.SetEnvPrefix("CROUPIER_EDGE")
            v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
            v.AutomaticEnv()
            if cfgFile != "" { v.SetConfigFile(cfgFile); _ = v.ReadInConfig() }
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

            if err := common.ValidateAddr(addr); err != nil { return fmt.Errorf("addr: %w", err) }
            if err := common.ValidateAddr(httpAddr); err != nil { return fmt.Errorf("http_addr: %w", err) }
            if err := common.ValidateTLS(cert, key, ca, true); err != nil { return err }

            creds, err := loadTLS(cert, key, ca, true)
            if err != nil { return fmt.Errorf("load TLS: %w", err) }

            lis, err := net.Listen("tcp", addr)
            if err != nil { return fmt.Errorf("listen: %w", err) }
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

            go func(){
                mux := http.NewServeMux()
                mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.WriteHeader(http.StatusOK); _,_ = w.Write([]byte("ok")) })
                mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request){ _ = json.NewEncoder(w).Encode(tun.MetricsMap()) })
                log.Printf("edge http listening on %s", httpAddr)
                _ = http.ListenAndServe(httpAddr, mux)
            }()
            log.Printf("edge listening on %s", addr)
            if err := s.Serve(lis); err != nil { return err }
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
