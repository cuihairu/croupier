// Package provision 实现云资源供给引擎（OPS-001）：用 hashicorp terraform-exec
// 驱动 tofu/terraform，provider 模板内嵌（aliyun/tencent/aws/hetzner），一键扩容
// 闭环 = 选厂商/地域/规格 → create → cloud-init 注入 croupier agent（bootstrap
// token+hub 地址）→ 自注册 → 按网络审批策略（个人直通/团队工单）→ 纳管。
//
// 设计边界：本包不实现任何云适配器——云资源全部走 Terraform provider 模板；
// croupier 只做策略与编排。执行层经 Runner 接口抽象：生产用 terraform-exec
// （TfexecRunner），测试用 FakeRunner 走同一编排代码路径（真实 provider 集成
// 需云凭据 + tofu 二进制，登记为部署后人工验收项，见 docs/review/）。
package provision

import (
	"time"
)

// Provider 是支持的云厂商标识。
type Provider string

const (
	// ProviderAliyun 阿里云（alicloud provider）。
	ProviderAliyun Provider = "aliyun"
	// ProviderTencent 腾讯云（tencentcloud provider）。
	ProviderTencent Provider = "tencent"
	// ProviderAWS AWS（aws provider）。
	ProviderAWS Provider = "aws"
	// ProviderHetzner Hetzner（hcloud provider）。
	ProviderHetzner Provider = "hetzner"
)

// AllProviders 返回支持的 provider 列表（模板渲染与校验共用真值）。
func AllProviders() []Provider {
	return []Provider{ProviderAliyun, ProviderTencent, ProviderAWS, ProviderHetzner}
}

// PolicyMode 是网络审批策略：个人直通（direct，自动放行）或团队工单（ticket，
// 需审批通过后继续）。
type PolicyMode string

const (
	// PolicyDirect 个人直通：请求创建即执行扩容。
	PolicyDirect PolicyMode = "direct"
	// PolicyTicket 团队工单：先建审批记录，批准后才执行扩容。
	PolicyTicket PolicyMode = "ticket"
)

// State 是供给运行的状态。
type State string

const (
	// StatePending 已受理，等待执行（ticket 模式下等待审批通过）。
	StatePending State = "pending"
	// StateApplying terraform apply 执行中。
	StateApplying State = "applying"
	// StateWaitingRegister 已创建，等待 agent 自注册（bootstrap 拨入）。
	StateWaitingRegister State = "waiting_register"
	// StateAdopted 已纳管（agent 注册成功，纳入 game/env 作用域）。
	StateAdopted State = "adopted"
	// StateFailed 失败（含 apply 失败/注册超时/审批拒绝）。
	StateFailed State = "failed"
	// StateDestroyed 已销毁（OPS-002 drain→destroy 复用同一状态）。
	StateDestroyed State = "destroyed"
)

// Spec 是实例规格（跨 provider 归一化；provider 模板映射到各自 instance_type）。
type Spec struct {
	// InstanceType 是规格，如 hetzner "cx21"、aws "t3.micro"、aliyun "ecs.s6-c1.m1.large"。
	InstanceType string `json:"instanceType"`
	// Image 是镜像标识；空则用 provider 模板默认（各云最新 LTS 或 provider 内默认）。
	Image string `json:"image,omitempty"`
	// DiskSizeGB 是系统盘大小（GB）；空用模板默认。
	DiskSizeGB int `json:"diskSizeGB,omitempty"`
}

// Request 是一次扩容请求（API 入参 + 编排输入）。
type Request struct {
	// Provider 云厂商（aliyun/tencent/aws/hetzner）。
	Provider Provider `json:"provider"`
	// Region 地域（如 "eu-central"、"cn-hangzhou"、"ap-southeast-1"）。
	Region string `json:"region"`
	// Count 实例数量（>=1）。
	Count int `json:"count"`
	// Spec 实例规格。
	Spec Spec `json:"spec"`
	// GameID 纳管作用域游戏。
	GameID string `json:"gameId"`
	// Env 纳管作用域环境（prod/staging/...）。
	Env string `json:"env"`
	// Policy 网络审批策略（direct/ticket）；空默认 direct。
	Policy PolicyMode `json:"policy,omitempty"`
	// Labels 附加标签（随 agent 注册注入，供筛选/纳管）。
	Labels map[string]string `json:"labels,omitempty"`
	// CloudInitExtra 追加到 cloud-init 的自定义片段（如安装内网 agent 包地址）。
	CloudInitExtra string `json:"cloudInitExtra,omitempty"`
	// RequestedBy 是发起人（认证上下文）。
	RequestedBy string `json:"requestedBy,omitempty"`
	// BootstrapToken 是注入 agent 的注册 token（敏感，仅编排内部使用，不入库）。
	BootstrapToken string `json:"-"`
	// HubAddr 是 agent 拨入的 hub 地址（host:port）。
	HubAddr string `json:"-"`
}

// Run 是一次供给运行的持久化状态。
type Run struct {
	// ID 是运行 ID（requestId）。
	ID string `json:"id"`
	// Request 是入参快照。
	Request Request `json:"request"`
	// State 当前状态。
	State State `json:"state"`
	// Outputs 是 terraform apply 的输出（实例 id/ip/agent_id 等）。
	Outputs map[string]string `json:"outputs,omitempty"`
	// AgentIDs 是已纳管的 agent id 列表（自注册完成）。
	AgentIDs []string `json:"agentIDs,omitempty"`
	// ApprovalID 是 ticket 模式创建的审批记录 id。
	ApprovalID string `json:"approvalID,omitempty"`
	// Error 是失败原因（State=Failed 时）。
	Error string `json:"error,omitempty"`
	// CreatedAt/UpdatedAt 时间戳。
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RegisterWait 是等待自注册的约束：超时与判定谓词。
type RegisterWait struct {
	// Timeout 等待注册的超时；0 用默认（5m）。
	Timeout time.Duration
	// ExpectedAgentIDs 由 apply 输出推导出的预期 agent id（可选）。
	ExpectedAgentIDs []string
}
