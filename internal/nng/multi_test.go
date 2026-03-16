package nng

import (
	"runtime"
	"strings"
	"testing"
)

// TestBuildServerAddrs tests building server addresses
func TestBuildServerAddrs(t *testing.T) {
	tests := []struct {
		name        string
		primaryAddr string
		ipcAddr     string
		wantCount   int
		wantTCP     bool
		wantIPC     bool
	}{
		{
			name:        "TCP only",
			primaryAddr: ":19090",
			ipcAddr:     "",
			wantCount:   1,
			wantTCP:     true,
			wantIPC:     false,
		},
		{
			name:        "IPC only",
			primaryAddr: "",
			ipcAddr:     "croupier-server",
			wantCount:   1,
			wantTCP:     false,
			wantIPC:     true,
		},
		{
			name:        "Both TCP and IPC",
			primaryAddr: ":19090",
			ipcAddr:     "croupier-server",
			wantCount:   2,
			wantTCP:     true,
			wantIPC:     true,
		},
		{
			name:        "Empty addresses",
			primaryAddr: "",
			ipcAddr:     "",
			wantCount:   0,
			wantTCP:     false,
			wantIPC:     false,
		},
		{
			name:        "IPC with prefix already",
			primaryAddr: ":19090",
			ipcAddr:     "ipc://custom-name",
			wantCount:   2,
			wantTCP:     true,
			wantIPC:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := BuildServerAddrs(tt.primaryAddr, tt.ipcAddr)

			if len(addrs) != tt.wantCount {
				t.Errorf("BuildServerAddrs() returned %d addresses, want %d", len(addrs), tt.wantCount)
			}

			hasTCP := false
			hasIPC := false
			for _, addr := range addrs {
				if addr.Transport == "tcp" {
					hasTCP = true
				}
				if addr.Transport == "ipc" {
					hasIPC = true
				}
			}

			if hasTCP != tt.wantTCP {
				t.Errorf("BuildServerAddrs() hasTCP=%v, want %v", hasTCP, tt.wantTCP)
			}
			if hasIPC != tt.wantIPC {
				t.Errorf("BuildServerAddrs() hasIPC=%v, want %v", hasIPC, tt.wantIPC)
			}
		})
	}
}

// TestBuildClientAddrs tests building client addresses
func TestBuildClientAddrs(t *testing.T) {
	tests := []struct {
		name       string
		serverAddr string
		ipcAddr    string
		wantCount  int
		firstIsIPC bool
	}{
		{
			name:       "TCP only",
			serverAddr: "localhost:19090",
			ipcAddr:    "",
			wantCount:  1,
			firstIsIPC: false,
		},
		{
			name:       "IPC and TCP",
			serverAddr: "localhost:19090",
			ipcAddr:    "croupier-server",
			wantCount:  2,
			firstIsIPC: isIPCSupported(),
		},
		{
			name:       "Empty addresses",
			serverAddr: "",
			ipcAddr:    "",
			wantCount:  0,
			firstIsIPC: false,
		},
		{
			name:       "TCP with prefix already",
			serverAddr: "tcp://localhost:19090",
			ipcAddr:    "",
			wantCount:  1,
			firstIsIPC: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := BuildClientAddrs(tt.serverAddr, tt.ipcAddr)

			if len(addrs) != tt.wantCount {
				t.Errorf("BuildClientAddrs() returned %d addresses, want %d", len(addrs), tt.wantCount)
			}

			if len(addrs) > 0 && tt.firstIsIPC {
				if !strings.HasPrefix(addrs[0], "ipc://") {
					t.Errorf("BuildClientAddrs() first address should be IPC, got %s", addrs[0])
				}
			}
		})
	}
}

// TestFormatListenURL tests formatting listen URLs
func TestFormatListenURL(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		transport string
		want      string
	}{
		{
			name:      "Simple address with TCP transport",
			addr:      ":19090",
			transport: "tcp",
			want:      "tcp://:19090",
		},
		{
			name:      "Simple address with IPC transport",
			addr:      "croupier-server",
			transport: "ipc",
			want:      "ipc://croupier-server",
		},
		{
			name:      "Address already has prefix",
			addr:      "tcp://localhost:19090",
			transport: "",
			want:      "tcp://localhost:19090",
		},
		{
			name:      "Empty transport defaults to TCP",
			addr:      ":19090",
			transport: "",
			want:      "tcp://:19090",
		},
		{
			name:      "IPC address already has prefix",
			addr:      "ipc://croupier-server",
			transport: "ipc",
			want:      "ipc://croupier-server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatListenURL(tt.addr, tt.transport)
			if got != tt.want {
				t.Errorf("FormatListenURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsIPCSupported tests IPC transport detection
func TestIsIPCSupported(t *testing.T) {
	// Test should return true for supported platforms
	supported := runtime.GOOS == "windows" || runtime.GOOS == "linux" ||
		runtime.GOOS == "darwin" || runtime.GOOS == "freebsd"

	if isIPCSupported() != supported {
		t.Errorf("isIPCSupported() = %v, expected %v for GOOS %s", isIPCSupported(), supported, runtime.GOOS)
	}
}

// TestDefaultIPCAddr tests default IPC address generation
func TestDefaultIPCAddr(t *testing.T) {
	tests := []struct {
		name       string
		service    string
		wantPrefix string
	}{
		{
			name:       "Custom service name",
			service:    "my-service",
			wantPrefix: "ipc://my-service",
		},
		{
			name:       "Empty service name uses default",
			service:    "",
			wantPrefix: "ipc://croupier",
		},
		{
			name:       "Service with spaces",
			service:    " test service ",
			wantPrefix: "ipc:// test service ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultIPCAddr(tt.service)
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("DefaultIPCAddr() = %v, want prefix %v", got, tt.wantPrefix)
			}
		})
	}
}

// TestAgentIPCAddr tests agent IPC address
func TestAgentIPCAddr(t *testing.T) {
	got := AgentIPCAddr()
	if got != "ipc://croupier-agent" {
		t.Errorf("AgentIPCAddr() = %v, want ipc://croupier-agent", got)
	}
}

// TestServerIPCAddr tests server IPC address
func TestServerIPCAddr(t *testing.T) {
	got := ServerIPCAddr()
	if got != "ipc://croupier-server" {
		t.Errorf("ServerIPCAddr() = %v, want ipc://croupier-server", got)
	}
}
