package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"google.golang.org/grpc/credentials"
	"os"
)

type ClientTLSConfig struct {
	CertFile           string
	KeyFile            string
	CAFile             string
	ServerName         string
	InsecureSkipVerify bool
}

// ClientTLSFromConfig builds TransportCredentials for a TLS client.
// - If CertFile/KeyFile are provided, client cert is loaded (mTLS).
// - If CAFile is provided, it is used as RootCAs; otherwise system roots are used.
// - ServerName is used for SNI/verification when set.
func ClientTLSFromConfig(cfg ClientTLSConfig) (credentials.TransportCredentials, error) {
	tlsCfg := &tls.Config{
		ServerName:         cfg.ServerName,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	if cfg.CertFile != "" || cfg.KeyFile != "" {
		if cfg.CertFile == "" || cfg.KeyFile == "" {
			return nil, fmt.Errorf("both cert_file and key_file are required when using client certificate")
		}
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load keypair: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("append ca: invalid pem")
		}
		tlsCfg.RootCAs = pool
	}

	return credentials.NewTLS(tlsCfg), nil
}

// ClientTLS builds TransportCredentials for a TLS client with mTLS and server name.
// certFile/keyFile: client certificate and key; caFile: CA cert for verifying server;
// serverName: SNI / verification name.
func ClientTLS(certFile, keyFile, caFile, serverName string) (credentials.TransportCredentials, error) {
	if certFile == "" || keyFile == "" || caFile == "" {
		return nil, fmt.Errorf("cert/key/ca are required")
	}
	return ClientTLSFromConfig(ClientTLSConfig{
		CertFile:   certFile,
		KeyFile:    keyFile,
		CAFile:     caFile,
		ServerName: serverName,
	})
}

// ServerTLS builds TransportCredentials for a TLS server. If requireClient is true,
// caFile must be provided and client cert verification is enforced.
func ServerTLS(certFile, keyFile, caFile string, requireClient bool) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load keypair: %w", err)
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	if requireClient {
		if caFile == "" {
			return nil, fmt.Errorf("ca certificate required for mTLS")
		}
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("append ca: invalid pem")
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return credentials.NewTLS(cfg), nil
}
