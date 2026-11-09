package config

import (
	"time"
)

// HeraldConfig herald 告警出口接入配置（plugin-mechanism 设计 §5/M2）。
// courier 式接法：croupier 只投事件，渠道/收件人由 herald 侧配置。
type HeraldConfig struct {
	// Enabled 缺省 false：关=纯 croupier 告警页，行为与现状一致。
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// BaseURL herald 服务地址（如 http://herald:8080）。
	BaseURL string `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	// App herald 侧 app 命名空间（herald 配置播种名，默认 croupier）。
	App string `json:"app,omitempty" yaml:"app,omitempty"`
	// TokenEnv 持 trigger token 的环境变量名（默认 HERALD_TRIGGER_TOKEN）：
	// 凭证走环境变量引用，不落配置文件。
	TokenEnv string `json:"tokenEnv,omitempty" yaml:"tokenEnv,omitempty"`
	// Target 默认受众 ref（默认 group:gm-ops，herald 侧解析到群组/值班表）。
	Target string `json:"target,omitempty" yaml:"target,omitempty"`
}

// ProvisionConfig 云资源供给引擎配置（OPS-001）。云 provider 凭据不落配置：
// 走各自环境变量（ALICLOUD_ACCESS_KEY_ID / TENCENTCLOUD_SECRET_ID /
// AWS_ACCESS_KEY_ID / HCLOUD_TOKEN）；agent 注册 bootstrap token 走
// BootstrapTokenEnv 环境变量引用，本段只含驱动与拓扑参数。
type ProvisionConfig struct {
	// Enabled 缺省 false：不装配供给引擎。
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Approval 网络级审批模式（OPS-003 审批双模式）：auto=个人直通（缺省），
	// workflow=团队扩容/销毁申请单（创建→审批→执行，审计留痕）。
	Approval string `json:"approval,omitempty" yaml:"approval,omitempty"`
	// BinaryPath tofu/terraform 二进制路径（默认 tofu）。
	BinaryPath string `json:"binaryPath,omitempty" yaml:"binaryPath,omitempty"`
	// WorkDir 各供给运行的 terraform 工作目录根（默认 data/provision）。
	WorkDir string `json:"workDir,omitempty" yaml:"workDir,omitempty"`
	// HubAddr agent 拨入地址 host:port（cloud-init 注入 user_data）。
	HubAddr string `json:"hubAddr,omitempty" yaml:"hubAddr,omitempty"`
	// BootstrapTokenEnv 持 agent 注册 token 的环境变量名（默认
	// CROUPIER_PROVISION_TOKEN；空=未配置，供给请求被拒）。
	BootstrapTokenEnv string `json:"bootstrapTokenEnv,omitempty" yaml:"bootstrapTokenEnv,omitempty"`
	// AgentInstallURL agent 安装脚本 URL（cloud-init 注入；空=只写配置不装）。
	AgentInstallURL string `json:"agentInstallUrl,omitempty" yaml:"agentInstallUrl,omitempty"`
	// RegisterTimeoutSec 等待自注册超时（秒，默认 300）。
	RegisterTimeoutSec int `json:"registerTimeoutSec,omitempty" yaml:"registerTimeoutSec,omitempty"`
}

// ProvisionApprovalAuto / ProvisionApprovalWorkflow 是供给网络级审批模式取值。
const (
	// ProvisionApprovalAuto 个人直通：扩容/销毁直接执行（缺省）。
	ProvisionApprovalAuto = "auto"
	// ProvisionApprovalWorkflow 团队申请单：扩容/销毁创建审批记录，
	// 批准后执行。
	ProvisionApprovalWorkflow = "workflow"
)

// EffectiveApproval 返回网络级审批模式，空用默认 auto。
func (c ProvisionConfig) EffectiveApproval() string {
	if c.Approval == ProvisionApprovalWorkflow {
		return ProvisionApprovalWorkflow
	}
	return ProvisionApprovalAuto
}

// EffectiveBinaryPath 返回二进制路径，空用默认 tofu。
func (c ProvisionConfig) EffectiveBinaryPath() string {
	if c.BinaryPath != "" {
		return c.BinaryPath
	}
	return "tofu"
}

// EffectiveWorkDir 返回工作目录根，空用默认 data/provision。
func (c ProvisionConfig) EffectiveWorkDir() string {
	if c.WorkDir != "" {
		return c.WorkDir
	}
	return "data/provision"
}

// EffectiveBootstrapTokenEnv 返回 token 环境变量名，空用默认。
func (c ProvisionConfig) EffectiveBootstrapTokenEnv() string {
	if c.BootstrapTokenEnv != "" {
		return c.BootstrapTokenEnv
	}
	return "CROUPIER_PROVISION_TOKEN"
}

// EffectiveRegisterTimeout 返回等待自注册超时（秒，默认 300）。
func (c ProvisionConfig) EffectiveRegisterTimeout() time.Duration {
	if c.RegisterTimeoutSec > 0 {
		return time.Duration(c.RegisterTimeoutSec) * time.Second
	}
	return 300 * time.Second
}
