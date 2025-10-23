package main

import (
    "crypto/tls"
    "crypto/x509"
    "flag"
    "io/ioutil"
    "log"
    "net"
    "sync"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/keepalive"

    controlv1 "github.com/your-org/croupier/gen/go/croupier/control/v1"
    controlserver "github.com/your-org/croupier/internal/server/control"
    functionv1 "github.com/your-org/croupier/gen/go/croupier/function/v1"
    functionserver "github.com/your-org/croupier/internal/server/function"
    httpserver "github.com/your-org/croupier/internal/server/http"
    // register json codec
    _ "github.com/your-org/croupier/internal/transport/jsoncodec"
    auditchain "github.com/your-org/croupier/internal/audit/chain"
    rbac "github.com/your-org/croupier/internal/auth/rbac"
    "github.com/your-org/croupier/internal/server/games"
)

// loadServerTLS builds a tls.Config for mTLS if caFile is provided.
func loadServerTLS(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, err
    }
    cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
    if caFile != "" {
        caPEM, err := ioutil.ReadFile(caFile)
        if err != nil {
            return nil, err
        }
        pool := x509.NewCertPool()
        if !pool.AppendCertsFromPEM(caPEM) {
            return nil, err
        }
        cfg.ClientCAs = pool
        cfg.ClientAuth = tls.RequireAndVerifyClientCert
    }
    return credentials.NewTLS(cfg), nil
}

func main() {
    addr := flag.String("addr", ":8443", "grpc listen address")
    httpAddr := flag.String("http_addr", ":8080", "http api listen address")
    rbacPath := flag.String("rbac_config", "configs/rbac.json", "rbac policy json path")
    cert := flag.String("cert", "", "server cert file")
    key := flag.String("key", "", "server key file")
    ca := flag.String("ca", "", "ca cert file for client cert verification (optional)")
    gamesPath := flag.String("games_config", "configs/games.json", "allowed games config (json)")
    flag.Parse()

    if *cert == "" || *key == "" {
        log.Fatal("TLS cert/key required: use --cert and --key")
    }

    creds, err := loadServerTLS(*cert, *key, *ca)
    if err != nil {
        log.Fatalf("load TLS: %v", err)
    }

    lis, err := net.Listen("tcp", *addr)
    if err != nil {
        log.Fatalf("listen: %v", err)
    }

    s := grpc.NewServer(
        grpc.Creds(creds),
        grpc.KeepaliveParams(keepalive.ServerParameters{}),
    )

    // Register services
    // Allowed games store
    gstore := games.NewStore(*gamesPath)
    if err := gstore.Load(); err != nil { log.Fatalf("load games: %v", err) }
    ctrl := controlserver.NewServer(gstore)
    controlv1.RegisterControlServiceServer(s, ctrl)
    fnsrv := functionserver.NewServer(ctrl.Store())
    functionv1.RegisterFunctionServiceServer(s, fnsrv)

    var wg sync.WaitGroup
    wg.Add(2)
    go func() {
        defer wg.Done()
        log.Printf("croupier-core (grpc) listening on %s", *addr)
        if err := s.Serve(lis); err != nil {
            log.Fatalf("serve grpc: %v", err)
        }
    }()
    go func() {
        defer wg.Done()
        aw, err := auditchain.NewWriter("logs/audit.log")
        if err != nil { log.Fatalf("audit: %v", err) }
        defer aw.Close()
        var pol *rbac.Policy
        if p, err := rbac.LoadPolicy(*rbacPath); err == nil { pol = p } else { pol = rbac.NewPolicy(); pol.Grant("user:dev", "*"); pol.Grant("user:dev", "job:cancel") }
        httpSrv, err := httpserver.NewServer("descriptors", functionserver.NewClientAdapter(fnsrv), aw, pol, gstore)
        if err != nil { log.Fatalf("http server: %v", err) }
        if err := httpSrv.ListenAndServe(*httpAddr); err != nil {
            log.Fatalf("serve http: %v", err)
        }
    }()
    wg.Wait()
}
