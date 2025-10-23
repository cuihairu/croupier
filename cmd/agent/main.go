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

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/keepalive"

    controlclient "github.com/your-org/croupier/internal/agent/control"
    controlv1 "github.com/your-org/croupier/gen/go/croupier/control/v1"
    functionv1 "github.com/your-org/croupier/gen/go/croupier/function/v1"
    agentfunc "github.com/your-org/croupier/internal/agent/function"
    localv1 "github.com/your-org/croupier/gen/go/croupier/agent/local/v1"
    locallib "github.com/your-org/croupier/internal/agent/local"
    localreg "github.com/your-org/croupier/internal/agent/registry"
    "github.com/your-org/croupier/internal/agent/jobs"
    // register json codec
    _ "github.com/your-org/croupier/internal/transport/jsoncodec"
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
    coreAddr := flag.String("core_addr", "127.0.0.1:8443", "core grpc address")
    serverName := flag.String("server_name", "", "tls server name for core (SNI)")
    cert := flag.String("cert", "", "client mTLS cert file")
    key := flag.String("key", "", "client mTLS key file")
    ca := flag.String("ca", "", "ca cert file to verify core")
    insecureLocal := flag.Bool("insecure_local", true, "use insecure for local listener (development)")
    agentID := flag.String("agent_id", "agent-1", "agent id")
    agentVersion := flag.String("agent_version", "0.1.0", "agent version")
    gameID := flag.String("game_id", "", "game id (required if server enforces whitelist)")
    env := flag.String("env", "", "environment (optional) e.g. prod/stage/test")
    flag.Parse()

    // Connect to Core with mTLS
    var dialOpt grpc.DialOption
    if *cert != "" && *key != "" && *ca != "" {
        creds, err := loadClientTLS(*cert, *key, *ca, *serverName)
        if err != nil {
            log.Fatalf("load TLS: %v", err)
        }
        dialOpt = grpc.WithTransportCredentials(creds)
    } else {
        log.Printf("WARNING: no mTLS provided, using insecure dial to core for development")
        dialOpt = grpc.WithTransportCredentials(insecure.NewCredentials())
    }

    coreConn, err := grpc.Dial(*coreAddr, dialOpt, grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second}), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
    if err != nil {
        log.Fatalf("dial core: %v", err)
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
    localv1.RegisterLocalControlServiceServer(srv, locallib.NewServer(lstore, controlv1.NewControlServiceClient(coreConn), *agentID, *agentVersion, *localAddr, *gameID, *env))

    log.Printf("croupier-agent listening on %s; connected to core %s", *localAddr, *coreAddr)
    if err := srv.Serve(lis); err != nil {
        log.Fatalf("serve local: %v", err)
    }
}
