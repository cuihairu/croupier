package console

import (
	"encoding/json"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
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
	PageKey   string          `uri:"pageKey" binding:"required"`
	BindingID string          `uri:"bindingId" binding:"required"`
	Payload   json.RawMessage `json:"payload"`
}

type ConsoleExecuteBindingResponse struct {
	Result spec.PageExecutionResult `json:"result"`
}
