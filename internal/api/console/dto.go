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
	// IdempotencyKey comes from the Idempotency-Key request header (read by the
	// server, not written into the request body by the browser): the same key maps
	// to only one real execution inside the idempotency window.
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
	// Replayed marks that this response is an idempotent replay (the same key already
	// completed execution and the result came from storage rather than a fresh
	// dispatch): the frontend uses it to tell apart "retried after an unknown
	// result and got the landed result" from a new execution.
	Replayed bool `json:"replayed,omitempty"`
}

// ConsoleDraftCompositeExecuteRequest is a draft preview execution request: the body
// has the same shape as the composite page preview endpoint (page key + section
// requests + template snapshot), plus the bindingId to execute and the renderer
// context. The server rebuilds the draft spec through the same generation chain
// as the creation entry point (section contract resolution, binding allocation,
// static interleaving, template snapshot, publish-grade selector validation) and
// only dispatches execution after the same set of execution gates passes; the
// browser does not submit functionId/routing/scope (the same rule as published
// page execution).
type ConsoleDraftCompositeExecuteRequest struct {
	PageKey   string                                `json:"pageKey" binding:"required"`
	BindingID string                                `json:"bindingId" binding:"required"`
	Sections  []contractsvc.CompositeSectionRequest `json:"sections"`

	// ComponentTemplates passes through the page-level template snapshot (same shape as
	// the preview/creation entry points).
	ComponentTemplates []spec.ComponentTemplateUsage `json:"componentTemplates,omitempty"`

	Context ConsoleBindingExecutionContext `json:"context"`

	// IdempotencyKey comes from the Idempotency-Key request header: draft preview
	// execution really dispatches the function, so it enjoys the same server-side
	// idempotency and safe-retry semantics.
	IdempotencyKey string `json:"-"`
}
