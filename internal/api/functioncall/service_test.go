package functioncall

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func setupSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db := setupTestDB(t)
	return &svc.ServiceContext{DB: db}
}

func seedTaskRun(t *testing.T, db *gorm.DB, taskID, functionID, status string) *model.TaskRun {
	t.Helper()
	run := &model.TaskRun{
		TaskID:       taskID,
		FunctionID:   functionID,
		Status:       status,
		InputPayload: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, db.Create(run).Error)
	return run
}

func newTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected status %d, got %d body=%s", want, rec.Code, rec.Body.String())
	}
}

func assertErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "body not JSON object: %s", rec.Body.String())
	errCode, _ := body["error"].(string)
	require.NotEmpty(t, errCode, "missing 'error' field, body=%s", rec.Body.String())
	msg, _ := body["message"].(string)
	require.NotEmpty(t, msg, "missing 'message' field, body=%s", rec.Body.String())
}

func TestService_List_Empty(t *testing.T) {
	svc := NewService(setupSvcCtx(t))

	resp, err := svc.List(context.Background(), &ListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Calls)
	assert.Equal(t, 0, resp.Total)
}

func TestService_List_ReturnsSeededCalls(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-1", "player.ban", "succeeded")
	seedTaskRun(t, svcCtx.DB, "t-2", "player.kick", "running")
	svc := NewService(svcCtx)

	resp, err := svc.List(context.Background(), &ListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 2, len(resp.Calls))
}

func TestService_Detail_EmptyID(t *testing.T) {
	svc := NewService(setupSvcCtx(t))

	resp, err := svc.Detail(context.Background(), &DetailRequest{ID: "  "})
	require.Error(t, err)
	assert.Nil(t, resp)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
}

func TestService_Detail_NotFound(t *testing.T) {
	svc := NewService(setupSvcCtx(t))

	resp, err := svc.Detail(context.Background(), &DetailRequest{ID: "missing"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Detail_Found(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-found", "player.ban", "running")
	svc := NewService(svcCtx)

	resp, err := svc.Detail(context.Background(), &DetailRequest{ID: "t-found"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "t-found", resp.ID)
	assert.Equal(t, "t-found", resp.TaskID)
	assert.Equal(t, "running", resp.Status)
}

func TestService_Cancel_OnExistingTask(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-cancel", "player.ban", "running")
	svc := NewService(svcCtx)

	require.NoError(t, svc.Cancel(context.Background(), &DetailRequest{ID: "t-cancel"}))
}

func TestService_Stats_OK(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "s-1", "fn", "succeeded")
	seedTaskRun(t, svcCtx.DB, "s-2", "fn", "failed")
	seedTaskRun(t, svcCtx.DB, "s-3", "fn", "running")
	svc := NewService(svcCtx)

	resp, err := svc.Stats(context.Background(), &ListRequest{Page: 1, PageSize: 50})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 3, resp.Total)
	assert.GreaterOrEqual(t, resp.Succeeded, 1)
	assert.GreaterOrEqual(t, resp.Failed, 1)
	assert.GreaterOrEqual(t, resp.Running, 1)
}

func TestService_Rerun_AlwaysReturnsError(t *testing.T) {
	svc := NewService(setupSvcCtx(t))

	resp, err := svc.Rerun(context.Background(), &RerunRequest{ID: "any"})
	require.Error(t, err)
	assert.Nil(t, resp)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
}

func TestService_List_FiltersByFunction(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "f-1", "player.ban", "succeeded")
	seedTaskRun(t, svcCtx.DB, "f-2", "player.kick", "running")
	svc := NewService(svcCtx)

	resp, err := svc.List(context.Background(), &ListRequest{FunctionID: "player.ban", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, len(resp.Calls))
	assert.Equal(t, "player.ban", resp.Calls[0].FunctionID)
}
