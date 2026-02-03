// Package nng provides NNG (Nanomsg Next Generation) transport layer for Croupier.
package nng

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// createTLSConfig creates TLS configuration from server config.
func createTLSConfig(cfg *Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Load CA certificate
	if cfg.CAFile != "" {
		pemBytes, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
		tlsConfig.RootCAs = pool
	} else {
		// Use system root CAs
		systemCAs, err := x509.SystemCertPool()
		if err == nil {
			tlsConfig.RootCAs = systemCAs
		}
	}

	// Load server certificate and key
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load server certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate for client certificate verification
	if cfg.CAFile != "" {
		pemBytes, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
		tlsConfig.ClientCAs = pool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	} else {
		// If no CA provided, don't require client certs but still use TLS
		tlsConfig.ClientAuth = tls.NoClientCert
	}

	// Configure whether to verify client certificates
	if cfg.InsecureSkipVerify {
		tlsConfig.ClientAuth = tls.NoClientCert
	}

	// Configure server name verification for client certs
	if cfg.ServerName != "" {
		tlsConfig.ServerName = cfg.ServerName
	}

	return tlsConfig, nil
}
