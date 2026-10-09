// Package execlog 统一执行审计（agent-core K3，与 healthprobe 同级三 agent
// 共用）。每个 agent 的每次执行都留审计记录：本地轮转文件为真值（离线不丢），
// Uploader 尽力上报 server 落审计存储；命令类执行走 audit-first 放行闸——
// 审计记录未持久化即拒绝执行（§6.2）。
package execlog

import "encoding/json"

// Status 执行结果状态。admitted 是 audit-first 闸的放行凭据（执行前落盘），
// success/failure 是执行完成后的结果记录。
type Status string

const (
	StatusAdmitted Status = "admitted"
	StatusSuccess  Status = "success"
	StatusFailure  Status = "failure"
)

// Record 一次执行的审计记录（§6.2 十一字段）。JSON 键为平台契约，
// lowerCamelCase；params 为入参全文（敏感字段沿用调用方脱敏规则）。
type Record struct {
	TaskID        string          `json:"taskId,omitempty"`
	Operator      string          `json:"operator"`
	AgentType     string          `json:"agentType"`
	AgentInstance string          `json:"agentInstance"`
	Action        string          `json:"action"`
	Params        json.RawMessage `json:"params,omitempty"`
	Status        Status          `json:"status"`
	Summary       string          `json:"summary,omitempty"`
	DurationMs    int64           `json:"durationMs"`
	TsUnixMs      int64           `json:"tsUnixMs"`
	GameID        string          `json:"gameId"`
	Env           string          `json:"env"`
}

// ScopeRequired 报告记录缺少强制 scope（game_id+env 与注册标签一致）。
func (r *Record) ScopeRequired() bool { return r.GameID != "" && r.Env != "" }
