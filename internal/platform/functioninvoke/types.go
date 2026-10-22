// Package functioninvoke 承载函数调用的唯一生产执行链。scope、禁用、分配、
// 策略审批、审计与执行留痕六面在 Invoke 内一次性收口；HTTP 入口
// （internal/api/function）与 logic 侧薄壳（internal/logic/function）都只做
// DTO 适配后转发到这里，不允许再各自维护旁路执行实现。
package functioninvoke

import "encoding/json"

// Request 是统一执行链的入参。两个入口各自把自身 DTO 映射为本结构后转发。
type Request struct {
	ID              string          `json:"id"`
	Params          json.RawMessage `json:"params,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	GameID          string          `json:"gameId"`
	Env             string          `json:"env"`
	Mode            string          `json:"mode"`
	Route           string          `json:"route"`
	TargetServiceID string          `json:"targetServiceId"`
	HashKey         string          `json:"hashKey"`
	// TimeoutMs 声明本次同步调用的超时预算（毫秒，[1000,60000] 越界
	// clamp）。仅 sync 模式有意义；async 任务有自己的生命周期。
	TimeoutMs int               `json:"timeoutMs,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Response 是统一执行链的出参。
type Response struct {
	TaskId           string          `json:"taskId"`
	Result           json.RawMessage `json:"result,omitempty"`
	ApprovalID       string          `json:"approvalId,omitempty"`
	ApprovalRequired bool            `json:"approvalRequired,omitempty"`
	ApprovalWorkflow string          `json:"approvalWorkflow,omitempty"`
	// TraceID identifies the OTel trace of this invocation so dashboards can
	// jump to Jaeger/Grafana for the full request path.
	TraceID string `json:"traceId,omitempty"`
	// ExecutionMetadata is server-internal dispatch context for audit/tracing.
	// It is intentionally not serialized to API clients.
	ExecutionMetadata map[string]string `json:"-"`
	// Broadcast carries per-agent outcomes when route=broadcast. Empty when
	// the call did not use broadcast routing.
	Broadcast *BroadcastResult `json:"broadcast,omitempty"`
}

// BroadcastResult aggregates per-agent outcomes from a broadcast invocation.
type BroadcastResult struct {
	Total   int                  `json:"total"`
	Success int                  `json:"success"`
	Failure int                  `json:"failure"`
	Results []BroadcastAgentItem `json:"results,omitempty"`
}

// BroadcastAgentItem captures one agent's outcome in a broadcast invocation.
type BroadcastAgentItem struct {
	AgentID string          `json:"agentId"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}
