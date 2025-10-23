package main

import (
    "crypto/tls"
    "crypto/x509"
    "flag"
    "io/ioutil"
    "log"
    "net"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/keepalive"

    controlv1 "github.com/your-org/croupier/gen/go/croupier/control/v1"
    functionv1 "github.com/your-org/croupier/gen/go/croupier/function/v1"
    controlserver "github.com/your-org/croupier/internal/server/control"
    functionserver "github.com/your-org/croupier/internal/edge/function"
    "github.com/your-org/croupier/internal/server/games"
    tunnelsrv "github.com/your-org/croupier/internal/edge/tunnel"
    tunnelv1 "github.com/your-org/croupier/gen/go/croupier/tunnel/v1"
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

func main() {
    // Ports can be the same; FunctionService and ControlService share one listener.
    addr := flag.String("addr", ":9443", "edge grpc listen")
    cert := flag.String("cert", "", "TLS cert file")
    key := flag.String("key", "", "TLS key file")
    ca := flag.String("ca", "", "CA cert file (client verify)")
    gamesPath := flag.String("games_config", "configs/games.json", "allowed games config (json)")
    flag.Parse()

    if *cert == "" || *key == "" { log.Fatal("TLS cert/key required") }
    creds, err := loadTLS(*cert, *key, *ca, true)
    if err != nil { log.Fatalf("load TLS: %v", err) }

    lis, err := net.Listen("tcp", *addr)
    if err != nil { log.Fatalf("listen: %v", err) }
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

    log.Printf("edge listening on %s", *addr)
    if err := s.Serve(lis); err != nil { log.Fatalf("serve: %v", err) }
}
