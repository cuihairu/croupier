package console

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/service/notify"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingValidator forces ShouldBindQuery to fail at the validation stage so
// the handler binding-error branches become reachable (the request DTOs only
// carry plain string form fields, which cannot fail mapForm otherwise).
type failingValidator struct{}

func (failingValidator) ValidateStruct(any) error { return errors.New("forced bind failure") }

func (failingValidator) Engine() any { return nil }

func withFailingValidator(t *testing.T) {
	t.Helper()
	original := binding.Validator
	binding.Validator = failingValidator{}
	t.Cleanup(func() { binding.Validator = original })
}

// TestMenuHandlerBindError covers the query-binding failure branch of
// Handler.Menu.
func TestMenuHandlerBindError(t *testing.T) {
	withFailingValidator(t)
	service, _ := newConsoleTestService(t, "console:read")
	handler := NewHandler(service)

	ctx, rec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/menu", ""))
	handler.Menu(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "forced bind failure")
}

// TestPagesHandlerBindError covers the query-binding failure branch of
// Handler.Pages (same forced validation failure as Menu).
func TestPagesHandlerBindError(t *testing.T) {
	withFailingValidator(t)
	service, _ := newConsoleTestService(t, "console:read")
	handler := NewHandler(service)

	ctx, rec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/pages", ""))
	handler.Pages(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "forced bind failure")
}

// TestCreatePageApprovalWithNotifyService covers the recipient
// de-duplication branch: when the NotifyService is configured, the approval
// actor is appended to the recipients list unless already present.
func TestCreatePageApprovalWithNotifyService(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	service.svcCtx.ApprovalsStore = approvals.NewMemStore()
	service.svcCtx.NotifyService = notify.New(nil, nil)
	require.NotNil(t, service.svcCtx.NotifyService)

	approvalID, err := service.createPageApproval(ctx, "demo-game", "development",
		spec.PageFunctionBinding{ID: "b1", FunctionID: "player.query"},
		spec.BindingContractSnapshot{BindingID: "b1", FunctionID: "player.query"},
		json.RawMessage(`{"keyword":"a"}`), "", map[string]string{"traceId": "t-1"})
	require.NoError(t, err)
	assert.NotEmpty(t, approvalID)
}
