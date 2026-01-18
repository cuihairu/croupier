package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cuihairu/croupier/internal/devcert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

// ServerTLSConfig holds server TLS configuration.
type ServerTLSConfig struct {
	CertFile  string
	KeyFile   string
	CAFile    string // If non-empty, enables mTLS
	AutoGen   bool   // If true, auto-generate certs when CertFile/KeyFile are empty
	ConfigDir string // Directory containing config file (for auto-generated cert location)
}

// EnsureServerTLSCredentials creates TransportCredentials for a gRPC server.
//   - If CertFile/KeyFile are provided, they are used.
//   - If AutoGen is true and CertFile/KeyFile are empty, dev certs are auto-generated
//     to <ConfigDir>/certs/ (e.g., "etc/certs/" when config is "etc/server.yaml").
//   - If CAFile is provided, mTLS is enabled.
func EnsureServerTLSCredentials(cfg ServerTLSConfig) (grpc.ServerOption, error) {
	var certFile, keyFile, caFile string

	// Use provided certs if available
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		certFile = cfg.CertFile
		keyFile = cfg.KeyFile
		caFile = cfg.CAFile
	} else if cfg.AutoGen {
		// Auto-generate dev certs
		certDir := filepath.Join(filepath.Dir(cfg.ConfigDir), "certs")
		if certDir == "." || certDir == "/certs" {
			certDir = "etc/certs"
		}
		fmt.Printf("No TLS certificate configured, auto-generating dev certs in %s...\n", certDir)

		caCrt, caKey, err := devcert.EnsureDevCA(certDir)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure dev CA: %w", err)
		}
		certFile, keyFile, err = devcert.EnsureServerCert(certDir, caCrt, caKey, []string{"localhost", "127.0.0.1"})
		if err != nil {
			return nil, fmt.Errorf("failed to ensure server cert: %w", err)
		}

		fmt.Printf("  CA: %s\n", caCrt)
		fmt.Printf("  Cert: %s\n", certFile)
	} else {
		// No TLS
		return nil, nil
	}

	requireClient := caFile != ""
	creds, err := ServerTLS(certFile, keyFile, caFile, requireClient)
	if err != nil {
		return nil, err
	}

	opt := grpc.Creds(creds)
	if requireClient {
		fmt.Printf("gRPC server with mTLS enabled\n")
	} else {
		fmt.Printf("gRPC server with TLS enabled\n")
	}
	return opt, nil
}
