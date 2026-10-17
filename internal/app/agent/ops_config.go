package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/tasks/scheduler"
	"gopkg.in/yaml.v3"
)

// OpsConfig defines configuration for the ops module.
// WARNING: This module can execute privileged operations. Enable with caution.
type OpsConfig struct {
	// Enabled controls whether the ops module is active.
	// Default: false (must be explicitly enabled)
	Enabled bool `json:"enabled" yaml:"enabled"`

	// AllowRestart permits process restart/stop/start operations.
	// Requires Enabled=true. Default: false
	AllowRestart bool `json:"allowRestart" yaml:"allowRestart"`

	// AllowExec permits arbitrary command execution.
	// WARNING: This is extremely dangerous. Use with extreme caution.
	// Requires Enabled=true. Default: false
	AllowExec bool `json:"allowExec" yaml:"allowExec"`

	// AllowUpdate permits managed-process binary updates (§10.4 更新状态).
	// Independent authorization gate (§10.7 程序操作=明确受授权动作): updates
	// are a separate action family from restart/exec, gated on their own key.
	// Content-pinned: every update must carry the expected sha256 (fail-closed).
	// Requires Enabled=true. Default: false
	AllowUpdate bool `json:"allowUpdate" yaml:"allowUpdate"`

	// MetricsInterval defines how often to collect and report metrics.
	// Default: 30s
	MetricsInterval time.Duration `json:"metricsInterval" yaml:"metricsInterval"`

	// MetricsEnabled controls whether metrics collection is active.
	// Default: true (when Enabled=true)
	MetricsEnabled bool `json:"metricsEnabled" yaml:"metricsEnabled"`

	// ManagedProcesses defines processes that can be managed (restart/stop/start).
	// Each entry maps a logical name to a process configuration.
	ManagedProcesses map[string]ManagedProcessConfig `json:"managedProcesses" yaml:"managedProcesses"`

	// ExecAllowedCommands limits which commands can be executed.
	// Hard prerequisite for AllowExec (agent-supervisor-design §10.7 白名单
	// 前置): the list must be non-empty when AllowExec=true — an empty list
	// denies everything (fail-closed), it never means "allow all".
	ExecAllowedCommands []string `json:"execAllowedCommands" yaml:"execAllowedCommands"`

	// ExecTimeout is the maximum execution time for commands.
	// Default: 60s, Max: 300s
	ExecTimeout time.Duration `json:"execTimeout" yaml:"execTimeout"`

	// SupervisorLog configures the supervisor event log (rotating file). The
	// event log is the full-truth record; a truncated copy is mirrored to the
	// server's in-memory ring for the panel.
	SupervisorLog SupervisorLogConfig `json:"supervisorLog" yaml:"supervisorLog"`

	// SnapshotDir is the base directory holding crash snapshot artifacts, one
	// sub-directory per managed process. Created with 0700 permissions (the
	// dumps contain process memory). Default: logs/snapshots
	SnapshotDir string `json:"snapshotDir" yaml:"snapshotDir"`
}

// UnmarshalYAML preserves the old snake_case metric keys while the remaining
// fields use the canonical lower-camel-case configuration contract.
func (c *OpsConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain OpsConfig
	decoded := DefaultOpsConfig()
	if err := value.Decode((*plain)(decoded)); err != nil {
		return err
	}
	var compat struct {
		MetricsInterval time.Duration `yaml:"metrics_interval"`
		MetricsEnabled  *bool         `yaml:"metrics_enabled"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if compat.MetricsInterval != 0 {
		decoded.MetricsInterval = compat.MetricsInterval
	}
	if compat.MetricsEnabled != nil {
		decoded.MetricsEnabled = *compat.MetricsEnabled
	}
	*c = *decoded
	return nil
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
//
// Kind 区分受管对象类别（agent-supervisor-design §10.1 统一受管对象）：
// program 是游戏辅助程序，probe-adapter 是外部探测适配器——两类共用同一套
// 生命周期（启停/监管/快照/调度/更新），不各建一套。适配器清单只声明身份、
// 入口、协议、能力（§10.8）；游戏地址/频率/授权/凭证留在探测面配置，不进清单。
type ManagedProcessConfig struct {
	// Command is the command to start the process.
	Command string `json:"command" yaml:"command"`

	// Args are the command arguments.
	Args []string `json:"args" yaml:"args"`

	// Kind is the managed-object category. Empty or "program" means a game
	// sidecar program (the default, so pre-existing configs are unchanged);
	// "probe-adapter" declares an external probe adapter that the probe
	// surface may reference. Closed set — unknown values fail at load.
	Kind string `json:"kind" yaml:"kind"`

	// AdapterID is the adapter identity (manifest metadata.id, e.g.
	// "com.example.game-a"). Required when Kind=probe-adapter: it is the
	// reference anchor for the probe surface and the authorization check.
	AdapterID string `json:"adapterId" yaml:"adapterId"`

	// AdapterVersion is the declared adapter version (manifest metadata.version).
	AdapterVersion string `json:"adapterVersion" yaml:"adapterVersion"`

	// AdapterProtocol is the IPC protocol name. Empty falls back to the
	// canonical croupier-probe-jsonl-v1 contract.
	AdapterProtocol string `json:"adapterProtocol" yaml:"adapterProtocol"`

	// AdapterCapabilities is the adapter's declared capability set (e.g.
	// login / handshake). Declaration is not verification: the platform still
	// checks version, capability claims and authorization scope.
	AdapterCapabilities []string `json:"adapterCapabilities" yaml:"adapterCapabilities"`

	// WorkingDir is the working directory for the process.
	WorkingDir string `json:"workingDir" yaml:"workingDir"`

	// Env are environment variables for the process.
	Env map[string]string `json:"env" yaml:"env"`

	// GracefulTimeout is the time to wait for graceful shutdown.
	// Default: 30s
	GracefulTimeout time.Duration `json:"gracefulTimeout" yaml:"gracefulTimeout"`

	// RestartDelay is the delay before restarting after a crash.
	// Default: 5s
	RestartDelay time.Duration `json:"restartDelay" yaml:"restartDelay"`

	// AutoRestart controls whether to automatically restart on crash.
	// Default: false
	AutoRestart bool `json:"autoRestart" yaml:"autoRestart"`

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

	// Schedule declares timed start/stop/restart actions (§10.6 调度状态).
	// Same five-field cron model as the server-side task scheduler
	// (internal/tasks/scheduler) — one schedule model across the repo. Local
	// declared intent from the agent's own config: same trust root as
	// autoRestart, so allowRestart (the remote-operation gate) does not apply.
	// No catch-up: slots missed while the agent is down never fire (crond
	// semantics).
	Schedule []ProcessScheduleAction `json:"schedule" yaml:"schedule"`
}

// ProcessScheduleAction is a single timed action on a managed process.
type ProcessScheduleAction struct {
	// Action is the lifecycle action to fire. Closed set: start/stop/restart.
	Action string `json:"action" yaml:"action"`

	// Cron is a five-field cron expression (minute hour day month weekday),
	// parsed by the same parser as the server-side task scheduler.
	Cron string `json:"cron" yaml:"cron"`
}

// snapshotProfileClosure is the closed set of snapshot profiles (S3).
var snapshotProfileClosure = map[string]bool{
	"none": true, "go": true, "node": true, "python": true, "jvm": true,
}

// 受管对象类别闭集（§10.1）。kind=program 是缺省类别；probe-adapter 是外部
// 探测适配器（§10.8 清单只声明身份/入口/协议/能力）。
const (
	ManagedKindProgram      = "program"
	ManagedKindProbeAdapter = "probe-adapter"
	// defaultAdapterProtocol 是外部探测适配器的 IPC 契约名（字段规范由
	// §10.8 待定项收敛，契约名先行固定）。
	defaultAdapterProtocol = "croupier-probe-jsonl-v1"
)

var managedKindClosure = map[string]bool{
	ManagedKindProgram: true, ManagedKindProbeAdapter: true,
}

// 定时动作闭集（§10.6 调度状态）。
const (
	ScheduleActionStart   = "start"
	ScheduleActionStop    = "stop"
	ScheduleActionRestart = "restart"
)

var scheduleActionClosure = map[string]bool{
	ScheduleActionStart: true, ScheduleActionStop: true, ScheduleActionRestart: true,
}

// kind 报告受管对象类别；缺省=program（存量配置零漂移）。
func (c *ManagedProcessConfig) kind() string {
	if c == nil {
		return ManagedKindProgram
	}
	switch k := strings.TrimSpace(c.Kind); k {
	case "", ManagedKindProgram:
		return ManagedKindProgram
	case ManagedKindProbeAdapter:
		return ManagedKindProbeAdapter
	default:
		return k
	}
}

// EffectiveKind returns the normalized managed-object kind.
func (c *ManagedProcessConfig) EffectiveKind() string {
	return c.kind()
}

// EffectiveAdapterProtocol returns the adapter IPC protocol name, defaulting
// to the canonical contract name when unset.
func (c *ManagedProcessConfig) EffectiveAdapterProtocol() string {
	if c == nil {
		return defaultAdapterProtocol
	}
	if p := strings.TrimSpace(c.AdapterProtocol); p != "" {
		return p
	}
	return defaultAdapterProtocol
}

// Validate reports whether the managed-object kind and snapshot profile are
// in their closed sets. Empty values are accepted (means program / none); any
// other unknown value is an error — a typo must fail at load, not silently
// misclassify the object or disable snapshots.
func (c *ManagedProcessConfig) Validate() error {
	if !managedKindClosure[c.kind()] {
		return fmt.Errorf("unknown kind %q (closed set: program/probe-adapter)", c.Kind)
	}
	if c.kind() == ManagedKindProbeAdapter && strings.TrimSpace(c.AdapterID) == "" {
		return fmt.Errorf("kind=probe-adapter requires adapterId (probe-surface reference and authorization anchor)")
	}
	for i, sc := range c.Schedule {
		if !scheduleActionClosure[sc.Action] {
			return fmt.Errorf("schedule[%d]: unknown action %q (closed set: start/stop/restart)", i, sc.Action)
		}
		if _, err := scheduler.ParseCron(sc.Cron); err != nil {
			return fmt.Errorf("schedule[%d]: %w", i, err)
		}
	}
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
	if c.AllowExec && len(c.ExecAllowedCommands) == 0 {
		return fmt.Errorf("allowExec requires execAllowedCommands (whitelist is a hard prerequisite; empty list never means allow-all)")
	}
	for name := range c.ManagedProcesses {
		cfg := c.ManagedProcesses[name]
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("managedProcesses[%s]: %w", name, err)
		}
	}
	return nil
}
