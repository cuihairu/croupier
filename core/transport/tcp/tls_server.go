package tcp

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
)

// ServerTLSConfig builds the server-side *tls.Config for a TLS listener
// from the shared Config fields: MinVersion 1.2, CAFile enables mTLS
// (RequireAndVerifyClientCert), insecureSkipVerify downgrades to
// NoClientCert. Unlike BuildServerTLSConfig a certificate is OPTIONAL
// here (mirrors Server's listen path, where the "enabled" gate lives in
// the caller).
func ServerTLSConfig(config *Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if config.VerifyConnection != nil {
		tlsConfig.VerifyConnection = config.VerifyConnection
	}

	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load server certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if config.CAFile != "" {
		pemBytes, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, fmt.Errorf("append CA certificate")
		}
		tlsConfig.ClientCAs = pool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if config.InsecureSkipVerify {
		tlsConfig.ClientAuth = tls.NoClientCert
	}

	return tlsConfig, nil
}

// BuildServerTLSConfig builds a server-side *tls.Config from plain file
// settings with the exact semantics of Server's TLS listen path. Exported for
// the Agent local gateway (tcp_local_listener) so both TLS surfaces share one
// implementation: MinVersion 1.2, CAFile enables mTLS
// (RequireAndVerifyClientCert), insecureSkipVerify downgrades to NoClientCert.
//
// Unlike Server's listen path, a certificate is REQUIRED here: callers gate on
// an explicit "enabled" flag, and a cert-less TLS listener would accept TCP
// connections only to fail every handshake.
func BuildServerTLSConfig(certFile, keyFile, caFile string, insecureSkipVerify bool) (*tls.Config, error) {
	if strings.TrimSpace(certFile) == "" || strings.TrimSpace(keyFile) == "" {
		return nil, fmt.Errorf("tls enabled but certFile/keyFile missing: provide both to secure the listener")
	}
	return ServerTLSConfig(&Config{
		CertFile:           certFile,
		KeyFile:            keyFile,
		CAFile:             caFile,
		InsecureSkipVerify: insecureSkipVerify,
	})
}
