package workspace

import (
	"net/http"
	"testing"
)

func TestMapWorkspaceErrorCode(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		rawCode   string
		operation string
		expected  string
	}{
		{
			name:      "rollback not found",
			status:    http.StatusNotFound,
			rawCode:   "not_found",
			operation: "rollback",
			expected:  "workspace_version_not_found",
		},
		{
			name:      "versions detail not found",
			status:    http.StatusNotFound,
			rawCode:   "not_found",
			operation: "versions_detail",
			expected:  "workspace_version_not_found",
		},
		{
			name:      "publish internal error",
			status:    http.StatusInternalServerError,
			rawCode:   "internal_error",
			operation: "publish",
			expected:  "workspace_publish_failed",
		},
		{
			name:      "forbidden",
			status:    http.StatusForbidden,
			rawCode:   "forbidden",
			operation: "save",
			expected:  "forbidden",
		},
		{
			name:      "invalid config",
			status:    http.StatusBadRequest,
			rawCode:   "bad_request",
			operation: "save",
			expected:  "workspace_invalid_config",
		},
		{
			name:      "not found config",
			status:    http.StatusNotFound,
			rawCode:   "not_found",
			operation: "get",
			expected:  "workspace_not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapWorkspaceErrorCode(tt.status, tt.rawCode, tt.operation)
			if got != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}
