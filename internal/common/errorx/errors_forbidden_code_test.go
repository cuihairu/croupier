package errorx

import (
	"net/http"
	"testing"
)

// NewForbiddenWithCode 此前 0%：带稳定业务码的 403 构造器（契约字段
// StableCode 供前端按 error 码分支，避免依赖本地化 message）。
func TestNewForbiddenWithCode(t *testing.T) {
	details := map[string]any{"requiredScope": "ops:write"}
	err := NewForbiddenWithCode("approval_required", "需要审批", details)

	if err.Code != http.StatusForbidden {
		t.Errorf("Wrong code: got %d, want %d", err.Code, http.StatusForbidden)
	}
	if err.StableCode != "approval_required" {
		t.Errorf("Wrong stable code: got %q, want %q", err.StableCode, "approval_required")
	}
	if err.Message != "需要审批" {
		t.Errorf("Wrong message: got %q", err.Message)
	}
	if err.Details["requiredScope"] != "ops:write" {
		t.Errorf("Details not passed through: got %v", err.Details)
	}
	if err.Error() != "需要审批" {
		t.Errorf("Error() returned wrong message: got %q", err.Error())
	}
}
