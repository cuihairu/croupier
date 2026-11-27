package croupier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// AUD-069: a pending-approval response must be preserved as a recognizable
// outcome rather than mis-decoded as a missing result / missing task ID.
func TestHTTPInvokerPreservesPendingApproval(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		body    interface{}
		call    func(*httpInvoker) error
		wantID  string
		wantWf  string
		wantSta string
	}{
		{
			name: "invoke approvalRequired",
			path: "/api/v1/functions/player.ban/invoke",
			body: map[string]interface{}{
				"taskId": "", "approvalId": "apr-1", "approvalRequired": true,
				"approvalWorkflow": "two_person", "traceId": "tr-1",
			},
			call: func(i *httpInvoker) error {
				_, err := i.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{})
				return err
			},
			wantID: "apr-1", wantWf: "two_person",
		},
		{
			name: "task start status approval_required",
			path: "/api/v1/tasks",
			body: map[string]interface{}{
				"taskId": "", "status": "approval_required", "approvalId": "apr-2",
				"approvalWorkflow": "single_admin",
			},
			call: func(i *httpInvoker) error {
				_, err := i.StartTask(context.Background(), "player.ban", `{}`, InvokeOptions{})
				return err
			},
			wantID: "apr-2", wantWf: "single_admin", wantSta: "approval_required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.path {
					t.Errorf("unexpected path %q", r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(tt.body)
			}))
			defer server.Close()
			inv := NewHTTPInvoker(&InvokerConfig{Address: server.URL})
			err := tt.call(inv.(*httpInvoker))
			if !IsApprovalRequired(err) {
				t.Fatalf("want ApprovalRequiredError, got %v", err)
			}
			pending := err.(*ApprovalRequiredError)
			if pending.ApprovalID != tt.wantID || pending.ApprovalWorkflow != tt.wantWf || pending.Status != tt.wantSta {
				t.Fatalf("unexpected pending approval: %+v", pending)
			}
			if IsResultUnknown(err) {
				t.Fatal("pending approval must not be marked result-unknown")
			}
		})
	}
}

func TestHTTPInvokerMissingApprovalIDStaysMissingOutcome(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"taskId": "", "status": "approval_required", "approvalWorkflow": "single_admin",
		})
	}))
	defer server.Close()
	inv := NewHTTPInvoker(&InvokerConfig{Address: server.URL})
	_, err := inv.StartTask(context.Background(), "player.ban", `{}`, InvokeOptions{})
	if err == nil || IsApprovalRequired(err) {
		t.Fatalf("want ordinary missing-taskId error, got %v", err)
	}
}
