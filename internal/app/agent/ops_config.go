package agent

import (
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
	AllowRestart bool `json:"allow_restart" yaml:"allow_restart"`

	// AllowExec permits arbitrary command execution.
	// WARNING: This is extremely dangerous. Use with extreme caution.
	// Requires Enabled=true. Default: false
	AllowExec bool `json:"allow_exec" yaml:"allow_exec"`

	// MetricsInterval defines how often to collect and report metrics.
	// Default: 30s
	MetricsInterval time.Duration `json:"metrics_interval" yaml:"metrics_interval"`

	// MetricsEnabled controls whether metrics collection is active.
	// Default: true (when Enabled=true)
	MetricsEnabled bool `json:"metrics_enabled" yaml:"metrics_enabled"`

	// ManagedProcesses defines processes that can be managed (restart/stop/start).
	// Each entry maps a logical name to a process configuration.
	ManagedProcesses map[string]ManagedProcessConfig `json:"managed_processes" yaml:"managed_processes"`

	// ExecAllowedCommands limits which commands can be executed.
	// If empty, all commands are allowed (when AllowExec=true).
	// If non-empty, only commands in this list are allowed.
	ExecAllowedCommands []string `json:"exec_allowed_commands" yaml:"exec_allowed_commands"`

	// ExecTimeout is the maximum execution time for commands.
	// Default: 60s, Max: 300s
	ExecTimeout time.Duration `json:"exec_timeout" yaml:"exec_timeout"`
}

// ManagedProcessConfig defines how to manage a process.
type ManagedProcessConfig struct {
	// Command is the command to start the process.
	Command string `json:"command" yaml:"command"`

	// Args are the command arguments.
	Args []string `json:"args" yaml:"args"`

	// WorkingDir is the working directory for the process.
	WorkingDir string `json:"working_dir" yaml:"working_dir"`

	// Env are environment variables for the process.
	Env map[string]string `json:"env" yaml:"env"`

	// GracefulTimeout is the time to wait for graceful shutdown.
	// Default: 30s
	GracefulTimeout time.Duration `json:"graceful_timeout" yaml:"graceful_timeout"`

	// RestartDelay is the delay before restarting after a crash.
	// Default: 5s
	RestartDelay time.Duration `json:"restart_delay" yaml:"restart_delay"`

	// AutoRestart controls whether to automatically restart on crash.
	// Default: false
	AutoRestart bool `json:"auto_restart" yaml:"auto_restart"`
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

// Validate validates the ops configuration.
func (c *OpsConfig) Validate() error {
	if c.MetricsInterval < time.Second {
		c.MetricsInterval = 30 * time.Second
	}
	if c.ExecTimeout <= 0 {
		c.ExecTimeout = 60 * time.Second
	}
	if c.ExecTimeout > 300*time.Second {
		c.ExecTimeout = 300 * time.Second
	}
	return nil
}
