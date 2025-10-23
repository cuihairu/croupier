package main

import (
    "crypto/tls"
    "crypto/x509"
    "flag"
    "io/ioutil"
    "log"
    "net"
    "sync"
    "fmt"

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
    addr := flag.String("addr", ":8443", "grpc listen address")
    httpAddr := flag.String("http_addr", ":8080", "http api listen address")
    edgeAddr := flag.String("edge_addr", "", "optional edge address for forwarding function calls (DEV PoC)")
    rbacPath := flag.String("rbac_config", "configs/rbac.json", "rbac policy json path")
    cert := flag.String("cert", "", "server cert file")
    key := flag.String("key", "", "server key file")
    ca := flag.String("ca", "", "ca cert file for client cert verification (optional)")
    gamesPath := flag.String("games_config", "configs/games.json", "allowed games config (json)")
    usersPath := flag.String("users_config", "configs/users.json", "users config json")
    jwtSecret := flag.String("jwt_secret", "dev-secret", "jwt hs256 secret")
    flag.Parse()

    // Auto-generate dev certs when not provided (DEV ONLY)
    if *cert == "" || *key == "" || *ca == "" {
        out := "configs/dev"
        caCrt, caKey, err := devcert.EnsureDevCA(out)
        if err != nil { log.Fatalf("generate dev CA: %v", err) }
        // include common localhost hosts for dev
        srvCrt, srvKey, err := devcert.EnsureServerCert(out, caCrt, caKey, []string{"localhost", "127.0.0.1"})
        if err != nil { log.Fatalf("generate dev server cert: %v", err) }
        // set flags to generated paths
        *cert, *key, *ca = srvCrt, srvKey, caCrt
        log.Printf("[devcert] generated dev TLS certs under %s (DEV ONLY)", out)
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
    var invoker httpserver.FunctionInvoker
    if *edgeAddr != "" {
        // Forward all FunctionService calls to Edge
        fwd := functionserver.NewForwarder(*edgeAddr)
        functionv1.RegisterFunctionServiceServer(s, fwd)
        invoker = functionserver.NewForwarderInvoker(fwd)
    } else {
        // Use default function server config when running in-core
        fnsrv := functionserver.NewServer(ctrl.Store(), nil)
        functionv1.RegisterFunctionServiceServer(s, fnsrv)
        invoker = functionserver.NewClientAdapter(fnsrv)
    }

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
        if p, err := rbac.LoadPolicy(*rbacPath); err == nil { pol = p } else { pol = rbac.NewPolicy(); pol.Grant("user:dev", "*"); pol.Grant("user:dev", "job:cancel"); pol.Grant("role:admin", "*") }
        var us *users.Store
        if s, err := users.Load(*usersPath); err == nil { us = s } else { log.Printf("users load failed: %v", err) }
        jm := jwt.NewManager(*jwtSecret)
        httpSrv, err := httpserver.NewServer("descriptors", invoker, aw, pol, gstore, ctrl.Store(), us, jm)
        if err != nil { log.Fatalf("http server: %v", err) }
        if err := httpSrv.ListenAndServe(*httpAddr); err != nil {
            log.Fatalf("serve http: %v", err)
        }
    }()
    wg.Wait()
}
