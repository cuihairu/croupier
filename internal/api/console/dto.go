package console

import (
	"encoding/json"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	contractsvc "github.com/cuihairu/croupier/internal/service"
)

// ConsoleMenuRequest is the request for console menu.
type ConsoleMenuRequest struct {
	// Language is the preferred locale for menu labels.
	// If empty, uses system default.
	Language string `form:"lang"`
}

// ConsoleMenuResponse is the response with console menu.
type ConsoleMenuResponse struct {
	spec.ConsoleMenuSpec
}

// ConsolePagesRequest is the request to list published pages.
type ConsolePagesRequest struct {
	Category string `form:"category"`
}

// ConsolePagesResponse is the response with published pages.
type ConsolePagesResponse struct {
	Items []spec.PublishedPageSpec `json:"items"`
}

// ConsolePageRequest is the request to get a single published page.
type ConsolePageRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

// ConsolePageResponse is the response with a published page.
type ConsolePageResponse struct {
	Page spec.PublishedPageSpec `json:"page"`
}

type ConsoleExecuteBindingRequest struct {
	PageKey   string                         `uri:"pageKey" binding:"required"`
	BindingID string                         `uri:"bindingId" binding:"required"`
	Context   ConsoleBindingExecutionContext `json:"context"`
	// IdempotencyKey 来自 Idempotency-Key 请求头（服务端读头，不由浏览器写在
	// 请求体）：同一 key 在幂等窗口内只对应一次真实执行。
	IdempotencyKey string `json:"-"`
}

// ConsoleBindingExecutionContext contains renderer-owned source values for
// PageBinding typed selectors. It is not the function payload.
type ConsoleBindingExecutionContext struct {
	Form      json.RawMessage            `json:"form,omitempty"`
	Row       json.RawMessage            `json:"row,omitempty"`
	Selection json.RawMessage            `json:"selection,omitempty"`
	Detail    json.RawMessage            `json:"detail,omitempty"`
	PageState map[string]json.RawMessage `json:"pageState,omitempty"`
}

type ConsoleExecuteBindingResponse struct {
	Result spec.PageExecutionResult `json:"result"`
	// Replayed 标记本次返回是幂等重放（同 key 已完成执行、结果取自存储而非
	// 新下发）：前端据此区分「结果未知后重试拿到落地结果」与新执行。
	Replayed bool `json:"replayed,omitempty"`
}

// ConsoleDraftCompositeExecuteRequest 是草稿预览执行请求：请求体与组合页预览
// 端点同形（页面 key + 区块请求 + 模板快照），另带待执行的 bindingId 与渲染器
// 上下文。服务端按与创建入口同一生成链重建草稿 spec（区块契约解析、绑定分配、
// static 交错、模板快照、发布级 selector 校验）并通过同一套执行门禁后再下发
// 执行；浏览器不提交 functionId/路由/scope（与已发布页面执行同规）。
type ConsoleDraftCompositeExecuteRequest struct {
	PageKey   string                                `json:"pageKey" binding:"required"`
	BindingID string                                `json:"bindingId" binding:"required"`
	Sections  []contractsvc.CompositeSectionRequest `json:"sections"`

	// ComponentTemplates 页面级模板快照透传（与预览/创建入口同形）。
	ComponentTemplates []spec.ComponentTemplateUsage `json:"componentTemplates,omitempty"`

	Context ConsoleBindingExecutionContext `json:"context"`

	// IdempotencyKey 来自 Idempotency-Key 请求头：草稿预览执行会真实下发
	// 函数，同样享受服务端幂等与安全重试语义。
	IdempotencyKey string `json:"-"`
}
