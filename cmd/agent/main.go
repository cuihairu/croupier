package main

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "flag"
    "io/ioutil"
    "log"
    "net"
    "time"
    "net/http"
    "encoding/json"
    "strings"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    // "google.golang.org/grpc/credentials/insecure"
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
    // register json codec
    _ "github.com/cuihairu/croupier/internal/transport/jsoncodec"
    "github.com/cuihairu/croupier/internal/devcert"
)

func loadClientTLS(certFile, keyFile, caFile string, serverName string) (credentials.TransportCredentials, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, err
    }
    caPEM, err := ioutil.ReadFile(caFile)
    if err != nil {
        return nil, err
    }
    pool := x509.NewCertPool()
    if !pool.AppendCertsFromPEM(caPEM) {
        return nil, err
    }
    cfg := &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:      pool,
        ServerName:   serverName,
    }
    return credentials.NewTLS(cfg), nil
}

func main() {
    localAddr := flag.String("local_addr", ":19090", "local gRPC listen for game servers")
    coreAddr := flag.String("core_addr", "127.0.0.1:8443", "server grpc address")
    serverName := flag.String("server_name", "", "tls server name for server (SNI)")
    cert := flag.String("cert", "", "client mTLS cert file")
    key := flag.String("key", "", "client mTLS key file")
    ca := flag.String("ca", "", "ca cert file to verify server")
    insecureLocal := flag.Bool("insecure_local", true, "use insecure for local listener (development)")
    agentID := flag.String("agent_id", "agent-1", "agent id")
    agentVersion := flag.String("agent_version", "0.1.0", "agent version")
    gameID := flag.String("game_id", "", "game id (required if server enforces whitelist)")
    env := flag.String("env", "", "environment (optional) e.g. prod/stage/test")
    httpAddr := flag.String("http_addr", ":19091", "agent http listen for health/metrics")
    flag.Parse()

    // Auto-generate dev certs when not provided (DEV ONLY)
    if (*cert == "" || *key == "" || *ca == "") && *coreAddr != "" {
        out := "configs/dev"
        caCrt, caKey, err := devcert.EnsureDevCA(out)
        if err != nil { log.Fatalf("generate dev CA: %v", err) }
        agCrt, agKey, err := devcert.EnsureAgentCert(out, caCrt, caKey, *agentID)
        if err != nil { log.Fatalf("generate dev agent cert: %v", err) }
        *cert, *key, *ca = agCrt, agKey, caCrt
        log.Printf("[devcert] generated dev mTLS certs under %s (DEV ONLY)", out)
    }

    // Connect to Server with mTLS (required by default)
    var dialOpt grpc.DialOption
    if *cert != "" && *key != "" && *ca != "" {
        // Default SNI from core_addr host if not provided
        sni := *serverName
        if sni == "" {
            host := *coreAddr
            if i := strings.Index(host, "://"); i >= 0 { host = host[i+3:] }
            if i := strings.LastIndex(host, ":"); i >= 0 { host = host[:i] }
            sni = host
        }
        creds, err := loadClientTLS(*cert, *key, *ca, sni)
        if err != nil {
            log.Fatalf("load TLS: %v", err)
        }
        dialOpt = grpc.WithTransportCredentials(creds)
    } else {
        log.Fatalf("TLS cert/key/ca required for agent outbound; provide --cert/--key/--ca")
    }

    coreConn, err := grpc.Dial(*coreAddr, dialOpt, grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second}), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
    if err != nil {
        log.Fatalf("dial server: %v", err)
    }
    defer coreConn.Close()

    // Bootstrap register/heartbeat (placeholder function list; Local server will update on RegisterLocal)
    go func() {
        cc := controlclient.NewClient(coreConn)
        fns := []*controlv1.FunctionDescriptor{}
        ctx := context.Background()
        cc.RegisterAndHeartbeat(ctx, *agentID, *agentVersion, *localAddr, *gameID, *env, fns)
    }()

    // Local gRPC for game servers to connect
    lis, err := net.Listen("tcp", *localAddr)
    if err != nil {
        log.Fatalf("listen local: %v", err)
    }

    var srv *grpc.Server
    if *insecureLocal {
        srv = grpc.NewServer()
    } else {
        log.Fatalf("secure local server not implemented in skeleton; run with --insecure_local")
    }

    // Local registry (function id -> local game server endpoint/version)
    lstore := localreg.NewLocalStore()
    exec := jobs.NewExecutor()
    // Register local FunctionService endpoint (routes to local game servers & job executor)
    functionv1.RegisterFunctionServiceServer(srv, agentfunc.NewServer(lstore, exec))
    // Register LocalControl service for SDKs to register themselves
    localv1.RegisterLocalControlServiceServer(srv, locallib.NewServer(lstore, controlv1.NewControlServiceClient(coreConn), *agentID, *agentVersion, *localAddr, *gameID, *env, exec))
    // Open tunnel to Edge/Server for Invoke proxy
    go func(){
        t := tunn.NewClient(*coreAddr, *agentID, *gameID, *env, *localAddr)
        backoff := time.Second
        for {
            err := t.Start(context.Background())
            if err != nil { log.Printf("tunnel disconnected: %v", err) }
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
            // summarize instances
            mp := lstore.List(); total := 0; fns := 0
            for _, arr := range mp { fns++; total += len(arr) }
            _ = json.NewEncoder(w).Encode(map[string]any{"functions": fns, "instances": total, "tunnel_reconnects": tunn.Reconnects()})
        })
        log.Printf("agent http listening on %s", *httpAddr)
        _ = http.ListenAndServe(*httpAddr, mux)
    }()
    log.Printf("croupier-agent listening on %s; connected to server %s", *localAddr, *coreAddr)
    // prune stale instances periodically
    go func(){
        ticker := time.NewTicker(30 * time.Second); defer ticker.Stop()
        for range ticker.C { removed := lstore.Prune(60*time.Second); if removed > 0 { log.Printf("pruned %d stale local instances", removed) } }
    }()
    if err := srv.Serve(lis); err != nil {
        log.Fatalf("serve local: %v", err)
    }
}
