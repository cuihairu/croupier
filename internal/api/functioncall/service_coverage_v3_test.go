package functioncall

import (
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	taskapi "github.com/cuihairu/croupier/internal/api/task"
)

// TestService_List_DefaultPage tests that Page<=0 defaults to 1 and PageSize<=0 defaults to 20.
func TestService_List_DefaultPage(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "def-1", "player.ban", "succeeded")
	s := NewService(svcCtx)

	resp, err := s.List(t.Context(), &ListRequest{Page: 0, PageSize: 0})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
	assert.Equal(t, 1, resp.Total)
}

// TestService_List_NegativePage tests that negative page defaults to 1.
func TestService_List_NegativePage(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)

	resp, err := s.List(t.Context(), &ListRequest{Page: -5, PageSize: -10})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
}

// TestService_Stats_AllStatuses tests all status branches in Stats.
func TestService_Stats_AllStatuses(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "st-1", "fn", "succeeded")
	seedTaskRun(t, svcCtx.DB, "st-2", "fn", "failed")
	seedTaskRun(t, svcCtx.DB, "st-3", "fn", "running")
	seedTaskRun(t, svcCtx.DB, "st-4", "fn", "cancelled")
	seedTaskRun(t, svcCtx.DB, "st-5", "fn", "canceled")
	seedTaskRun(t, svcCtx.DB, "st-6", "fn", "timeout")
	seedTaskRun(t, svcCtx.DB, "st-7", "fn", "something_else")
	s := NewService(svcCtx)

	resp, err := s.Stats(t.Context(), &ListRequest{Page: 1, PageSize: 50})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 7, resp.Total)
	assert.GreaterOrEqual(t, resp.Succeeded, 1)
	assert.GreaterOrEqual(t, resp.Failed, 1)
	assert.GreaterOrEqual(t, resp.Running, 1)
	assert.GreaterOrEqual(t, resp.Cancelled, 2) // "cancelled" and "canceled"
	assert.GreaterOrEqual(t, resp.Timeout, 1)
	assert.GreaterOrEqual(t, resp.Other, 1) // "something_else"
}

// TestService_Stats_Empty tests stats with no data.
func TestService_Stats_Empty(t *testing.T) {
	s := NewService(setupSvcCtx(t))

	resp, err := s.Stats(t.Context(), &ListRequest{Page: 1, PageSize: 50})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total)
	assert.Equal(t, 0, resp.Succeeded)
	assert.Equal(t, 0, resp.Failed)
}

// TestService_Stats_UnknownStatus tests the default/other branch for unknown statuses.
func TestService_Stats_UnknownStatus(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "unk-1", "fn", "")
	s := NewService(svcCtx)

	resp, err := s.Stats(t.Context(), &ListRequest{Page: 1, PageSize: 50})
	require.NoError(t, err)
	// Empty status in fromTask becomes "unknown", which should fall into the "other" branch
	assert.Equal(t, 1, resp.Other)
}

// TestService_List_FilterByStatus tests filtering by status.
func TestService_List_FilterByStatus(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "fs-1", "player.ban", "succeeded")
	seedTaskRun(t, svcCtx.DB, "fs-2", "player.ban", "failed")
	s := NewService(svcCtx)

	resp, err := s.List(t.Context(), &ListRequest{Status: "succeeded", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Calls))
	assert.Equal(t, "succeeded", resp.Calls[0].Status)
}

// TestService_List_FilterByGameIDEnv tests filtering by game_id and env.
func TestService_List_FilterByGameIDEnv(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "ge-1", "player.ban", "succeeded")
	s := NewService(svcCtx)

	resp, err := s.List(t.Context(), &ListRequest{GameID: "game1", Env: "prod", Page: 1, PageSize: 10})
	require.NoError(t, err)
	// The task was created with default empty game_id/env, so filtering for game1 should return 0
	assert.Equal(t, 0, resp.Total)
}

// TestService_Detail_EmptyID_V3 tests that empty/whitespace ID returns BadRequest.
func TestService_Detail_EmptyID_V3(t *testing.T) {
	s := NewService(setupSvcCtx(t))

	resp, err := s.Detail(t.Context(), &DetailRequest{ID: ""})
	require.Error(t, err)
	assert.Nil(t, resp)

	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 400, codeErr.Code)
}

// TestService_Cancel_WhitespaceID tests Cancel with whitespace ID.
func TestService_Cancel_WhitespaceID(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)

	err := s.Cancel(t.Context(), &DetailRequest{ID: "   "})
	// Empty ID causes an error from the task service validation
	require.Error(t, err)
}

// TestFromTask_AllFields tests that fromTask correctly maps all fields.
func TestFromTask_AllFields(t *testing.T) {
	input := taskapi.Item{
		ID:         "t-99",
		FunctionID: "player.list",
		Status:     "succeeded",
		GameID:     "game1",
		Env:        "prod",
		AgentID:    "agent-1",
		StartedAt:  "2024-01-01T00:00:00Z",
		FinishedAt: "2024-01-01T00:00:01Z",
		Error:      "some error",
		CreatedAt:  "2024-01-01T00:00:00Z",
	}

	result := fromTask(input)
	assert.Equal(t, "t-99", result.ID)
	assert.Equal(t, "t-99", result.TaskID)
	assert.Equal(t, "player.list", result.FunctionID)
	assert.Equal(t, "succeeded", result.Status)
	assert.Equal(t, "game1", result.GameID)
	assert.Equal(t, "prod", result.Env)
	assert.Equal(t, "agent-1", result.AgentID)
	assert.Equal(t, "2024-01-01T00:00:00Z", result.StartedAt)
	assert.Equal(t, "2024-01-01T00:00:01Z", result.FinishedAt)
	assert.Equal(t, "some error", result.ErrorMsg)
	assert.Equal(t, "2024-01-01T00:00:00Z", result.CreatedAt)
}

// TestService_Rerun_WithPayload tests Rerun with payload (still returns error).
func TestService_Rerun_WithPayload(t *testing.T) {
	s := NewService(setupSvcCtx(t))

	resp, err := s.Rerun(t.Context(), &RerunRequest{
		ID:      "any",
		Payload: map[string]string{"key": "value"},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// TestHandler_Rerun_EmptyContentLength tests Rerun with no body (ContentLength == 0).
func TestHandler_Rerun_EmptyContentLength(t *testing.T) {
	t.Setenv("GIN_MODE", "test")
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	handler := NewHandler(s)
	router := newRouter(handler)

	rec := doReq(t, router, "POST", "/calls/some-id/rerun", "")
	// Empty body means ContentLength == 0, so ShouldBindJSON is skipped; Rerun returns 400
	assert.Equal(t, 400, rec.Code)
	assertErrorShape(t, rec)
}

// TestHandler_Cancel_OnMissingTask tests Cancel on a task that doesn't exist.
func TestHandler_Cancel_OnMissingTask(t *testing.T) {
	t.Setenv("GIN_MODE", "test")
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	handler := NewHandler(s)
	router := newRouter(handler)

	rec := doReq(t, router, "POST", "/calls/nonexistent/cancel", "")
	// Cancel on nonexistent task returns 404
	assert.Equal(t, 404, rec.Code)
}

// TestService_List_WithEnvContext tests ResolveEnv/ResolveGameID with context.
func TestService_List_WithEnvContext(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "ctx-1", "player.ban", "succeeded")
	s := NewService(svcCtx)

	// Empty game_id/env should resolve from context (which is empty in test)
	resp, err := s.List(t.Context(), &ListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
}

// TestService_Stats_WithFilters tests Stats with filter parameters.
func TestService_Stats_WithFilters(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "sf-1", "player.ban", "succeeded")
	seedTaskRun(t, svcCtx.DB, "sf-2", "mail.send", "failed")
	s := NewService(svcCtx)

	resp, err := s.Stats(t.Context(), &ListRequest{FunctionID: "player.ban", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, 1, resp.Succeeded)
}

// TestHandler_List_BindQueryError tests handler List with invalid query params.
func TestHandler_List_BindQueryError(t *testing.T) {
	t.Setenv("GIN_MODE", "test")
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	handler := NewHandler(s)
	router := newRouter(handler)

	// pageSize=abc is not a valid int, should cause bind error
	rec := doReq(t, router, "GET", "/calls?page=1&pageSize=abc", "")
	assert.NotEqual(t, 200, rec.Code)
}

// TestHandler_Stats_BindQueryError tests handler Stats with invalid query params.
func TestHandler_Stats_BindQueryError(t *testing.T) {
	t.Setenv("GIN_MODE", "test")
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	handler := NewHandler(s)
	router := newRouter(handler)

	// page=abc is not a valid int
	rec := doReq(t, router, "GET", "/calls/stats?page=abc", "")
	assert.NotEqual(t, 200, rec.Code)
}
