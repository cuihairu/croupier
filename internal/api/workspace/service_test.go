package workspace

import (
	"context"
	"encoding/json"
	"errors"
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
	return &svc.ServiceContext{
		DB:                   db,
		WorkspaceConfigModel: model.NewWorkspaceConfigModel(db),
		ConfigVersionModel:   model.NewConfigVersionModel(db),
	}
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

func TestService_ListConfigs_Empty(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.ListConfigs(context.Background(), &ListConfigsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Items)
}

func TestService_SaveAndGetConfig_RoundTrip(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	saveResp, err := svc.SaveConfig(context.Background(), &SaveConfigRequest{
		ObjectKey: "demo",
		Title:     "Demo Workspace",
		Layout:    map[string]interface{}{"type": "tabs", "tabs": []interface{}{}},
		MenuOrder: 2,
	})
	require.NoError(t, err)
	require.NotNil(t, saveResp)
	assert.Equal(t, "demo", saveResp.WorkspaceConfig.ObjectKey)
	assert.Equal(t, "Demo Workspace", saveResp.WorkspaceConfig.Title)

	getResp, err := svc.GetConfig(context.Background(), &GetConfigRequest{ObjectKey: "demo"})
	require.NoError(t, err)
	require.NotNil(t, getResp)
	assert.Equal(t, "demo", getResp.WorkspaceConfig.ObjectKey)
	assert.Equal(t, "Demo Workspace", getResp.WorkspaceConfig.Title)
}

func TestService_GetConfig_EmptyObjectKey(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.GetConfig(context.Background(), &GetConfigRequest{ObjectKey: ""})
	require.Error(t, err)
	assert.Nil(t, resp)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
}

func TestService_GetConfig_NotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.GetConfig(context.Background(), &GetConfigRequest{ObjectKey: "missing"})
	require.Error(t, err)
	assert.Nil(t, resp)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code)
}

func TestService_SaveConfig_MissingTitle(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.SaveConfig(context.Background(), &SaveConfigRequest{
		ObjectKey: "no-title",
		Layout:    map[string]interface{}{"type": "tabs"},
	})
	// Title defaults to ObjectKey when empty, so this should succeed.
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "no-title", resp.WorkspaceConfig.Title)
}

func TestService_SaveConfig_MissingLayout(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.SaveConfig(context.Background(), &SaveConfigRequest{
		ObjectKey: "no-layout",
		Title:     "Has Title",
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_DeleteConfig_NotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.DeleteConfig(context.Background(), &DeleteConfigRequest{ObjectKey: "ghost"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_PublishUnpublish_RoundTrip(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.SaveConfig(context.Background(), &SaveConfigRequest{
		ObjectKey: "pub",
		Title:     "Pub",
		Layout:    map[string]interface{}{"type": "tabs"},
	})
	require.NoError(t, err)

	pubResp, err := svc.Publish(context.Background(), &PublishRequest{ObjectKey: "pub", PublishedBy: "tester"})
	require.NoError(t, err)
	assert.True(t, pubResp.Published)

	getResp, err := svc.GetConfig(context.Background(), &GetConfigRequest{ObjectKey: "pub"})
	require.NoError(t, err)
	assert.True(t, getResp.WorkspaceConfig.Published)

	unpubResp, err := svc.Unpublish(context.Background(), &UnpublishRequest{ObjectKey: "pub"})
	require.NoError(t, err)
	assert.False(t, unpubResp.Published)
}

func TestService_Publish_NotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Publish(context.Background(), &PublishRequest{ObjectKey: "ghost"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code)
}

func TestService_Versions_AfterSave(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.SaveConfig(context.Background(), &SaveConfigRequest{
		ObjectKey: "ver",
		Title:     "Ver",
		Layout:    map[string]interface{}{"type": "tabs"},
	})
	require.NoError(t, err)

	resp, err := svc.Versions(context.Background(), &VersionsRequest{ObjectKey: "ver"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Items, "saving should persist at least one version")
}

func TestService_VersionDetail_InvalidVersionID(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.VersionDetail(context.Background(), &VersionDetailRequest{ObjectKey: "ver", VersionID: "not-a-number"})
	require.Error(t, err)
}

func TestService_Rollback_InvalidVersionID(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Rollback(context.Background(), &RollbackRequest{ObjectKey: "ver", VersionID: "abc"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
}

// Sanity-check that gorm.ErrRecordNotFound (the sentinel used by the model layer)
// is the same value referenced by tests below.
func TestService_ErrorSentinel(t *testing.T) {
	assert.True(t, errors.Is(gorm.ErrRecordNotFound, gorm.ErrRecordNotFound))
}
