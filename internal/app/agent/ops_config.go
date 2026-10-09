package agent

import (
	"fmt"
	"strings"
	"time"
)

// OpsConfig defines configuration for the ops module.
// WARNING: This module can execute privileged operations. Enable with caution.
type OpsConfig struct {
	// Enabled controls whether the ops module is active.
	// Default: false (must be explicitly enabled)
	Enabled bool `json:"enabled" yaml:"enabled"`

	// AllowRestart permits process restart/stop/start operations.
	// Requires Enabled=true. Default: false
	AllowRestart bool `json:"allowRestart" yaml:"allow_restart"`

	// AllowExec permits arbitrary command execution.
	// WARNING: This is extremely dangerous. Use with extreme caution.
	// Requires Enabled=true. Default: false
	AllowExec bool `json:"allowExec" yaml:"allow_exec"`

	// MetricsInterval defines how often to collect and report metrics.
	// Default: 30s
	MetricsInterval time.Duration `json:"metricsInterval" yaml:"metrics_interval"`

	// MetricsEnabled controls whether metrics collection is active.
	// Default: true (when Enabled=true)
	MetricsEnabled bool `json:"metricsEnabled" yaml:"metrics_enabled"`

	// ManagedProcesses defines processes that can be managed (restart/stop/start).
	// Each entry maps a logical name to a process configuration.
	ManagedProcesses map[string]ManagedProcessConfig `json:"managedProcesses" yaml:"managed_processes"`

	// ExecAllowedCommands limits which commands can be executed.
	// If empty, all commands are allowed (when AllowExec=true).
	// If non-empty, only commands in this list are allowed.
	ExecAllowedCommands []string `json:"execAllowedCommands" yaml:"exec_allowed_commands"`

	// ExecTimeout is the maximum execution time for commands.
	// Default: 60s, Max: 300s
	ExecTimeout time.Duration `json:"execTimeout" yaml:"exec_timeout"`

	// SupervisorLog configures the supervisor event log (rotating file). The
	// event log is the full-truth record; a truncated copy is mirrored to the
	// server's in-memory ring for the panel.
	SupervisorLog SupervisorLogConfig `json:"supervisorLog" yaml:"supervisorLog"`

	// SnapshotDir is the base directory holding crash snapshot artifacts, one
	// sub-directory per managed process. Created with 0700 permissions (the
	// dumps contain process memory). Default: logs/snapshots
	SnapshotDir string `json:"snapshotDir" yaml:"snapshotDir"`
}

// SupervisorLogConfig configures the rotating supervisor event log file.
type SupervisorLogConfig struct {
	// Dir is the directory holding the rotating log file.
	// Default: <data dir>/supervisor (falls back to os temp dir when empty)
	Dir string `json:"dir" yaml:"dir"`

	// MaxSizeMB is the max size in MB before rotation. Default: 10
	MaxSizeMB int `json:"maxSizeMB" yaml:"maxSizeMB"`

	// MaxBackups is the max number of rotated files to keep. Default: 5
	MaxBackups int `json:"maxBackups" yaml:"maxBackups"`

	// MaxAgeDays is the max age in days of rotated files. Default: 7
	MaxAgeDays int `json:"maxAgeDays" yaml:"maxAgeDays"`
}

// ManagedProcessConfig defines how to manage a process.
type ManagedProcessConfig struct {
	// Command is the command to start the process.
	Command string `json:"command" yaml:"command"`

	// Args are the command arguments.
	Args []string `json:"args" yaml:"args"`

	// WorkingDir is the working directory for the process.
	WorkingDir string `json:"workingDir" yaml:"working_dir"`

	// Env are environment variables for the process.
	Env map[string]string `json:"env" yaml:"env"`

	// GracefulTimeout is the time to wait for graceful shutdown.
	// Default: 30s
	GracefulTimeout time.Duration `json:"gracefulTimeout" yaml:"graceful_timeout"`

	// RestartDelay is the delay before restarting after a crash.
	// Default: 5s
	RestartDelay time.Duration `json:"restartDelay" yaml:"restart_delay"`

	// AutoRestart controls whether to automatically restart on crash.
	// Default: false
	AutoRestart bool `json:"autoRestart" yaml:"auto_restart"`

	// RestartBackoffInitial is the first backoff delay in the exponential
	// restart sequence (1s → 2s → 4s …). Default: 1s
	RestartBackoffInitial time.Duration `json:"restartBackoffInitial" yaml:"restartBackoffInitial"`

	// RestartBackoffMax caps the backoff delay. Default: 60s
	RestartBackoffMax time.Duration `json:"restartBackoffMax" yaml:"restartBackoffMax"`

	// RestartBreakerLimit is the consecutive-failure count that trips the
	// circuit breaker (state becomes BROKEN, auto-restart stops). 0 disables
	// the breaker (not recommended). Default: 5
	RestartBreakerLimit int `json:"restartBreakerLimit" yaml:"restartBreakerLimit"`

	// MemThresholdBytes marks the process with the mem_over_limit flag when
	// sampled RSS reaches this value. 0 disables the check. Default: 0
	MemThresholdBytes int64 `json:"memThresholdBytes" yaml:"memThresholdBytes"`

	// CpuThresholdPercent marks the process with the cpu_over_limit flag when
	// sampled CPU usage reaches this percentage. 0 disables the check. Default: 0
	CpuThresholdPercent float64 `json:"cpuThresholdPercent" yaml:"cpuThresholdPercent"`

	// SnapshotProfile selects the crash snapshot profile injected at spawn
	// (closed set: none/go/node/python/jvm, explicit declaration — no language
	// auto-detection). Default: none (no snapshot switches injected).
	SnapshotProfile string `json:"snapshotProfile" yaml:"snapshotProfile"`
}

// snapshotProfileClosure is the closed set of snapshot profiles (S3).
var snapshotProfileClosure = map[string]bool{
	"none": true, "go": true, "node": true, "python": true, "jvm": true,
}

// Validate reports whether the snapshot profile is in the closed set. An
// empty value is accepted (means "none"); any other unknown value is an
// error — a typo must fail at load, not silently disable snapshots.
func (c *ManagedProcessConfig) Validate() error {
	if c.SnapshotProfile == "" {
		return nil
	}
	if !snapshotProfileClosure[c.SnapshotProfile] {
		return fmt.Errorf("unknown snapshotProfile %q (closed set: none/go/node/python/jvm)", c.SnapshotProfile)
	}
	return nil
}

// DefaultOpsConfig returns the default ops configuration.
func DefaultOpsConfig() *OpsConfig {
	return &OpsConfig{
		Enabled:          false,
		AllowRestart:     false,
		AllowExec:        false,
		MetricsInterval:  30 * time.Second,
		MetricsEnabled:   true,
		ManagedProcesses: make(map[string]ManagedProcessConfig),
		ExecTimeout:      60 * time.Second,
	}
}

// SnapshotDirOrDefault returns the configured snapshot base directory, or
// "logs/snapshots" when unset (same relative convention as SupervisorLog).
func (c *OpsConfig) SnapshotDirOrDefault() string {
	if d := strings.TrimSpace(c.SnapshotDir); d != "" {
		return d
	}
	return "logs/snapshots"
}

// Validate validates the ops configuration.
//
// MetricsInterval limits:
//   - Minimum (3s): Prevents excessive CPU/memory usage from frequent collection.
//     Each collection involves reading /proc, calculating averages, and serializing data.
//     Below 3s, the overhead becomes significant relative to the monitoring benefit.
//   - Maximum (24h): Ensures system metrics remain reasonably current for operational
//     visibility. Beyond 24h, metrics become stale and useless for troubleshooting.
//     For production environments, 30-60s is recommended; for development, 5-10m is fine.
func (c *OpsConfig) Validate() error {
	// MetricsInterval minimum is 3 seconds to prevent excessive resource usage
	const minMetricsInterval = 3 * time.Second
	// MetricsInterval maximum is 24 hours to ensure metrics remain useful
	const maxMetricsInterval = 24 * time.Hour

	if c.MetricsInterval < minMetricsInterval {
		c.MetricsInterval = minMetricsInterval
	}
	if c.MetricsInterval > maxMetricsInterval {
		c.MetricsInterval = maxMetricsInterval
	}
	if c.ExecTimeout <= 0 {
		c.ExecTimeout = 60 * time.Second
	}
	if c.ExecTimeout > 300*time.Second {
		c.ExecTimeout = 300 * time.Second
	}
	for name := range c.ManagedProcesses {
		cfg := c.ManagedProcesses[name]
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("managedProcesses[%s]: %w", name, err)
		}
	}
	return nil
}
