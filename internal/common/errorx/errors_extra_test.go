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
