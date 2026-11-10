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
