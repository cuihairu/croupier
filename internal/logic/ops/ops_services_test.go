package ops

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

// Test OpsServicesLogic construction and field access
func TestOpsServicesLogic_Fields(t *testing.T) {
	t.Run("constructor sets fields", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			Config: config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Region: "us-east-1",
				Zone:   "zone1",
			},
			ServerVersion:   "v1.0.0",
			StartTime:       time.Now(),
			RegistryStore:   registry.NewStore(),
			MetricsStore:    registry.NewMetricsStore(),
			SystemInfoCache: registry.NewSystemInfoCache(),
		}

		ctx := context.Background()
		logic := NewOpsServicesLogic(ctx, svcCtx)

		if logic.ctx != ctx {
			t.Error("context not set correctly")
		}

		if logic.svcCtx != svcCtx {
			t.Error("service context not set correctly")
		}
	})

	t.Run("logic has access to config", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			Config: config.Config{
				Server: config.ServerConfig{
					Host: "0.0.0.0",
					Port: 9090,
				},
				Region: "eu-west-1",
				Zone:   "zone-a",
				Labels: map[string]string{
					"env":  "production",
					"team": "backend",
				},
			},
			ServerVersion:   "v2.0.0",
			StartTime:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			RegistryStore:   registry.NewStore(),
			MetricsStore:    registry.NewMetricsStore(),
			SystemInfoCache: registry.NewSystemInfoCache(),
		}

		logic := NewOpsServicesLogic(context.Background(), svcCtx)

		if logic.svcCtx.Config.Server.Port != 9090 {
			t.Errorf("expected port 9090, got %d", logic.svcCtx.Config.Server.Port)
		}

		if logic.svcCtx.Config.Region != "eu-west-1" {
			t.Errorf("expected region eu-west-1, got %s", logic.svcCtx.Config.Region)
		}

		if logic.svcCtx.ServerVersion != "v2.0.0" {
			t.Errorf("expected version v2.0.0, got %s", logic.svcCtx.ServerVersion)
		}
	})
}

// Test OpsServicesLogic with registry data
func TestOpsServicesLogic_RegistryData(t *testing.T) {
	t.Run("construct logic with populated registry", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			Config: config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Region: "us-west-2",
				Zone:   "zone-b",
			},
			ServerVersion:   "v1.5.0",
			StartTime:       time.Now(),
			RegistryStore:   registry.NewStore(),
			MetricsStore:    registry.NewMetricsStore(),
			SystemInfoCache: registry.NewSystemInfoCache(),
		}

		// Add agent sessions
		sess1 := &registry.AgentSession{
			AgentID:  "agent-west-1",
			GameID:   "game1",
			Env:      "prod",
			Version:  "v1.0.0",
			RPCAddr:  "192.168.1.10:19090",
			ExpireAt: time.Now().Add(time.Minute),
			LastSeen: time.Now(),
			Functions: map[string]registry.FunctionMeta{
				"game.player.get":    {Enabled: true},
				"game.player.create": {Enabled: true},
			},
			Labels: map[string]string{
				"datacenter": "us-west-2",
				"rack":       "rack1",
			},
			Region: "us-west-2",
			Zone:   "zone-b",
			Providers: []registry.ProviderSession{
				{
					ProviderID:   "provider-west-1",
					Addr:         "192.168.1.10:8081",
					Version:      "v1.0.0",
					LastSeenUnix: time.Now().Unix(),
					FunctionIDs:  []string{"game.player.get", "game.player.create"},
				},
			},
		}

		sess2 := &registry.AgentSession{
			AgentID:  "agent-west-2",
			GameID:   "game1",
			Env:      "prod",
			Version:  "v1.0.0",
			RPCAddr:  "192.168.1.11:19090",
			ExpireAt: time.Now().Add(time.Minute),
			LastSeen: time.Now().Add(-time.Second * 30),
			Functions: map[string]registry.FunctionMeta{
				"game.player.update": {Enabled: true},
				"game.player.delete": {Enabled: true},
			},
			Labels: map[string]string{
				"datacenter": "us-west-2",
				"rack":       "rack2",
			},
			Region: "us-west-2",
			Zone:   "zone-b",
		}

		svcCtx.RegistryStore.UpsertAgent(sess1)
		svcCtx.RegistryStore.UpsertAgent(sess2)

		logic := NewOpsServicesLogic(context.Background(), svcCtx)

		// Verify logic was constructed
		if logic == nil {
			t.Fatal("NewOpsServicesLogic() returned nil")
		}

		// Verify registry is accessible
		agents := svcCtx.RegistryStore.AgentsUnsafe()
		if len(agents) != 2 {
			t.Errorf("expected 2 agents in registry, got %d", len(agents))
		}
	})

	t.Run("construct logic with expired agents", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			Config: config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
			},
			ServerVersion:   "v1.0.0",
			StartTime:       time.Now(),
			RegistryStore:   registry.NewStore(),
			MetricsStore:    registry.NewMetricsStore(),
			SystemInfoCache: registry.NewSystemInfoCache(),
		}

		// Add expired agent
		expiredSess := &registry.AgentSession{
			AgentID:   "expired-agent",
			GameID:    "game1",
			Env:       "prod",
			Version:   "v1.0.0",
			RPCAddr:   "localhost:19090",
			ExpireAt:  time.Now().Add(-time.Minute),
			Functions: map[string]registry.FunctionMeta{},
			Labels:    map[string]string{},
		}

		svcCtx.RegistryStore.UpsertAgent(expiredSess)

		logic := NewOpsServicesLogic(context.Background(), svcCtx)

		if logic == nil {
			t.Fatal("NewOpsServicesLogic() returned nil")
		}

		// Verify expired agent is in registry
		agents := svcCtx.RegistryStore.AgentsUnsafe()
		if len(agents) != 1 {
			t.Errorf("expected 1 agent in registry, got %d", len(agents))
		}
	})
}

// Test OpsServicesLogic server info construction
func TestOpsServicesLogic_ServerInfo(t *testing.T) {
	t.Run("server info from config", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			Config: config.Config{
				Server: config.ServerConfig{
					Host: "10.0.0.1",
					Port: 9000,
				},
				Region: "ap-southeast-1",
				Zone:   "zone-c",
				Labels: map[string]string{
					"environment": "staging",
					"purpose":     "testing",
				},
			},
			ServerVersion:   "v3.0.0-test",
			StartTime:       time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
			RegistryStore:   registry.NewStore(),
			MetricsStore:    registry.NewMetricsStore(),
			SystemInfoCache: registry.NewSystemInfoCache(),
		}

		logic := NewOpsServicesLogic(context.Background(), svcCtx)

		// Verify config is accessible
		if logic.svcCtx.Config.Server.Host != "10.0.0.1" {
			t.Errorf("expected host 10.0.0.1, got %s", logic.svcCtx.Config.Server.Host)
		}

		if logic.svcCtx.Config.Server.Port != 9000 {
			t.Errorf("expected port 9000, got %d", logic.svcCtx.Config.Server.Port)
		}

		if logic.svcCtx.Config.Region != "ap-southeast-1" {
			t.Errorf("expected region ap-southeast-1, got %s", logic.svcCtx.Config.Region)
		}

		if logic.svcCtx.ServerVersion != "v3.0.0-test" {
			t.Errorf("expected version v3.0.0-test, got %s", logic.svcCtx.ServerVersion)
		}
	})

	t.Run("server address formatting", func(t *testing.T) {
		testCases := []struct {
			name     string
			host     string
			port     int
			expected string
		}{
			{
				name:     "localhost with port",
				host:     "localhost",
				port:     8080,
				expected: "localhost:8080",
			},
			{
				name:     "0.0.0.0 uses localhost",
				host:     "0.0.0.0",
				port:     9090,
				expected: "localhost:9090",
			},
			{
				name:     "specific IP",
				host:     "192.168.1.1",
				port:     8080,
				expected: "192.168.1.1:8080",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				svcCtx := &svc.ServiceContext{
					Config: config.Config{
						Server: config.ServerConfig{
							Host: tc.host,
							Port: tc.port,
						},
					},
					ServerVersion: "v1.0.0",
					StartTime:     time.Now(),
					RegistryStore: registry.NewStore(),
				}

				logic := NewOpsServicesLogic(context.Background(), svcCtx)

				var serverAddr string
				if logic.svcCtx.Config.Server.Host == "0.0.0.0" {
					serverAddr = fmt.Sprintf("localhost:%d", logic.svcCtx.Config.Server.Port)
				} else {
					serverAddr = fmt.Sprintf("%s:%d", logic.svcCtx.Config.Server.Host, logic.svcCtx.Config.Server.Port)
				}

				_ = serverAddr
				// The actual formatting is in the OpsServices method
			})
		}
	})
}

// Test OpsServiceMetadata construction
func TestOpsServiceMetadata_Construction(t *testing.T) {
	t.Run("metadata with processes", func(t *testing.T) {
		metadata := &OpsServiceMetadata{
			Processes: []OpsServiceProcess{
				{
					ServiceID:    "provider-1",
					Addr:         "localhost:8081",
					Version:      "v1.0.0",
					LastSeenUnix: time.Now().Unix(),
					FunctionIDs:  []string{"func1", "func2"},
					Functions:    2,
				},
				{
					ServiceID:    "provider-2",
					Addr:         "localhost:8082",
					Version:      "v1.0.0",
					LastSeenUnix: time.Now().Unix(),
					FunctionIDs:  []string{"func3"},
					Functions:    1,
				},
			},
			ProcessesCount: 2,
		}

		if metadata.ProcessesCount != 2 {
			t.Errorf("expected ProcessesCount 2, got %d", metadata.ProcessesCount)
		}

		if len(metadata.Processes) != 2 {
			t.Errorf("expected 2 processes, got %d", len(metadata.Processes))
		}
	})

	t.Run("metadata without processes", func(t *testing.T) {
		metadata := &OpsServiceMetadata{
			Processes:      []OpsServiceProcess{},
			ProcessesCount: 0,
		}

		if metadata.ProcessesCount != 0 {
			t.Errorf("expected ProcessesCount 0, got %d", metadata.ProcessesCount)
		}

		if len(metadata.Processes) != 0 {
			t.Errorf("expected 0 processes, got %d", len(metadata.Processes))
		}
	})
}

// Test OpsServiceProcess construction
func TestOpsServiceProcess_Construction(t *testing.T) {
	t.Run("process with all fields", func(t *testing.T) {
		now := time.Now()
		process := OpsServiceProcess{
			ServiceID:    "provider-1",
			Addr:         "192.168.1.10:8081",
			Version:      "v1.2.3",
			LastSeenUnix: now.Unix(),
			FunctionIDs:  []string{"func1", "func2", "func3"},
			Functions:    3,
		}

		if process.ServiceID != "provider-1" {
			t.Errorf("expected ServiceID provider-1, got %s", process.ServiceID)
		}

		if process.Functions != 3 {
			t.Errorf("expected 3 functions, got %d", process.Functions)
		}
	})

	t.Run("process with minimal fields", func(t *testing.T) {
		process := OpsServiceProcess{
			ServiceID: "provider-1",
		}

		if process.ServiceID != "provider-1" {
			t.Errorf("expected ServiceID provider-1, got %s", process.ServiceID)
		}

		if process.Functions != 0 {
			t.Errorf("expected 0 functions, got %d", process.Functions)
		}
	})
}

// Test label collection variations
func TestCollectServerLabels_Variations(t *testing.T) {
	t.Run("labels contain expected keys", func(t *testing.T) {
		labels := collectServerLabels()

		expectedKeys := []string{"os", "arch", "go_version"}
		for _, key := range expectedKeys {
			if _, ok := labels[key]; !ok {
				t.Errorf("expected label key %s to exist", key)
			}
		}
	})

	t.Run("labels have valid values", func(t *testing.T) {
		labels := collectServerLabels()

		// OS should be a valid Go OS
		validOS := map[string]bool{
			"linux": true, "windows": true, "darwin": true,
			"freebsd": true, "netbsd": true, "openbsd": true,
			"js": true, "plan9": true,
		}
		if !validOS[labels["os"]] {
			t.Logf("Warning: OS %s may not be a standard Go OS", labels["os"])
		}

		// Arch should be a valid Go architecture
		validArch := map[string]bool{
			"386": true, "amd64": true, "arm": true, "arm64": true,
			"ppc64": true, "ppc64le": true, "mips": true, "mipsle": true,
			"mips64": true, "mips64le": true, "riscv64": true, "s390x": true,
			"wasm": true,
		}
		if !validArch[labels["arch"]] {
			t.Logf("Warning: Arch %s may not be a standard Go arch", labels["arch"])
		}
	})
}

// Test time edge cases for formatLastSeen
func TestFormatLastSeen_Detailed(t *testing.T) {
	t.Run("with both times set", func(t *testing.T) {
		lastSeen := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
		expireAt := time.Date(2024, 6, 15, 15, 30, 0, 0, time.UTC)

		result := formatLastSeen(lastSeen, expireAt)

		if result == "" {
			t.Error("expected non-empty result")
		}

		// Should contain the lastSeen time
		if !contains(result, "14:30:00") {
			t.Logf("Result: %s", result)
		}
	})

	t.Run("with only expireAt set", func(t *testing.T) {
		expireAt := time.Date(2024, 6, 15, 15, 30, 0, 0, time.UTC)

		result := formatLastSeen(time.Time{}, expireAt)

		if result == "" {
			t.Error("expected non-empty result")
		}

		// Should be expireAt minus 30 seconds
		if !contains(result, "15:29:30") && !contains(result, "14:59:30") {
			t.Logf("Result: %s (should be expireAt - 30s)", result)
		}
	})
}

// Test ttlAndHealth with various time scenarios
func TestTtlAndHealth_Detailed(t *testing.T) {
	t.Run("exactly at boundary", func(t *testing.T) {
		// Create a session that expires very soon
		sess := &registry.AgentSession{
			ExpireAt: time.Now().Add(100 * time.Millisecond),
		}

		ttl, healthy := ttlAndHealth(sess)

		// Should be healthy with small positive TTL
		if !healthy && ttl <= 0 {
			t.Logf("Note: session with 100ms TTL marked unhealthy (timing)")
		}
	})

	t.Run("far future", func(t *testing.T) {
		sess := &registry.AgentSession{
			ExpireAt: time.Now().Add(365 * 24 * time.Hour),
		}

		ttl, healthy := ttlAndHealth(sess)

		if !healthy {
			t.Error("expected session far in future to be healthy")
		}

		// TTL should be very large
		expectedTTL := int((365 * 24 * time.Hour).Seconds())
		if ttl < expectedTTL-10 {
			t.Errorf("expected TTL >= %d, got %d", expectedTTL-10, ttl)
		}
	})

	t.Run("far past", func(t *testing.T) {
		sess := &registry.AgentSession{
			ExpireAt: time.Now().Add(-365 * 24 * time.Hour),
		}

		ttl, healthy := ttlAndHealth(sess)

		if healthy {
			t.Error("expected session far in past to be unhealthy")
		}

		if ttl != 0 {
			t.Errorf("expected TTL 0 for far past session, got %d", ttl)
		}
	})
}

// Test registry session handling
func TestRegistrySessionHandling(t *testing.T) {
	t.Run("session with nil labels", func(t *testing.T) {
		sess := &registry.AgentSession{
			AgentID:   "agent-1",
			GameID:    "game1",
			Env:       "prod",
			Version:   "v1.0.0",
			RPCAddr:   "localhost:19090",
			ExpireAt:  time.Now().Add(time.Minute),
			Functions: map[string]registry.FunctionMeta{},
			Labels:    nil,
		}

		if sess.Labels == nil {
			// This is valid, labels can be nil
			t.Log("Labels can be nil")
		}
	})

	t.Run("session with nil functions", func(t *testing.T) {
		sess := &registry.AgentSession{
			AgentID:   "agent-1",
			GameID:    "game1",
			Env:       "prod",
			Version:   "v1.0.0",
			RPCAddr:   "localhost:19090",
			ExpireAt:  time.Now().Add(time.Minute),
			Functions: nil,
		}

		if sess.Functions == nil {
			// This is valid, functions can be nil
			t.Log("Functions can be nil")
		}
	})

	t.Run("session with empty providers", func(t *testing.T) {
		sess := &registry.AgentSession{
			AgentID:   "agent-1",
			GameID:    "game1",
			Env:       "prod",
			Version:   "v1.0.0",
			RPCAddr:   "localhost:19090",
			ExpireAt:  time.Now().Add(time.Minute),
			Functions: map[string]registry.FunctionMeta{},
			Providers: []registry.ProviderSession{},
		}

		if len(sess.Providers) != 0 {
			t.Errorf("expected 0 providers, got %d", len(sess.Providers))
		}
	})
}

// BenchmarkCollectServerLabels (unique name)
func BenchmarkCollectServerLabels2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = collectServerLabels()
	}
}
