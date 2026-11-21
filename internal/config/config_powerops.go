package config

// PoweropsConfig is the remote machine power lifecycle (OPS power
// management, docs/design/ops-power-management.md). enabled=false (default)
// means the orchestrator is not wired and /ops/power endpoints honestly
// report 503 powerops_not_configured.
type PoweropsConfig struct {
	// Enabled 缺省 false：不装配电源编排器。
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// IPMI 出厂默认走 ipmitool；二进制缺失时该 provider 不装配（提交时报
	// provider 缺失，不拖到批准后失败）。
	IPMI PoweropsIPMIConfig `json:"ipmi,omitempty" yaml:"ipmi,omitempty"`
	// Cloud 命令模板层（design §9）：逐动作命令行，{instanceRef} 占位。
	Cloud PoweropsCloudConfig `json:"cloud,omitempty" yaml:"cloud,omitempty"`
	// VerifyTimeoutMs 状态确认窗口（毫秒，默认 120000）。
	VerifyTimeoutMs int `json:"verifyTimeoutMs,omitempty" yaml:"verifyTimeoutMs,omitempty"`
	// VerifyIntervalMs 轮询间隔（毫秒，默认 2000）。
	VerifyIntervalMs int `json:"verifyIntervalMs,omitempty" yaml:"verifyIntervalMs,omitempty"`
	// ActionTimeoutMs 单次 provider 动作调用（含状态查询）的独立上限（毫秒，
	// 默认 30000）：到点即记该行失败并继续下一台，永不返回的 provider 不阻塞
	// 请求收敛（PWR-002）。
	ActionTimeoutMs int `json:"actionTimeoutMs,omitempty" yaml:"actionTimeoutMs,omitempty"`
	// RequestTimeoutMs 整个请求执行（动作 + 状态确认）的上限（毫秒，默认 300000）：
	// 超时后请求落为可观察终态，不再无限挂在 executing/verifying。
	RequestTimeoutMs int `json:"requestTimeoutMs,omitempty" yaml:"requestTimeoutMs,omitempty"`
}

// PoweropsIPMIConfig is the ipmitool wrapper config. Password lives in the
// named environment variable (provision bootstrap-token pattern) — committed
// config carries the name, never the secret.
type PoweropsIPMIConfig struct {
	// BinaryPath 默认 ipmitool。
	BinaryPath string `json:"binaryPath,omitempty" yaml:"binaryPath,omitempty"`
	// User BMC 用户名（可空：部分管理网 BMC 免认证）。
	User string `json:"user,omitempty" yaml:"user,omitempty"`
	// PasswordEnv 存 BMC 密码的环境变量名（可空）。
	PasswordEnv string `json:"passwordEnv,omitempty" yaml:"passwordEnv,omitempty"`
	// ExtraArgs 追加在 chassis 子命令前（如 -I lanplus）。
	ExtraArgs []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
}

// PoweropsCloudConfig maps actions to provider CLI command templates; the
// {instanceRef} placeholder is replaced per machine.
type PoweropsCloudConfig struct {
	PowerOnCmd  string `json:"powerOnCmd,omitempty" yaml:"powerOnCmd,omitempty"`
	PowerOffCmd string `json:"powerOffCmd,omitempty" yaml:"powerOffCmd,omitempty"`
	ForceOffCmd string `json:"forceOffCmd,omitempty" yaml:"forceOffCmd,omitempty"`
	RebootCmd   string `json:"rebootCmd,omitempty" yaml:"rebootCmd,omitempty"`
	StatusCmd   string `json:"statusCmd,omitempty" yaml:"statusCmd,omitempty"`
}

// EffectiveVerifyTimeout defaults to 120s.
func (c PoweropsConfig) EffectiveVerifyTimeout() int {
	if c.VerifyTimeoutMs > 0 {
		return c.VerifyTimeoutMs
	}
	return 120000
}

// EffectiveVerifyInterval defaults to 2s.
func (c PoweropsConfig) EffectiveVerifyInterval() int {
	if c.VerifyIntervalMs > 0 {
		return c.VerifyIntervalMs
	}
	return 2000
}

// EffectiveActionTimeout defaults to 30s.
func (c PoweropsConfig) EffectiveActionTimeout() int {
	if c.ActionTimeoutMs > 0 {
		return c.ActionTimeoutMs
	}
	return 30000
}

// EffectiveRequestTimeout defaults to 5m.
func (c PoweropsConfig) EffectiveRequestTimeout() int {
	if c.RequestTimeoutMs > 0 {
		return c.RequestTimeoutMs
	}
	return 300000
}
