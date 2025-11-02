package main

import (
    "flag"
    "log/slog"
    "os"
    "net"

    "google.golang.org/grpc"
    "google.golang.org/grpc/keepalive"

    controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
    functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
    controlserver "github.com/cuihairu/croupier/internal/server/control"
    functionserver "github.com/cuihairu/croupier/internal/edge/function"
    "github.com/cuihairu/croupier/internal/server/games"
    tunnelsrv "github.com/cuihairu/croupier/internal/edge/tunnel"
    tunnelv1 "github.com/cuihairu/croupier/pkg/pb/croupier/tunnel/v1"
    "net/http"
    "encoding/json"
    jobv1 "github.com/cuihairu/croupier/pkg/pb/croupier/edge/job/v1"
    jobserver "github.com/cuihairu/croupier/internal/edge/job"
    common "github.com/cuihairu/croupier/internal/cli/common"
    tlsutil "github.com/cuihairu/croupier/internal/tlsutil"
)

func main() {
    // initialize logger (stdout, console) before prints; can be overridden by env LOG_OUTPUT or config in other modes
    common.SetupLoggerWithFile("info", "console", "", 0, 0, 0, false)
    // Ports can be the same; FunctionService and ControlService share one listener.
    addr := flag.String("addr", ":9443", "edge grpc listen")
    cert := flag.String("cert", "", "TLS cert file")
    key := flag.String("key", "", "TLS key file")
    ca := flag.String("ca", "", "CA cert file (client verify)")
    gamesPath := flag.String("games_config", "configs/games.json", "allowed games config (json)")
    httpAddr := flag.String("http_addr", ":9080", "edge http listen for health/metrics")
    flag.Parse()

    if *cert == "" || *key == "" { slog.Error("TLS cert/key required"); os.Exit(1) }
    creds, err := tlsutil.ServerTLS(*cert, *key, *ca, true)
    if err != nil { slog.Error("load TLS", "error", err); os.Exit(1) }

    lis, err := net.Listen("tcp", *addr)
    if err != nil { slog.Error("listen", "error", err); os.Exit(1) }
    s := grpc.NewServer(grpc.Creds(creds), grpc.KeepaliveParams(keepalive.ServerParameters{}))

    gstore := games.NewStore(*gamesPath)
    _ = gstore.Load()
    ctrl := controlserver.NewServer(gstore)
    controlv1.RegisterControlServiceServer(s, ctrl)
    // Tunnel service for Agent connections
    tun := tunnelsrv.NewServer()
    tunnelv1.RegisterTunnelServiceServer(s, tun)
    // FunctionService at edge routes to Agent via tunnel, fallback to RPCAddr
    fn := functionserver.NewEdgeServer(ctrl.Store(), tun)
    functionv1.RegisterFunctionServiceServer(s, fn)
    // JobService for job_result query
    jobv1.RegisterJobServiceServer(s, jobserver.New(tun))

    // HTTP health/metrics
    go func(){
        mux := http.NewServeMux()
        mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.WriteHeader(http.StatusOK); _,_ = w.Write([]byte("ok")) })
        mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request){ _ = json.NewEncoder(w).Encode(tun.MetricsMap()) })
        slog.Info("edge http listening", "addr", *httpAddr)
        _ = http.ListenAndServe(*httpAddr, mux)
    }()
    slog.Info("edge listening", "addr", *addr)
    if err := s.Serve(lis); err != nil { slog.Error("serve", "error", err); os.Exit(1) }
}
