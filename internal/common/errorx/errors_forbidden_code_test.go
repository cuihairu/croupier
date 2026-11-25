package errorx

import (
	"net/http"
	"testing"
)

// NewForbiddenWithCode previously had 0% coverage: the 403 constructor
// carrying a stable business code (the contract field StableCode lets the
// frontend branch on the error code instead of relying on the localized
// message).
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
