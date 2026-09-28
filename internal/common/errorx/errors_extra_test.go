package errorx

import (
	"net/http"
	"testing"
)

func TestErrorCodeFallsBackToMap(t *testing.T) {
	e := NewBadRequest("bad")
	if got := e.ErrorCode(); got != "bad_request" {
		t.Fatalf("ErrorCode() = %q, want bad_request", got)
	}
}

func TestErrorCodeStableWins(t *testing.T) {
	e := NewConflictWithCode("conflict_state", "状态冲突", map[string]any{"id": 1})
	if got := e.ErrorCode(); got != "conflict_state" {
		t.Fatalf("ErrorCode() = %q, want conflict_state", got)
	}
	if e.Code != http.StatusConflict {
		t.Fatalf("Code = %d, want %d", e.Code, http.StatusConflict)
	}
}

func TestNewForbiddenWithCode(t *testing.T) {
	details := map[string]any{"role": "viewer"}
	e := NewForbiddenWithCode("role_denied", "角色不允许该操作", details)
	if e.Code != http.StatusForbidden {
		t.Fatalf("Code = %d, want %d", e.Code, http.StatusForbidden)
	}
	if got := e.ErrorCode(); got != "role_denied" {
		t.Fatalf("ErrorCode() = %q, want role_denied", got)
	}
	if e.Message != "角色不允许该操作" {
		t.Fatalf("Message = %q", e.Message)
	}
	if e.Details["role"] != "viewer" {
		t.Fatalf("Details = %v, want role=viewer", e.Details)
	}
}
