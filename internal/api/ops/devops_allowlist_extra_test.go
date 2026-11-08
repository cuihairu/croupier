package ops

// devops_allowlist.go 补测：service 面与 capture gates 同款通道语义逐分支
// （发布递增、未发布空形态、scope 隔离、校验拦截、乐观锁冲突、回滚重发布
// 与 lint 演进拦截、版本历史、dry-run 预检），外加五个 gin handler 的往返
// 与坏载荷 400 面。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func validAllowlistContent() string {
	return `[{"host":"h1","ports":[22],"protocols":["ssh"]}]`
}

func TestDevopsAllowlistPutCreatesIncrementingVersions(t *testing.T) {
	s, ctx := newCaptureGatesService(t)

	resp, err := s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: validAllowlistContent(), Message: "first"})
	require.NoError(t, err)
	require.Equal(t, 1, resp.Version)

	resp2, err := s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: validAllowlistContent()})
	require.NoError(t, err)
	require.Equal(t, 2, resp2.Version)

	got, err := s.DevopsAllowlistGet(ctx)
	require.NoError(t, err)
	require.Equal(t, "devops.allowlist", got.Key)
	require.Equal(t, 2, got.Version)
	require.Contains(t, got.Content, "h1")
	require.Equal(t, "ops", got.Namespace)
	require.Equal(t, "system", got.CreatedBy)
	require.NotEmpty(t, got.UpdatedAt)
}

func TestDevopsAllowlistGetEmptyWhenUnpublished(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	got, err := s.DevopsAllowlistGet(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, got.Version)
	require.Empty(t, got.Content)
	require.Equal(t, "devops.allowlist", got.Key)
}

func TestDevopsAllowlistGetIsScopeIsolated(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	_, err := s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: validAllowlistContent()})
	require.NoError(t, err)

	other := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})
	got, err := s.DevopsAllowlistGet(other)
	require.NoError(t, err)
	require.Equal(t, 0, got.Version, "other scope must not see demo/prod publish")
}

func TestDevopsAllowlistPutValidation(t *testing.T) {
	s, ctx := newCaptureGatesService(t)

	// 空内容 / 坏 JSON / 缺 host
	_, err := s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: "  "})
	require.ErrorContains(t, err, "content is required")
	_, err = s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: "{bad"})
	require.Error(t, err)
	_, err = s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: `[{"ports":[22]}]`})
	require.ErrorContains(t, err, "host is required")

	// 无 scope → 拒绝发布
	_, err = s.DevopsAllowlistPut(context.Background(), &DevopsAllowlistPutRequest{Content: validAllowlistContent()})
	require.ErrorContains(t, err, "require game and env scope")

	// 乐观锁：发布 v2 后再带过期基线 baseVersion=1 → 409
	_, err = s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: validAllowlistContent()})
	require.NoError(t, err)
	_, err = s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: validAllowlistContent(), BaseVersion: 1})
	require.NoError(t, err, "current base version must pass")
	_, err = s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: validAllowlistContent(), BaseVersion: 1})
	require.Error(t, err)
	require.Equal(t, 409, httpCode(err), "stale baseVersion must conflict")
}

func TestDevopsAllowlistRollbackRepublishesHistoricalContent(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	v1 := `[{"host":"v1-host","ports":[2222],"protocols":["ssh"]}]`
	_, err := s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: v1})
	require.NoError(t, err)
	_, err = s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: validAllowlistContent()})
	require.NoError(t, err)

	resp, err := s.DevopsAllowlistRollback(ctx, &DevopsAllowlistRollbackRequest{Version: 1})
	require.NoError(t, err)
	require.Equal(t, 3, resp.Version)

	got, err := s.DevopsAllowlistGet(ctx)
	require.NoError(t, err)
	require.Contains(t, got.Content, "v1-host", "rollback must republish historical content")

	items, err := s.DevopsAllowlistVersionsGet(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Contains(t, items[0].Message, "rollback from version 1")
}

func TestDevopsAllowlistRollbackValidation(t *testing.T) {
	s, ctx := newCaptureGatesService(t)

	// 版本号缺失 / 无 scope
	_, err := s.DevopsAllowlistRollback(ctx, &DevopsAllowlistRollbackRequest{})
	require.ErrorContains(t, err, "rollback version is required")
	_, err = s.DevopsAllowlistRollback(context.Background(), &DevopsAllowlistRollbackRequest{Version: 1})
	require.ErrorContains(t, err, "require game and env scope")

	// 未知版本 → 404
	_, err = s.DevopsAllowlistRollback(ctx, &DevopsAllowlistRollbackRequest{Version: 9})
	require.Error(t, err)
	require.Equal(t, 404, httpCode(err))

	// lint 规则演进后的存量坏行：直插 model 绕过服务层 lint → 回滚被当前校验拦下
	_, err = s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "devops.allowlist", Content: `{"reconcile":`, Format: "json",
		GameID: "demo", Env: "prod", Namespace: model.ConfigNamespaceOps,
	}, "legacy")
	require.NoError(t, err)
	_, err = s.DevopsAllowlistRollback(ctx, &DevopsAllowlistRollbackRequest{Version: 1})
	require.Error(t, err)
	require.Equal(t, 422, httpCode(err))
}

func TestDevopsAllowlistVersionsList(t *testing.T) {
	s, ctx := newCaptureGatesService(t)

	// 未发布 → 空数组而非 404
	items, err := s.DevopsAllowlistVersionsGet(ctx)
	require.NoError(t, err)
	require.Len(t, items, 0)

	// 无 scope → 校验错误
	_, err = s.DevopsAllowlistVersionsGet(context.Background())
	require.ErrorContains(t, err, "require game and env scope")

	_, err = s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: validAllowlistContent(), Message: "m1"})
	require.NoError(t, err)
	_, err = s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: validAllowlistContent(), Message: "m2"})
	require.NoError(t, err)
	items, err = s.DevopsAllowlistVersionsGet(ctx)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, 2, items[0].Version, "history must be newest first")
	require.Equal(t, "m2", items[0].Message)
	require.Equal(t, "m1", items[1].Message)
}

func TestDevopsAllowlistValidateDryRun(t *testing.T) {
	s, _ := newCaptureGatesService(t)

	resp, err := s.DevopsAllowlistValidate(&DevopsAllowlistValidateRequest{Content: validAllowlistContent()})
	require.NoError(t, err)
	require.True(t, resp.Valid)
	require.Equal(t, 1, resp.Entries)
	require.Equal(t, 1, resp.Ports)

	// 多主机多端口汇总
	resp, err = s.DevopsAllowlistValidate(&DevopsAllowlistValidateRequest{
		Content: `[{"host":"a","ports":[22,2222],"protocols":["ssh"]},{"host":"b","ports":[80],"protocols":["vnc"]}]`,
	})
	require.NoError(t, err)
	require.Equal(t, 2, resp.Entries)
	require.Equal(t, 3, resp.Ports)

	// nil / 空 / 坏 JSON / 缺 host
	_, err = s.DevopsAllowlistValidate(nil)
	require.ErrorContains(t, err, "content is required")
	_, err = s.DevopsAllowlistValidate(&DevopsAllowlistValidateRequest{Content: " \t"})
	require.ErrorContains(t, err, "content is required")
	_, err = s.DevopsAllowlistValidate(&DevopsAllowlistValidateRequest{Content: "not-json"})
	require.Error(t, err)
	_, err = s.DevopsAllowlistValidate(&DevopsAllowlistValidateRequest{Content: `[{"host":"","ports":[1]}]`})
	require.ErrorContains(t, err, "host is required")
}

// withScopeRequest 挂 game/env scope 到测试请求上下文。
func withScopeRequest(t *testing.T, method, target string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	req = req.WithContext(svc.WithGameScope(req.Context(), svc.GameScope{GameID: "demo", Env: "prod"}))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func TestDevopsAllowlistHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbSvc, _ := newCaptureGatesService(t)
	h := NewHandler(dbSvc)

	// GET 未发布 → 空形态
	c, w := withScopeRequest(t, http.MethodGet, "/ops/devops/allowlist", "")
	h.DevopsAllowlistGet(c)
	require.Equal(t, http.StatusOK, w.Code)
	var got DevopsAllowlistGetResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "devops.allowlist", got.Key)

	// PUT 坏 JSON → 400
	c, w = withScopeRequest(t, http.MethodPut, "/ops/devops/allowlist", "{bad")
	h.DevopsAllowlistPut(c)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// PUT 发布 → 200 + 版本号
	c, w = withScopeRequest(t, http.MethodPut, "/ops/devops/allowlist",
		`{"content":"[{\"host\":\"h1\",\"ports\":[22],\"protocols\":[\"ssh\"]}]"}`)
	h.DevopsAllowlistPut(c)
	require.Equal(t, http.StatusOK, w.Code)
	var put DevopsAllowlistPutResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &put))
	require.Equal(t, 1, put.Version)

	// Validate 坏 JSON → 400；合法 → 200
	c, w = withScopeRequest(t, http.MethodPost, "/ops/devops/allowlist/validate", "{bad")
	h.DevopsAllowlistValidate(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	c, w = withScopeRequest(t, http.MethodPost, "/ops/devops/allowlist/validate",
		`{"content":"[{\"host\":\"h1\",\"ports\":[22],\"protocols\":[\"ssh\"]}]"}`)
	h.DevopsAllowlistValidate(c)
	require.Equal(t, http.StatusOK, w.Code)

	// Rollback 坏 JSON → 400；合法 → 200
	c, w = withScopeRequest(t, http.MethodPost, "/ops/devops/allowlist/rollback", "{bad")
	h.DevopsAllowlistRollback(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	c, w = withScopeRequest(t, http.MethodPost, "/ops/devops/allowlist/rollback", `{"version":1}`)
	h.DevopsAllowlistRollback(c)
	require.Equal(t, http.StatusOK, w.Code)

	// Versions → 200
	c, w = withScopeRequest(t, http.MethodGet, "/ops/devops/allowlist/versions", "")
	h.DevopsAllowlistVersionsGet(c)
	require.Equal(t, http.StatusOK, w.Code)
	var versions []DevopsAllowlistVersionItem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &versions))
	require.Len(t, versions, 2)
}
