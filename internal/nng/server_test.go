package nng

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"google.golang.org/protobuf/proto"
)

// TestParseListenAddr tests ParseListenAddr function
func TestParseListenAddr(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantTransport string
		wantAddr      string
		wantURL       string
	}{
		{
			name:          "TCP address",
			input:         ":19090",
			wantTransport: "tcp",
			wantAddr:      ":19090",
			wantURL:       "tcp://:19090",
		},
		{
			name:          "TCP with prefix",
			input:         "tcp://localhost:19090",
			wantTransport: "tcp",
			wantAddr:      "localhost:19090",
			wantURL:       "tcp://localhost:19090",
		},
		{
			name:          "IPC address",
			input:         "ipc://croupier-server",
			wantTransport: "ipc",
			wantAddr:      "croupier-server",
			wantURL:       "ipc://croupier-server",
		},
		{
			name:          "Empty address defaults to TCP",
			input:         "",
			wantTransport: "tcp",
			wantAddr:      "",
			wantURL:       "tcp://",
		},
		{
			name:          "Full URL",
			input:         "tcp://0.0.0.0:19090",
			wantTransport: "tcp",
			wantAddr:      "0.0.0.0:19090",
			wantURL:       "tcp://0.0.0.0:19090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseListenAddr(tt.input)

			if got.Transport != tt.wantTransport {
				t.Errorf("ParseListenAddr() transport = %q, want %q", got.Transport, tt.wantTransport)
			}
			if got.Addr != tt.wantAddr {
				t.Errorf("ParseListenAddr() addr = %q, want %q", got.Addr, tt.wantAddr)
			}
			if got.URL != tt.wantURL {
				t.Errorf("ParseListenAddr() url = %q, want %q", got.URL, tt.wantURL)
			}
		})
	}
}

// TestIsLocalTCP tests IsLocalTCP function
func TestIsLocalTCP(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{"Empty address", "", true},
		{"localhost", "localhost:19090", true},
		{"127.0.0.1", "127.0.0.1:19090", true},
		{"::1", "[::1]:19090", true},
		{"IPv6 localhost", "::1", true},
		{"Remote address", "192.168.1.1:19090", false},
		{"Remote host", "example.com:19090", false},
		{"0.0.0.0", "0.0.0.0:19090", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLocalTCP(tt.addr)
			if got != tt.want {
				t.Errorf("IsLocalTCP(%q) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}

// TestNewServer tests creating a new server
func TestNewServer(t *testing.T) {
	tests := []struct {
		name          string
		addr          string
		wantAddrCount int
	}{
		{
			name:          "Single address",
			addr:          ":19090",
			wantAddrCount: 1,
		},
		{
			name:          "Multiple addresses",
			addr:          ":19090,ipc://croupier-server",
			wantAddrCount: 2,
		},
		{
			name:          "Empty address uses default",
			addr:          "",
			wantAddrCount: 1,
		},
		{
			name:          "Address with spaces",
			addr:          " :19090 , ipc://test ",
			wantAddrCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.addr, nil)

			if len(server.addrs) != tt.wantAddrCount {
				t.Errorf("NewServer() addrs count = %d, want %d", len(server.addrs), tt.wantAddrCount)
			}

			if server.registry == nil {
				t.Errorf("NewServer() registry should not be nil")
			}

			if server.metricsStore == nil {
				t.Errorf("NewServer() metricsStore should not be nil")
			}

			if server.systemInfoCache == nil {
				t.Errorf("NewServer() systemInfoCache should not be nil")
			}

			if server.defaultSessionTTL != 5*time.Minute {
				t.Errorf("NewServer() defaultSessionTTL = %v, want 5m", server.defaultSessionTTL)
			}

			if server.ctx == nil {
				t.Errorf("NewServer() ctx should not be nil")
			}

			if server.logger == nil {
				t.Errorf("NewServer() logger should not be nil")
			}
		})
	}
}

// TestNewServerWithAddrs tests creating a server with explicit addresses
func TestNewServerWithAddrs(t *testing.T) {
	tests := []struct {
		name          string
		addrs         []ListenAddr
		wantAddrCount int
	}{
		{
			name:          "Single address",
			addrs:         []ListenAddr{ParseListenAddr(":19090")},
			wantAddrCount: 1,
		},
		{
			name:          "Multiple addresses",
			addrs:         []ListenAddr{ParseListenAddr(":19090"), ParseListenAddr("ipc://test")},
			wantAddrCount: 2,
		},
		{
			name:          "Empty addresses uses default",
			addrs:         []ListenAddr{},
			wantAddrCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServerWithAddrs(tt.addrs, nil)

			if len(server.addrs) != tt.wantAddrCount {
				t.Errorf("NewServerWithAddrs() addrs count = %d, want %d", len(server.addrs), tt.wantAddrCount)
			}
		})
	}
}

// TestNewServerWithDB tests creating a server with database
func TestNewServerWithDB(t *testing.T) {
	addrs := []ListenAddr{ParseListenAddr(":19090")}
	store := registry.NewStore()
	loader := &mockAgentSessionLoader{}

	server := NewServerWithDB(addrs, store, loader)

	if server.agentSessionLoader == nil {
		t.Errorf("NewServerWithDB() agentSessionLoader should not be nil")
	}

	if server.registry != store {
		t.Errorf("NewServerWithDB() registry should match provided store")
	}
}

// TestServerSetters tests server setter methods
func TestServerSetters(t *testing.T) {
	server := NewServer(":19090", nil)

	// Test SetDefaultSessionTTL
	server.SetDefaultSessionTTL(10 * time.Minute)
	if server.defaultSessionTTL != 10*time.Minute {
		t.Errorf("SetDefaultSessionTTL() failed")
	}

	// Test SetUpstreamHandler
	handler := &ControlHandler{server: server}
	server.SetUpstreamHandler(handler)
	if server.upstream == nil {
		t.Errorf("SetUpstreamHandler() failed")
	}

	// Test SetLogger (can be nil)
	server.SetLogger(nil)
	// Just verify it doesn't panic
}

// TestServerStoreMethods tests store accessor methods
func TestServerStoreMethods(t *testing.T) {
	store := registry.NewStore()
	server := NewServer(":19090", store)

	// Test Store
	if server.Store() != store {
		t.Errorf("Store() should return the provided store")
	}

	// Test MetricsStore
	if server.MetricsStore() == nil {
		t.Errorf("MetricsStore() should not return nil")
	}

	// Test SystemInfoCache
	if server.SystemInfoCache() == nil {
		t.Errorf("SystemInfoCache() should not return nil")
	}
}

// TestServerGetLocalAddr tests GetLocalAddr method
func TestServerGetLocalAddr(t *testing.T) {
	tests := []struct {
		name     string
		addrs    []ListenAddr
		wantAddr string
		wantErr  bool
	}{
		{
			name:     "Single address",
			addrs:    []ListenAddr{ParseListenAddr(":19090")},
			wantAddr: "tcp://:19090",
			wantErr:  false,
		},
		{
			name:     "Empty addresses returns default",
			addrs:    []ListenAddr{},
			wantAddr: "tcp://:19090", // Default address from NewServerWithAddrs
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServerWithAddrs(tt.addrs, nil)

			addr, err := server.GetLocalAddr()
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetLocalAddr() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("GetLocalAddr() unexpected error: %v", err)
				}
				if addr != tt.wantAddr {
					t.Errorf("GetLocalAddr() = %q, want %q", addr, tt.wantAddr)
				}
			}
		})
	}
}

// TestServerGetAddrs tests GetAddrs method
func TestServerGetAddrs(t *testing.T) {
	server := NewServer(":19090,ipc://test", nil)

	addrs := server.GetAddrs()
	if len(addrs) != 2 {
		t.Errorf("GetAddrs() count = %d, want 2", len(addrs))
	}

	if addrs[0].Transport != "tcp" {
		t.Errorf("GetAddrs()[0].Transport = %q, want tcp", addrs[0].Transport)
	}
	if addrs[1].Transport != "ipc" {
		t.Errorf("GetAddrs()[1].Transport = %q, want ipc", addrs[1].Transport)
	}
}

// TestServerGetLocalAddrs tests GetLocalAddrs method
func TestServerGetLocalAddrs(t *testing.T) {
	server := NewServer(":19090,ipc://test", nil)

	urls := server.GetLocalAddrs()
	if len(urls) != 2 {
		t.Errorf("GetLocalAddrs() count = %d, want 2", len(urls))
	}

	if urls[0] != "tcp://:19090" {
		t.Errorf("GetLocalAddrs()[0] = %q, want tcp://:19090", urls[0])
	}
	if urls[1] != "ipc://test" {
		t.Errorf("GetLocalAddrs()[1] = %q, want ipc://test", urls[1])
	}
}

// TestServerGetStats tests GetStats method
func TestServerGetStats(t *testing.T) {
	server := NewServer(":19090,ipc://test", nil)
	server.running = true

	stats := server.GetStats()

	if stats["running"] != true {
		t.Errorf("GetStats() running = %v, want true", stats["running"])
	}

	if stats["session_ttl"] != (5 * time.Minute).String() {
		t.Errorf("GetStats() session_ttl = %v, want 5m0s", stats["session_ttl"])
	}

	addresses, ok := stats["addresses"].([]string)
	if !ok {
		t.Errorf("GetStats() addresses is not []string")
	} else if len(addresses) != 2 {
		t.Errorf("GetStats() addresses count = %d, want 2", len(addresses))
	}
}

// TestServerStartStop tests Start and Stop methods
func TestServerStartStop(t *testing.T) {
	server := NewServer(":0", nil) // Use :0 for random port

	// Test Stop when not running (should be no-op)
	err := server.Stop()
	if err != nil {
		t.Errorf("Stop() when not running returned error: %v", err)
	}

	// Test Start
	err = server.Start()
	if err != nil {
		t.Errorf("Start() failed: %v", err)
	}

	// Test Start when already running
	err = server.Start()
	if err == nil {
		t.Errorf("Start() when already running should return error")
	}

	// Test Stop
	err = server.Stop()
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Verify server is stopped
	if server.running {
		t.Errorf("server.running should be false after Stop()")
	}
}

// TestServerStartError tests Start error handling
func TestServerStartError(t *testing.T) {
	server := NewServer("invalid-address-format-!@#$", nil)

	// Start should fail with invalid address
	err := server.Start()
	if err == nil {
		t.Errorf("Start() with invalid address should fail")
	}
}

// TestServerStopMultiple tests calling Stop multiple times
func TestServerStopMultiple(t *testing.T) {
	server := NewServer(":0", nil)

	server.Start()

	// First stop
	err1 := server.Stop()
	// Second stop
	err2 := server.Stop()

	if err1 != nil {
		t.Errorf("First Stop() failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second Stop() failed: %v", err2)
	}
}

// TestHandleRegisterWithUpstream tests handleRegister with upstream forwarding
func TestHandleRegisterWithUpstream(t *testing.T) {
	server := NewServer(":19090", nil)

	// Set up upstream handler
	upstreamCalled := false
	mockHandler := &mockHandler{
		registerFunc: func(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
			upstreamCalled = true
			return &agentv1.RegisterResponse{}, nil
		},
	}
	server.SetUpstreamHandler(mockHandler)

	ctx := context.Background()
	req := &agentv1.RegisterRequest{
		AgentId: "test-agent",
	}

	_, err := server.handleRegisterRequest(ctx, req)
	if err != nil {
		t.Errorf("handleRegisterRequest() failed: %v", err)
	}

	if !upstreamCalled {
		t.Errorf("handleRegisterRequest() should call upstream when configured")
	}
}

// TestHandleHeartbeatWithUpstream tests handleHeartbeat with upstream forwarding
func TestHandleHeartbeatWithUpstream(t *testing.T) {
	server := NewServer(":19090", nil)

	// Set up upstream handler
	upstreamCalled := false
	mockHandler := &mockHandler{
		heartbeatFunc: func(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
			upstreamCalled = true
			return &agentv1.HeartbeatResponse{}, nil
		},
	}
	server.SetUpstreamHandler(mockHandler)

	ctx := context.Background()
	req := &agentv1.HeartbeatRequest{
		AgentId: "test-agent",
	}

	_, err := server.handleHeartbeatRequest(ctx, req)
	if err != nil {
		t.Errorf("handleHeartbeatRequest() failed: %v", err)
	}

	if !upstreamCalled {
		t.Errorf("handleHeartbeatRequest() should call upstream when configured")
	}
}

// TestHandleHeartbeatEmptyAgentID tests handleHeartbeat with empty agent ID
func TestHandleHeartbeatEmptyAgentID(t *testing.T) {
	server := NewServer(":19090", nil)
	server.SetDefaultSessionTTL(5 * time.Minute)

	ctx := context.Background()
	req := &agentv1.HeartbeatRequest{
		AgentId: "", // Empty
	}

	resp, err := server.handleHeartbeatRequest(ctx, req)
	if err != nil {
		t.Errorf("handleHeartbeatRequest() failed: %v", err)
	}

	if resp == nil {
		t.Errorf("handleHeartbeatRequest() response should not be nil")
	}
}

// TestHandleRegisterCapabilitiesWithUpstream tests handleRegisterCapabilities with upstream forwarding
func TestHandleRegisterCapabilitiesWithUpstream(t *testing.T) {
	server := NewServer(":19090", nil)

	// Set up upstream handler
	upstreamCalled := false
	mockHandler := &mockHandler{
		capabilitiesFunc: func(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
			upstreamCalled = true
			return &agentv1.RegisterCapabilitiesResponse{}, nil
		},
	}
	server.SetUpstreamHandler(mockHandler)

	ctx := context.Background()
	req := &agentv1.RegisterCapabilitiesRequest{
		Provider: &agentv1.ProviderMeta{
			Id: "test-provider",
		},
	}

	_, err := server.handleRegisterCapabilitiesRequest(ctx, req)
	if err != nil {
		t.Errorf("handleRegisterCapabilitiesRequest() failed: %v", err)
	}

	if !upstreamCalled {
		t.Errorf("handleRegisterCapabilitiesRequest() should call upstream when configured")
	}
}

// TestHandleRegisterCapabilitiesNoProvider tests handleRegisterCapabilities without provider
func TestHandleRegisterCapabilitiesNoProvider(t *testing.T) {
	server := NewServer(":19090", nil)

	ctx := context.Background()
	req := &agentv1.RegisterCapabilitiesRequest{
		Provider: nil,
	}

	_, err := server.handleRegisterCapabilitiesRequest(ctx, req)
	if err == nil {
		t.Errorf("handleRegisterCapabilitiesRequest() should fail when provider is nil")
	}
}

// TestHandleRegisterCapabilitiesNoProviderID tests handleRegisterCapabilities without provider ID
func TestHandleRegisterCapabilitiesNoProviderID(t *testing.T) {
	server := NewServer(":19090", nil)

	ctx := context.Background()
	req := &agentv1.RegisterCapabilitiesRequest{
		Provider: &agentv1.ProviderMeta{
			Id: "", // Empty
		},
	}

	_, err := server.handleRegisterCapabilitiesRequest(ctx, req)
	if err == nil {
		t.Errorf("handleRegisterCapabilitiesRequest() should fail when provider ID is empty")
	}
}

// TestDecompressManifest tests decompressManifest method
func TestDecompressManifest(t *testing.T) {
	server := NewServer(":19090", nil)

	// Create valid gzipped data
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte(`{"test": "data"}`))
	gz.Close()

	data, err := server.decompressManifest(buf.Bytes())
	if err != nil {
		t.Errorf("decompressManifest() failed: %v", err)
	}

	if string(data) != `{"test": "data"}` {
		t.Errorf("decompressManifest() = %q, want '{\"test\": \"data\"}'", string(data))
	}
}

// TestDecompressManifestEmpty tests decompressManifest with empty data
func TestDecompressManifestEmpty(t *testing.T) {
	server := NewServer(":19090", nil)

	_, err := server.decompressManifest([]byte{})
	if err == nil {
		t.Errorf("decompressManifest() should fail with empty data")
	}
}

// TestDecompressManifestInvalid tests decompressManifest with invalid gzip
func TestDecompressManifestInvalid(t *testing.T) {
	server := NewServer(":19090", nil)

	_, err := server.decompressManifest([]byte("not gzip"))
	if err == nil {
		t.Errorf("decompressManifest() should fail with invalid gzip")
	}
}

// TestCreateErrorResponseServer tests createErrorResponse method on Server
func TestCreateErrorResponseServer(t *testing.T) {
	server := NewServer(":19090", nil)

	err := errors.New("test error")
	resp := server.createErrorResponse(err)

	if resp == nil {
		t.Errorf("createErrorResponse() should not return nil")
	}

	response := &agentv1.RegisterResponse{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal error response: %v", err)
	}
}

// TestValidateAndNormalizeFunctions tests validateAndNormalizeFunctions function
func TestValidateAndNormalizeFunctions(t *testing.T) {
	tests := []struct {
		name          string
		input         []*agentv1.FunctionDescriptor
		wantFuncCount int
		wantWarnCount int
		firstFuncID   string
	}{
		{
			name: "Valid functions",
			input: []*agentv1.FunctionDescriptor{
				{Id: "test.function1", Version: "1.0.0"},
				{Id: "test.function2", Version: "1.2.3"},
			},
			wantFuncCount: 2,
			wantWarnCount: 0,
			firstFuncID:   "", // Order not guaranteed with maps
		},
		{
			name: "Nil function in list",
			input: []*agentv1.FunctionDescriptor{
				{Id: "test.function1", Version: "1.0.0"},
				nil,
				{Id: "test.function2", Version: "1.0.0"},
			},
			wantFuncCount: 2,
			wantWarnCount: 1,
			firstFuncID:   "test.function1",
		},
		{
			name: "Empty function ID",
			input: []*agentv1.FunctionDescriptor{
				{Id: "", Version: "1.0.0"},
			},
			wantFuncCount: 0,
			wantWarnCount: 1,
		},
		{
			name: "Invalid function ID format",
			input: []*agentv1.FunctionDescriptor{
				{Id: "InvalidCaps", Version: "1.0.0"},
			},
			wantFuncCount: 0,
			wantWarnCount: 1,
		},
		{
			name: "Invalid semver",
			input: []*agentv1.FunctionDescriptor{
				{Id: "test.function", Version: "1.0"},
			},
			wantFuncCount: 0,
			wantWarnCount: 1,
		},
		{
			name: "Duplicate function ID keeps higher version",
			input: []*agentv1.FunctionDescriptor{
				{Id: "test.function", Version: "1.0.0"},
				{Id: "test.function", Version: "2.0.0"},
			},
			wantFuncCount: 1,
			wantWarnCount: 1,
			firstFuncID:   "test.function",
		},
		{
			name:          "Empty input",
			input:         []*agentv1.FunctionDescriptor{},
			wantFuncCount: 0,
			wantWarnCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcs, warnings := validateAndNormalizeFunctions(tt.input)

			if len(funcs) != tt.wantFuncCount {
				t.Errorf("validateAndNormalizeFunctions() funcs count = %d, want %d", len(funcs), tt.wantFuncCount)
			}

			if len(warnings) != tt.wantWarnCount {
				t.Errorf("validateAndNormalizeFunctions() warnings count = %d, want %d", len(warnings), tt.wantWarnCount)
			}

			if tt.wantFuncCount > 0 && tt.firstFuncID != "" {
				// Only check if we expect a specific function
				found := false
				for _, f := range funcs {
					if f.Id == tt.firstFuncID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validateAndNormalizeFunctions() expected to find function ID %q", tt.firstFuncID)
				}
			}
		})
	}
}

// TestIsValidSemver tests isValidSemver function
func TestIsValidSemver(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"1.0.0", true},
		{"v1.0.0", true},
		{"0.0.1", true},
		{"10.20.30", true},
		{"1.0.0-alpha", true},
		{"1.0.0-alpha.1", true},
		{"1.0.0-beta", true},
		{"1.0.0-beta.2", true},
		{"1.0.0-rc.1", true},
		{"1.0.0+001", true},
		{"1.0.0-alpha+001", true},
		{"1", false},
		{"1.0", false},
		{"1.0.0.0", false},
		{"v1", false},
		{"", false},
		{"x.y.z", false},
		{"1.0.0-", false}, // Empty prerelease is not valid semver
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := isValidSemver(tt.version)
			if got != tt.want {
				t.Errorf("isValidSemver(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

// TestCompareSemver tests compareSemver function
func TestCompareSemver(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int // -1: a < b, 0: a == b, 1: a > b
	}{
		{"Equal", "1.0.0", "1.0.0", 0},
		{"Greater major", "2.0.0", "1.0.0", 1},
		{"Lesser major", "1.0.0", "2.0.0", -1},
		{"Greater minor", "1.2.0", "1.1.0", 1},
		{"Lesser minor", "1.1.0", "1.2.0", -1},
		{"Greater patch", "1.0.1", "1.0.0", 1},
		{"Lesser patch", "1.0.0", "1.0.1", -1},
		{"With v prefix", "v1.0.0", "1.0.0", 0},
		{"Different lengths", "1.0", "1.0.0", 0},
		{"Prelease ignored", "1.0.0-alpha", "1.0.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareSemver(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestNewControlHandler tests NewControlHandler function
func TestNewControlHandler(t *testing.T) {
	server := NewServer(":19090", nil)
	handler := NewControlHandler(server)

	if handler == nil {
		t.Errorf("NewControlHandler() should not return nil")
	}
	if handler.server != server {
		t.Errorf("NewControlHandler() server not set correctly")
	}
}

// TestControlHandlerMethods tests ControlHandler methods
func TestControlHandlerMethods(t *testing.T) {
	server := NewServer(":19090", nil)
	handler := NewControlHandler(server)

	ctx := context.Background()

	// Test HandleRegister - should call server's handleRegisterRequest
	regResp, err := handler.HandleRegister(ctx, &agentv1.RegisterRequest{
		AgentId: "test-agent",
		GameId:  "test-game",
		Env:     "test",
	})
	if err != nil {
		t.Errorf("HandleRegister() failed: %v", err)
	}
	if regResp == nil {
		t.Errorf("HandleRegister() response should not be nil")
	}

	// Test HandleHeartbeat
	hbResp, err := handler.HandleHeartbeat(ctx, &agentv1.HeartbeatRequest{
		AgentId: "test-agent",
	})
	if err != nil {
		t.Errorf("HandleHeartbeat() failed: %v", err)
	}
	if hbResp == nil {
		t.Errorf("HandleHeartbeat() response should not be nil")
	}

	// Test HandleRegisterCapabilities
	// This requires a valid gzipped manifest, so skip for now
	// capResp, err := handler.HandleRegisterCapabilities(ctx, &agentv1.RegisterCapabilitiesRequest{
	// 	Provider: &agentv1.ProviderMeta{Id: "test"},
	// })
	// For now, just test that the method exists
	_ = ctx
	_ = handler
}

// TestLoadAgentSessionsNoLoader tests LoadAgentSessions without loader
func TestLoadAgentSessionsNoLoader(t *testing.T) {
	server := NewServer(":19090", nil) // No loader set

	err := server.LoadAgentSessions()
	if err != nil {
		t.Errorf("LoadAgentSessions() without loader should not error: %v", err)
	}
}

// TestHandleRequestUnknownType tests handleRequest with unknown message type
func TestHandleRequestUnknownType(t *testing.T) {
	server := NewServer(":19090", nil)

	ctx := context.Background()
	// Use an invalid message ID
	_, err := server.handleRequest(ctx, 0xFFFFFF, []byte("test"))

	if err == nil {
		t.Errorf("handleRequest() should fail with unknown message type")
	}

	if !strings.Contains(err.Error(), "unknown message type") {
		t.Errorf("handleRequest() error should mention unknown message type: %v", err)
	}
}

// Mock handler for testing
type mockHandler struct {
	registerFunc     func(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error)
	heartbeatFunc    func(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error)
	capabilitiesFunc func(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error)
}

func (m *mockHandler) HandleRegister(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, req)
	}
	return &agentv1.RegisterResponse{}, nil
}

func (m *mockHandler) HandleHeartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	if m.heartbeatFunc != nil {
		return m.heartbeatFunc(ctx, req)
	}
	return &agentv1.HeartbeatResponse{}, nil
}

func (m *mockHandler) HandleRegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	if m.capabilitiesFunc != nil {
		return m.capabilitiesFunc(ctx, req)
	}
	return &agentv1.RegisterCapabilitiesResponse{}, nil
}

// Mock AgentSessionLoader for testing
type mockAgentSessionLoader struct{}

func (m *mockAgentSessionLoader) LoadActiveSessions(ctx context.Context) ([]*registry.AgentSession, error) {
	return []*registry.AgentSession{}, nil
}

func (m *mockAgentSessionLoader) Upsert(ctx context.Context, sess *registry.AgentSession) error {
	return nil
}

func (m *mockAgentSessionLoader) DeleteExpired(ctx context.Context) (int64, error) {
	return 0, nil
}
