package ops

// Extra coverage for devops_allowlist.go: every service-side branch with the same
// channel semantics as capture gates (publish increment, unpublished empty shape,
// scope isolation, validation interception, optimistic-lock conflict, rollback
// republish and lint-evolution interception, version history, dry-run precheck),
// plus the five gin handlers' round trips and the bad-payload 400 surface.

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

	// empty content / bad JSON / missing host
	_, err := s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: "  "})
	require.ErrorContains(t, err, "content is required")
	_, err = s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: "{bad"})
	require.Error(t, err)
	_, err = s.DevopsAllowlistPut(ctx, &DevopsAllowlistPutRequest{Content: `[{"ports":[22]}]`})
	require.ErrorContains(t, err, "host is required")

	// no scope → publish refused
	_, err = s.DevopsAllowlistPut(context.Background(), &DevopsAllowlistPutRequest{Content: validAllowlistContent()})
	require.ErrorContains(t, err, "require game and env scope")

	// optimistic lock: after publishing v2, a stale baseline baseVersion=1 → 409
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

	// missing version number / no scope
	_, err := s.DevopsAllowlistRollback(ctx, &DevopsAllowlistRollbackRequest{})
	require.ErrorContains(t, err, "rollback version is required")
	_, err = s.DevopsAllowlistRollback(context.Background(), &DevopsAllowlistRollbackRequest{Version: 1})
	require.ErrorContains(t, err, "require game and env scope")

	// unknown version → 404
	_, err = s.DevopsAllowlistRollback(ctx, &DevopsAllowlistRollbackRequest{Version: 9})
	require.Error(t, err)
	require.Equal(t, 404, httpCode(err))

	// legacy bad row after lint rules evolved: insert via the model to bypass the
	// service-layer lint → rollback is intercepted by the current validation
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

	// unpublished → empty array rather than 404
	items, err := s.DevopsAllowlistVersionsGet(ctx)
	require.NoError(t, err)
	require.Len(t, items, 0)

	// no scope → validation error
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

	// multiple hosts and ports aggregated
	resp, err = s.DevopsAllowlistValidate(&DevopsAllowlistValidateRequest{
		Content: `[{"host":"a","ports":[22,2222],"protocols":["ssh"]},{"host":"b","ports":[80],"protocols":["vnc"]}]`,
	})
	require.NoError(t, err)
	require.Equal(t, 2, resp.Entries)
	require.Equal(t, 3, resp.Ports)

	// nil / empty / bad JSON / missing host
	_, err = s.DevopsAllowlistValidate(nil)
	require.ErrorContains(t, err, "content is required")
	_, err = s.DevopsAllowlistValidate(&DevopsAllowlistValidateRequest{Content: " \t"})
	require.ErrorContains(t, err, "content is required")
	_, err = s.DevopsAllowlistValidate(&DevopsAllowlistValidateRequest{Content: "not-json"})
	require.Error(t, err)
	_, err = s.DevopsAllowlistValidate(&DevopsAllowlistValidateRequest{Content: `[{"host":"","ports":[1]}]`})
	require.ErrorContains(t, err, "host is required")
}

// withScopeRequest attaches a game/env scope to the test request context.
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

	// GET unpublished → empty shape
	c, w := withScopeRequest(t, http.MethodGet, "/ops/devops/allowlist", "")
	h.DevopsAllowlistGet(c)
	require.Equal(t, http.StatusOK, w.Code)
	var got DevopsAllowlistGetResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "devops.allowlist", got.Key)

	// PUT bad JSON → 400
	c, w = withScopeRequest(t, http.MethodPut, "/ops/devops/allowlist", "{bad")
	h.DevopsAllowlistPut(c)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// PUT publish → 200 + version number
	c, w = withScopeRequest(t, http.MethodPut, "/ops/devops/allowlist",
		`{"content":"[{\"host\":\"h1\",\"ports\":[22],\"protocols\":[\"ssh\"]}]"}`)
	h.DevopsAllowlistPut(c)
	require.Equal(t, http.StatusOK, w.Code)
	var put DevopsAllowlistPutResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &put))
	require.Equal(t, 1, put.Version)

	// Validate bad JSON → 400; valid → 200
	c, w = withScopeRequest(t, http.MethodPost, "/ops/devops/allowlist/validate", "{bad")
	h.DevopsAllowlistValidate(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	c, w = withScopeRequest(t, http.MethodPost, "/ops/devops/allowlist/validate",
		`{"content":"[{\"host\":\"h1\",\"ports\":[22],\"protocols\":[\"ssh\"]}]"}`)
	h.DevopsAllowlistValidate(c)
	require.Equal(t, http.StatusOK, w.Code)

	// Rollback bad JSON → 400; valid → 200
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
