// Package transport provides TCP transport layer for Croupier SDK.
package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// InboundHandler handles an Agent request received over an established
// Provider connection and returns the protobuf response body.
type InboundHandler func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error)

// Config holds the configuration for the transport layer.
type Config struct {
	// Address is the server address to connect to.
	// Can be a single address or comma-separated multiple addresses.
	// Examples: "127.0.0.1:19091" or "ipc://croupier-agent,127.0.0.1:19091"
	Address string

	// Addresses is a list of addresses to try (optional, takes precedence over Address)
	Addresses []string

	// IPCAddress is the IPC address to try first (optional)
	// If set, this will be tried before the TCP address
	IPCAddress string

	// ExternalAddress is the address to report to external services
	// Use this when the listening address differs from the external address
	// (e.g., behind NAT, proxy, or when using port 0 for auto-assign)
	ExternalAddress string

	// Host and Port for TCP transport (parsed from Address if not set)
	Host string
	Port int

	// TLS configuration
	Insecure   bool
	CAFile     string
	CertFile   string
	KeyFile    string
	ServerName string

	// Timeouts
	DialTimeout time.Duration
	RecvTimeout time.Duration
	SendTimeout time.Duration

	// Backpressure
	ReadQLen  int // receive queue length (default 128)
	WriteQLen int // send queue length (default 64)

	// InboundHandler enables bidirectional Provider sessions. When present,
	// requests sent by the Agent over this connection are handled in place.
	InboundHandler InboundHandler
}

// DefaultConfig returns a default configuration for the transport layer.
func DefaultConfig() *Config {
	return &Config{
		Address:     "127.0.0.1:19091",
		Host:        "127.0.0.1",
		Port:        19091,
		Insecure:    true,
		DialTimeout: 30 * time.Second,
		RecvTimeout: 30 * time.Second,
		SendTimeout: 5 * time.Second,
		ReadQLen:    128,
		WriteQLen:   64,
	}
}

// createTLSConfig creates a tls.Config from the given configuration.
func createTLSConfig(cfg *Config) (*tls.Config, error) {
	if cfg.Insecure {
		return nil, nil // insecure mode, no TLS config needed
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Load CA certificate
	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
		tlsConfig.RootCAs = caPool
	}

	// Load client certificate for mTLS
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Configure server name verification
	if cfg.ServerName != "" {
		tlsConfig.ServerName = cfg.ServerName
	}

	return tlsConfig, nil
}

// dialAddr returns the appropriate address string for dialing.
// Deprecated: Use buildDialAddrs instead for multi-address support.
func dialAddr(cfg *Config) string {
	addrs := buildDialAddrs(cfg)
	if len(addrs) > 0 {
		return addrs[0]
	}
	return "tcp://127.0.0.1:19091"
}

// buildDialAddrs builds a list of addresses to try for dialing.
// It prioritizes IPC for local connections.
func buildDialAddrs(cfg *Config) []string {
	var addrs []string

	// If Addresses is explicitly set, use it
	if len(cfg.Addresses) > 0 {
		for _, addr := range cfg.Addresses {
			if addr = strings.TrimSpace(addr); addr != "" {
				addrs = append(addrs, normalizeAddress(addr, cfg.Insecure))
			}
		}
		return addrs
	}

	// Try IPC address first if specified
	if cfg.IPCAddress != "" && isIPCSupported() {
		ipcAddr := strings.TrimSpace(cfg.IPCAddress)
		if !strings.HasPrefix(ipcAddr, "ipc://") {
			ipcAddr = "ipc://" + ipcAddr
		}
		addrs = append(addrs, ipcAddr)
	}

	// Parse main address for additional addresses
	if cfg.Address != "" {
		parts := strings.Split(cfg.Address, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				// Skip if it's the same as IPCAddress
				if cfg.IPCAddress != "" && part == cfg.IPCAddress {
					continue
				}
				addrs = append(addrs, normalizeAddress(part, cfg.Insecure))
			}
		}
	}

	// Default if no addresses specified
	if len(addrs) == 0 {
		addrs = []string{"tcp://127.0.0.1:19091"}
	}

	return addrs
}

// normalizeAddress ensures an address has the appropriate scheme prefix.
func normalizeAddress(addr string, insecure bool) string {
	// If already has a scheme, use as-is
	if strings.HasPrefix(addr, "inproc://") ||
		strings.HasPrefix(addr, "ipc://") ||
		strings.HasPrefix(addr, "tcp://") ||
		strings.HasPrefix(addr, "tls+tcp://") ||
		strings.HasPrefix(addr, "ws://") ||
		strings.HasPrefix(addr, "wss://") {
		return addr
	}

	// Add scheme based on security setting
	if !insecure {
		return "tls+tcp://" + addr
	}
	return "tcp://" + addr
}

// isIPCSupported checks if IPC transport is supported on this platform.
func isIPCSupported() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "freebsd"
}
