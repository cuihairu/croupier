// Package nng provides multi-transport support utilities
package nng

import (
	"fmt"
	"runtime"
	"strings"
)

// BuildServerAddrs constructs a list of listen addresses from configuration
// It combines the primary TCP address with optional IPC address
func BuildServerAddrs(primaryAddr string, ipcAddr string) []ListenAddr {
	var addrs []ListenAddr

	// Add primary address
	if primaryAddr != "" {
		addrs = append(addrs, ParseListenAddr(primaryAddr))
	}

	// Add IPC address if specified
	if ipcAddr != "" {
		// Ensure it has ipc:// prefix
		if !strings.HasPrefix(ipcAddr, "ipc://") {
			ipcAddr = "ipc://" + ipcAddr
		}
		addrs = append(addrs, ParseListenAddr(ipcAddr))
	}

	return addrs
}

// BuildClientAddrs constructs a list of addresses for client connection
// It prioritizes IPC for local connections, falling back to TCP
func BuildClientAddrs(serverAddr string, ipcAddr string) []string {
	var addrs []string

	// If IPC address is specified and we're on a supported platform, try it first
	if ipcAddr != "" && isIPCSupported() {
		// Ensure it has ipc:// prefix
		if !strings.HasPrefix(ipcAddr, "ipc://") {
			ipcAddr = "ipc://" + ipcAddr
		}
		addrs = append(addrs, ipcAddr)
	}

	// Add TCP address as fallback
	if serverAddr != "" {
		if !strings.HasPrefix(serverAddr, "tcp://") {
			serverAddr = "tcp://" + serverAddr
		}
		addrs = append(addrs, serverAddr)
	}

	return addrs
}

// FormatListenURL formats a listen address for NNG
func FormatListenURL(addr string, transport string) string {
	if strings.Contains(addr, "://") {
		return addr
	}
	if transport == "" {
		transport = "tcp"
	}
	return fmt.Sprintf("%s://%s", transport, addr)
}

// isIPCSupported checks if IPC transport is supported on this platform
func isIPCSupported() bool {
	// IPC is supported on Windows (Named Pipes) and Unix-like systems (Unix Domain Socket)
	// The mangos IPC transport works on both
	return runtime.GOOS == "windows" || runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "freebsd"
}

// DefaultIPCAddr returns the default IPC address for the service
func DefaultIPCAddr(serviceName string) string {
	if serviceName == "" {
		serviceName = "croupier"
	}
	return fmt.Sprintf("ipc://%s", serviceName)
}

// AgentIPCAddr returns the default IPC address for the agent
func AgentIPCAddr() string {
	return DefaultIPCAddr("croupier-agent")
}

// ServerIPCAddr returns the default IPC address for the server
func ServerIPCAddr() string {
	return DefaultIPCAddr("croupier-server")
}
