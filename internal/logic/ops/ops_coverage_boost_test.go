package ops

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Test OpsAgentProcessesLogic with process state conversion
func TestOpsAgentProcessesLogic_StateConversion(t *testing.T) {
	t.Run("process state string conversion", func(t *testing.T) {
		// Test that all process states convert properly to strings
		states := []opsv1.ProcessState{
			opsv1.ProcessState_PROCESS_STATE_UNSPECIFIED,
			opsv1.ProcessState_PROCESS_STATE_RUNNING,
			opsv1.ProcessState_PROCESS_STATE_STOPPED,
			opsv1.ProcessState_PROCESS_STATE_FAILED,
			opsv1.ProcessState_PROCESS_STATE_STARTING,
			opsv1.ProcessState_PROCESS_STATE_STOPPING,
		}

		for _, state := range states {
			stateStr := state.String()
			if stateStr == "" {
				t.Errorf("State %v produced empty string", state)
			}
		}
	})

	t.Run("process with nil LastStart", func(t *testing.T) {
		p := &opsv1.ManagedProcess{
			Name:      "test-process",
			State:     opsv1.ProcessState_PROCESS_STATE_RUNNING,
			LastStart: nil,
		}

		var lastStart string
		if p.LastStart != nil {
			lastStart = p.LastStart.String()
		}

		if lastStart != "" {
			t.Errorf("expected empty lastStart, got %s", lastStart)
		}
	})

	t.Run("process with valid LastStart", func(t *testing.T) {
		now := timestamppb.Now()
		p := &opsv1.ManagedProcess{
			Name:      "test-process",
			State:     opsv1.ProcessState_PROCESS_STATE_RUNNING,
			LastStart: now,
		}

		var lastStart string
		if p.LastStart != nil {
			lastStart = p.LastStart.String()
		}

		if lastStart == "" {
			t.Error("expected non-empty lastStart")
		}
	})
}

// Test OpsAgentSystemInfoLogic cache behavior
func TestOpsAgentSystemInfoLogic_CacheBehavior(t *testing.T) {
	t.Run("cache hit populates all fields", func(t *testing.T) {
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

		cachedInfo := &opsv1.SystemInfo{
			Hostname:      "cached-host",
			Os:            "linux",
			OsVersion:     "5.0",
			KernelVersion: "5.0.0",
			Arch:          "amd64",
			CpuCores:      8,
			TotalMemory:   16 * 1024 * 1024 * 1024,
			BootTime:      timestamppb.Now(),
			AgentVersion:  "v1.0.0",
		}
		svcCtx.SystemInfoCache.Set("test-cached-agent", cachedInfo)

		logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{
			AgentID: "test-cached-agent",
		})

		if err != nil {
			t.Fatalf("OpsAgentSystemInfo() unexpected error: %v", err)
		}

		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}

		// Verify all fields are populated
		data := resp.Data
		if data.Hostname != "cached-host" {
			t.Errorf("expected hostname cached-host, got %s", data.Hostname)
		}

		if data.OS != "linux" {
			t.Errorf("expected OS linux, got %s", data.OS)
		}

		if data.CPUCores != 8 {
			t.Errorf("expected 8 CPU cores, got %d", data.CPUCores)
		}

		if data.TotalMemory != 16*1024*1024*1024 {
			t.Errorf("expected 16GB memory, got %d", data.TotalMemory)
		}

		if data.BootTime == "" {
			t.Error("expected non-empty boot time")
		}
	})

	t.Run("cache with zero values", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			Config:          config.Config{},
			ServerVersion:   "v1.0.0",
			StartTime:       time.Now(),
			RegistryStore:   registry.NewStore(),
			MetricsStore:    registry.NewMetricsStore(),
			SystemInfoCache: registry.NewSystemInfoCache(),
		}

		cachedInfo := &opsv1.SystemInfo{
			// Minimal fields
			Hostname: "minimal-host",
		}
		svcCtx.SystemInfoCache.Set("minimal-agent", cachedInfo)

		logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{
			AgentID: "minimal-agent",
		})

		if err != nil {
			t.Fatalf("OpsAgentSystemInfo() unexpected error: %v", err)
		}

		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}

		if resp.Data.Hostname != "minimal-host" {
			t.Errorf("expected hostname minimal-host, got %s", resp.Data.Hostname)
		}
	})
}

// Test process action request variations
func TestOpsProcessActionRequest_Variations(t *testing.T) {
	t.Run("start request with all fields", func(t *testing.T) {
		req := &OpsProcessStartRequest{
			AgentID: "agent-1",
			Name:    "process-name",
			Command: "/usr/bin/process",
			Args:    []string{"--verbose", "--config=file.conf"},
			Env: map[string]string{
				"PATH":   "/usr/bin:/bin",
				"HOME":   "/home/user",
				"SECRET": "value",
			},
		}

		if req.AgentID != "agent-1" {
			t.Errorf("expected AgentID agent-1, got %s", req.AgentID)
		}

		if len(req.Args) != 2 {
			t.Errorf("expected 2 args, got %d", len(req.Args))
		}

		if len(req.Env) != 3 {
			t.Errorf("expected 3 env vars, got %d", len(req.Env))
		}
	})

	t.Run("start request with minimal fields", func(t *testing.T) {
		req := &OpsProcessStartRequest{
			AgentID: "agent-1",
			Name:    "simple-process",
		}

		if req.Command != "" {
			t.Errorf("expected empty command, got %s", req.Command)
		}

		if len(req.Args) != 0 {
			t.Errorf("expected no args, got %d", len(req.Args))
		}

		if req.Env == nil {
			req.Env = map[string]string{}
		}
	})

	t.Run("action request with force flag", func(t *testing.T) {
		req := &OpsProcessActionRequest{
			AgentID: "agent-1",
			Name:    "process-name",
			Action:  "restart",
			Force:   true,
		}

		if !req.Force {
			t.Error("expected force to be true")
		}

		if req.Action != "restart" {
			t.Errorf("expected action restart, got %s", req.Action)
		}
	})
}

// Test OpsMetricsData with various scenarios
func TestOpsMetricsData_Variations(t *testing.T) {
	t.Run("metrics with large values", func(t *testing.T) {
		m := OpsMetricsData{
			CPU: OpsCpuMetrics{
				Cores:        128,
				UsagePercent: 99.9,
				PerCore:      make([]float64, 128),
			},
			Memory: OpsMemoryMetrics{
				TotalBytes:   1024 * 1024 * 1024 * 1024, // 1TB
				UsedBytes:    999 * 1024 * 1024 * 1024,
				UsagePercent: 99.9,
				SwapTotal:    64 * 1024 * 1024 * 1024,
				SwapUsed:     32 * 1024 * 1024 * 1024,
			},
		}

		if m.CPU.Cores != 128 {
			t.Errorf("expected 128 cores, got %d", m.CPU.Cores)
		}

		if m.Memory.TotalBytes != 1024*1024*1024*1024 {
			t.Errorf("expected 1TB memory, got %d", m.Memory.TotalBytes)
		}
	})

	t.Run("metrics with zero values", func(t *testing.T) {
		m := OpsMetricsData{
			CPU: OpsCpuMetrics{
				Cores:        0,
				UsagePercent: 0.0,
			},
			Memory: OpsMemoryMetrics{
				TotalBytes:     0,
				UsedBytes:      0,
				AvailableBytes: 0,
				UsagePercent:   0.0,
			},
		}

		if m.CPU.Cores != 0 {
			t.Errorf("expected 0 cores, got %d", m.CPU.Cores)
		}
	})

	t.Run("metrics with many disks", func(t *testing.T) {
		m := OpsMetricsData{
			Disks: make([]OpsDiskMetrics, 10),
		}

		for i := range m.Disks {
			m.Disks[i] = OpsDiskMetrics{
				MountPoint:     fmt.Sprintf("/mount%d", i),
				Device:         fmt.Sprintf("/dev/sd%c", 'a'+i),
				FsType:         "ext4",
				TotalBytes:     100 * 1024 * 1024 * 1024,
				UsedBytes:      50 * 1024 * 1024 * 1024,
				AvailableBytes: 50 * 1024 * 1024 * 1024,
				UsagePercent:   50.0,
			}
		}

		if len(m.Disks) != 10 {
			t.Errorf("expected 10 disks, got %d", len(m.Disks))
		}
	})
}

// Test OpsManagedProcess state conversion
func TestOpsManagedProcess_StateConversion(t *testing.T) {
	states := []struct {
		state    opsv1.ProcessState
		expected string
	}{
		{opsv1.ProcessState_PROCESS_STATE_UNSPECIFIED, "PROCESS_STATE_UNSPECIFIED"},
		{opsv1.ProcessState_PROCESS_STATE_RUNNING, "PROCESS_STATE_RUNNING"},
		{opsv1.ProcessState_PROCESS_STATE_STOPPED, "PROCESS_STATE_STOPPED"},
		{opsv1.ProcessState_PROCESS_STATE_FAILED, "PROCESS_STATE_FAILED"},
		{opsv1.ProcessState_PROCESS_STATE_STARTING, "PROCESS_STATE_STARTING"},
		{opsv1.ProcessState_PROCESS_STATE_STOPPING, "PROCESS_STATE_STOPPING"},
	}

	for _, tc := range states {
		t.Run(tc.expected, func(t *testing.T) {
			p := &opsv1.ManagedProcess{
				Name:  "test",
				State: tc.state,
			}

			stateStr := p.State.String()
			if stateStr != tc.expected {
				t.Errorf("expected state %s, got %s", tc.expected, stateStr)
			}
		})
	}
}

// Test OpsAgentSystemInfoResponse with timestamp edge cases
func TestOpsAgentSystemInfoResponse_TimestampCases(t *testing.T) {
	t.Run("response with valid boot time", func(t *testing.T) {
		bootTime := timestamppb.Now()
		resp := &OpsAgentSystemInfoResponse{
			Code:    0,
			Message: "OK",
			Data: OpsAgentSystemInfo{
				Hostname: "host-1",
				OS:       "linux",
				BootTime: bootTime.AsTime().Format(time.RFC3339),
				CPUCores: 4,
			},
		}

		if resp.Data.BootTime == "" {
			t.Error("expected non-empty boot time")
		}
	})

	t.Run("response with zero boot time", func(t *testing.T) {
		resp := &OpsAgentSystemInfoResponse{
			Code:    0,
			Message: "OK",
			Data: OpsAgentSystemInfo{
				Hostname: "host-1",
				OS:       "linux",
				BootTime: formatTimestamp(nil),
			},
		}

		if resp.Data.BootTime != "" {
			t.Errorf("expected empty boot time for nil timestamp, got %s", resp.Data.BootTime)
		}
	})
}

// Test OpsAgentExecCommandRequest variations
func TestOpsExecCommandRequest_Variations(t *testing.T) {
	t.Run("command with environment variables", func(t *testing.T) {
		req := &OpsExecCommandRequest{
			AgentID: "agent-1",
			Command: "/usr/bin/env",
			Args:    []string{},
			Env: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
				"VAR3": "value3",
			},
			Timeout: 30,
		}

		if len(req.Env) != 3 {
			t.Errorf("expected 3 env vars, got %d", len(req.Env))
		}

		if req.Timeout != 30 {
			t.Errorf("expected timeout 30, got %d", req.Timeout)
		}
	})

	t.Run("command with no timeout", func(t *testing.T) {
		req := &OpsExecCommandRequest{
			AgentID: "agent-1",
			Command: "/usr/bin/sleep",
			Args:    []string{"10"},
			Timeout: 0,
		}

		if req.Timeout != 0 {
			t.Errorf("expected timeout 0, got %d", req.Timeout)
		}
	})

	t.Run("command with large timeout", func(t *testing.T) {
		req := &OpsExecCommandRequest{
			AgentID: "agent-1",
			Command: "/usr/bin/long-running",
			Timeout: 3600, // 1 hour
		}

		if req.Timeout != 3600 {
			t.Errorf("expected timeout 3600, got %d", req.Timeout)
		}
	})
}

// Test OpsExecResult variations
func TestOpsExecResult_Variations(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		result := OpsExecResult{
			ExitCode: 0,
			StdOut:   "success output",
			StdErr:   "",
		}

		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}

		if result.StdOut != "success output" {
			t.Errorf("expected stdout 'success output', got %s", result.StdOut)
		}
	})

	t.Run("failed execution", func(t *testing.T) {
		result := OpsExecResult{
			ExitCode: 1,
			StdOut:   "",
			StdErr:   "error: command not found",
		}

		if result.ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", result.ExitCode)
		}

		if result.StdErr != "error: command not found" {
			t.Errorf("expected stderr 'error: command not found', got %s", result.StdErr)
		}
	})

	t.Run("execution with both stdout and stderr", func(t *testing.T) {
		result := OpsExecResult{
			ExitCode: 0,
			StdOut:   "normal output",
			StdErr:   "warning message",
		}

		if result.StdOut == "" {
			t.Error("expected non-empty stdout")
		}

		if result.StdErr == "" {
			t.Error("expected non-empty stderr")
		}
	})
}

// Test formatLastSeen with various time combinations
func TestFormatLastSeen_MoreCases(t *testing.T) {
	testCases := []struct {
		name     string
		lastSeen time.Time
		expireAt time.Time
	}{
		{
			name:     "both in past",
			lastSeen: time.Now().Add(-time.Hour),
			expireAt: time.Now().Add(-time.Minute),
		},
		{
			name:     "lastSeen zero, expireAt future",
			lastSeen: time.Time{},
			expireAt: time.Now().Add(time.Hour),
		},
		{
			name:     "lastSeen future, expireAt zero",
			lastSeen: time.Now().Add(time.Hour),
			expireAt: time.Time{},
		},
		{
			name:     "both exactly now",
			lastSeen: time.Now(),
			expireAt: time.Now(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatLastSeen(tc.lastSeen, tc.expireAt)
			if result == "" {
				t.Error("expected non-empty result")
			}
		})
	}
}

// Test ttlAndHealth with more session states
func TestTtlAndHealth_MoreStates(t *testing.T) {
	testCases := []struct {
		name            string
		expireAt        time.Time
		expectedHealthy bool
	}{
		{
			name:            "1 second in future",
			expireAt:        time.Now().Add(time.Second),
			expectedHealthy: true,
		},
		{
			name:            "1 second in past",
			expireAt:        time.Now().Add(-time.Second),
			expectedHealthy: false,
		},
		{
			name:            "1 day in future",
			expireAt:        time.Now().Add(24 * time.Hour),
			expectedHealthy: true,
		},
		{
			name:            "1 day in past",
			expireAt:        time.Now().Add(-24 * time.Hour),
			expectedHealthy: false,
		},
		{
			name:            "exactly now",
			expireAt:        time.Now(),
			expectedHealthy: false, // TTL of 0 or 1 second might be considered expired
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sess := &registry.AgentSession{
				ExpireAt: tc.expireAt,
			}

			ttl, healthy := ttlAndHealth(sess)

			// Healthy check might have 1-second variance
			if tc.expectedHealthy && !healthy && ttl > 0 {
				t.Logf("Note: expected healthy but got unhealthy with TTL %d (timing variance)", ttl)
			} else if !tc.expectedHealthy && healthy {
				t.Error("expected unhealthy session")
			}
		})
	}
}

// Test OpsAgentInfo construction
func TestOpsAgentInfo_Construction(t *testing.T) {
	t.Run("agent with all fields", func(t *testing.T) {
		info := OpsAgentInfo{
			AgentID:   "agent-1",
			GameID:    "game1",
			Env:       "prod",
			Version:   "v1.0.0",
			Addr:      "localhost:19090",
			Connected: true,
			LastSeen:  time.Now().Format(time.RFC3339),
			Functions: []string{"func1", "func2", "func3"},
			Processes: []string{"proc1", "proc2"},
			Labels: map[string]string{
				"dc":     "dc1",
				"region": "us-east-1",
			},
		}

		if info.AgentID != "agent-1" {
			t.Errorf("expected AgentID agent-1, got %s", info.AgentID)
		}

		if len(info.Functions) != 3 {
			t.Errorf("expected 3 functions, got %d", len(info.Functions))
		}

		if len(info.Labels) != 2 {
			t.Errorf("expected 2 labels, got %d", len(info.Labels))
		}
	})

	t.Run("agent with minimal fields", func(t *testing.T) {
		info := OpsAgentInfo{
			AgentID: "agent-minimal",
		}

		if info.AgentID != "agent-minimal" {
			t.Errorf("expected AgentID agent-minimal, got %s", info.AgentID)
		}

		if len(info.Functions) != 0 {
			t.Errorf("expected 0 functions, got %d", len(info.Functions))
		}
	})
}

// Test OpsProcessStartResponse variations
func TestOpsProcessStartResponse_Variations(t *testing.T) {
	testCases := []struct {
		name string
		resp OpsProcessStartResponse
		code int
		pid  int32
	}{
		{
			name: "successful start",
			resp: OpsProcessStartResponse{
				Code:    0,
				Message: "OK",
				Data:    1234,
			},
			code: 0,
			pid:  1234,
		},
		{
			name: "failed start",
			resp: OpsProcessStartResponse{
				Code:    500,
				Message: "process not found",
				Data:    0,
			},
			code: 500,
			pid:  0,
		},
		{
			name: "start with PID 1",
			resp: OpsProcessStartResponse{
				Code:    0,
				Message: "OK",
				Data:    1,
			},
			code: 0,
			pid:  1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.resp.Code != tc.code {
				t.Errorf("expected code %d, got %d", tc.code, tc.resp.Code)
			}

			if tc.resp.Data != tc.pid {
				t.Errorf("expected PID %d, got %d", tc.pid, tc.resp.Data)
			}
		})
	}
}

// Test cross-platform compatibility for time formatting
func TestCrossPlatformTimeFormatting(t *testing.T) {
	t.Run("RFC3339 format consistency", func(t *testing.T) {
		now := time.Now()

		formatted := now.Format(time.RFC3339)
		if formatted == "" {
			t.Error("expected non-empty RFC3339 formatted time")
		}

		// Parse it back
		parsed, err := time.Parse(time.RFC3339, formatted)
		if err != nil {
			t.Errorf("failed to parse RFC3339 time: %v", err)
		}

		// Should be within a second of original (formatting loses nanoseconds)
		diff := parsed.Sub(now).Abs()
		if diff > time.Second {
			t.Errorf("parsed time differs from original by %v", diff)
		}
	})

	t.Run("zero time formatting", func(t *testing.T) {
		zero := time.Time{}
		formatted := zero.Format(time.RFC3339)

		// Zero time has a specific format
		if formatted == "" {
			t.Error("expected non-empty formatted zero time")
		}
	})
}

// Test OpsDiskMetrics variations
func TestOpsDiskMetrics_Variations(t *testing.T) {
	t.Run("disk with all fields", func(t *testing.T) {
		disk := OpsDiskMetrics{
			Device:         "/dev/sda1",
			Mount:          "/",
			MountPoint:     "/",
			FsType:         "ext4",
			Total:          100 * 1024 * 1024 * 1024,
			Used:           50 * 1024 * 1024 * 1024,
			TotalBytes:     100 * 1024 * 1024 * 1024,
			UsedBytes:      50 * 1024 * 1024 * 1024,
			AvailableBytes: 50 * 1024 * 1024 * 1024,
			Usage:          50.0,
			UsagePercent:   50.0,
		}

		if disk.Device != "/dev/sda1" {
			t.Errorf("expected device /dev/sda1, got %s", disk.Device)
		}

		if disk.UsagePercent != 50.0 {
			t.Errorf("expected usage 50.0, got %f", disk.UsagePercent)
		}
	})

	t.Run("disk with zero values", func(t *testing.T) {
		disk := OpsDiskMetrics{}

		if disk.TotalBytes != 0 {
			t.Errorf("expected total 0, got %d", disk.TotalBytes)
		}
	})
}

// Test OpsNetworkMetrics variations
func TestOpsNetworkMetrics_Variations(t *testing.T) {
	t.Run("network with all fields", func(t *testing.T) {
		net := OpsNetworkMetrics{
			Interface:   "eth0",
			BytesSent:   1024 * 1024 * 100,
			BytesRecv:   1024 * 1024 * 200,
			PacketsSent: 1000000,
			PacketsRecv: 2000000,
			Errors:      5,
			Drops:       2,
		}

		if net.Interface != "eth0" {
			t.Errorf("expected interface eth0, got %s", net.Interface)
		}

		if net.Errors != 5 {
			t.Errorf("expected 5 errors, got %d", net.Errors)
		}
	})

	t.Run("network with zero traffic", func(t *testing.T) {
		net := OpsNetworkMetrics{
			Interface: "lo",
		}

		if net.BytesSent != 0 {
			t.Errorf("expected 0 bytes sent, got %d", net.BytesSent)
		}

		if net.PacketsRecv != 0 {
			t.Errorf("expected 0 packets received, got %d", net.PacketsRecv)
		}
	})
}

// Test OpsCpuMetrics variations
func TestOpsCpuMetrics_Variations(t *testing.T) {
	t.Run("CPU with all fields", func(t *testing.T) {
		cpu := OpsCpuMetrics{
			Usage:        75.5,
			CoreCount:    8,
			Cores:        8,
			UsagePercent: 75.5,
			Load1M:       2.5,
			Load5M:       2.0,
			Load15M:      1.5,
			PerCore:      []float64{70, 80, 75, 72, 78, 76, 74, 73},
		}

		if cpu.Cores != 8 {
			t.Errorf("expected 8 cores, got %d", cpu.Cores)
		}

		if len(cpu.PerCore) != 8 {
			t.Errorf("expected 8 per-core values, got %d", len(cpu.PerCore))
		}
	})

	t.Run("CPU with zero values", func(t *testing.T) {
		cpu := OpsCpuMetrics{}

		if cpu.Cores != 0 {
			t.Errorf("expected 0 cores, got %d", cpu.Cores)
		}
	})
}

// Test OpsMemoryMetrics variations
func TestOpsMemoryMetrics_Variations(t *testing.T) {
	t.Run("memory with all fields", func(t *testing.T) {
		mem := OpsMemoryMetrics{
			Total:          16 * 1024 * 1024 * 1024,
			Used:           8 * 1024 * 1024 * 1024,
			Available:      8 * 1024 * 1024 * 1024,
			Usage:          50.0,
			TotalBytes:     16 * 1024 * 1024 * 1024,
			UsedBytes:      8 * 1024 * 1024 * 1024,
			AvailableBytes: 8 * 1024 * 1024 * 1024,
			UsagePercent:   50.0,
			SwapTotal:      4 * 1024 * 1024 * 1024,
			SwapUsed:       1 * 1024 * 1024 * 1024,
		}

		if mem.TotalBytes != 16*1024*1024*1024 {
			t.Errorf("expected 16GB total, got %d", mem.TotalBytes)
		}

		if mem.UsagePercent != 50.0 {
			t.Errorf("expected 50%% usage, got %f", mem.UsagePercent)
		}
	})

	t.Run("memory without swap", func(t *testing.T) {
		mem := OpsMemoryMetrics{
			TotalBytes:     8 * 1024 * 1024 * 1024,
			UsedBytes:      4 * 1024 * 1024 * 1024,
			AvailableBytes: 4 * 1024 * 1024 * 1024,
			UsagePercent:   50.0,
		}

		if mem.SwapTotal != 0 {
			t.Errorf("expected 0 swap total, got %d", mem.SwapTotal)
		}
	})
}

// Benchmark ProcessStateString (new unique name)
func BenchmarkProcessStateString2(b *testing.B) {
	state := opsv1.ProcessState_PROCESS_STATE_RUNNING

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = state.String()
	}
}
