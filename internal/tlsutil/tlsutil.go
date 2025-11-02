package tlsutil

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "os"
    "google.golang.org/grpc/credentials"
)

// ClientTLS builds TransportCredentials for a TLS client with mTLS and server name.
// certFile/keyFile: client certificate and key; caFile: CA cert for verifying server;
// serverName: SNI / verification name.
func ClientTLS(certFile, keyFile, caFile, serverName string) (credentials.TransportCredentials, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil { return nil, fmt.Errorf("load keypair: %w", err) }
    caPEM, err := os.ReadFile(caFile)
    if err != nil { return nil, fmt.Errorf("read ca: %w", err) }
    pool := x509.NewCertPool()
    if !pool.AppendCertsFromPEM(caPEM) { return nil, fmt.Errorf("append ca: invalid pem") }
    cfg := &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: pool, ServerName: serverName}
    return credentials.NewTLS(cfg), nil
}

// ServerTLS builds TransportCredentials for a TLS server. If requireClient is true,
// caFile must be provided and client cert verification is enforced.
func ServerTLS(certFile, keyFile, caFile string, requireClient bool) (credentials.TransportCredentials, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil { return nil, fmt.Errorf("load keypair: %w", err) }
    cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
    if requireClient {
        if caFile == "" { return nil, fmt.Errorf("ca certificate required for mTLS") }
        caPEM, err := os.ReadFile(caFile)
        if err != nil { return nil, fmt.Errorf("read ca: %w", err) }
        pool := x509.NewCertPool()
        if !pool.AppendCertsFromPEM(caPEM) { return nil, fmt.Errorf("append ca: invalid pem") }
        cfg.ClientCAs = pool
        cfg.ClientAuth = tls.RequireAndVerifyClientCert
    }
    return credentials.NewTLS(cfg), nil
}

