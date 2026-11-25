package cicd_test

// AUD-043 regression: the CICD webhook write-back payload's game/env binding is bound
// to the integration configuration.
//
// Root cause: IngestWebhook previously landed firstNonEmpty(row.GameID, p.GameID) -
// when the integration had a scope configured the payload was ignored, but when it had
// none the payload could inject an arbitrary scope (cross-domain writes into build
// records). The fix is fail-closed binding: when the payload supplies a scope it must
// match the integration configuration exactly, otherwise 400 scope_mismatch and
// nothing lands; when no scope is supplied it derives only from the integration and a
// payload value is never trusted for a write.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/api/cicd"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aud043PostWebhook delivers one webhook with the given token and returns the response
// recorder.
func aud043PostWebhook(t *testing.T, svcCicd *cicd.Service, integrationID uint, payload map[string]any, token string) *httptest.ResponseRecorder {
	t.Helper()
	h := cicd.NewHandler(svcCicd)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/cicd/webhooks/:id", h.Webhook)
	w := httptest.NewRecorder()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/"+strconv.FormatUint(uint64(integrationID), 10), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CICD-Token", token)
	r.ServeHTTP(w, req)
	return w
}

// aud043BuildCount returns the number of landed builds for the given integration
// (a rejected delivery must leave no trace).
func aud043BuildCount(t *testing.T, svcCicd *cicd.Service, integrationID uint) int64 {
	t.Helper()
	resp, err := svcCicd.ListBuilds(context.Background(), &cicd.BuildListRequest{IntegrationID: integrationID})
	require.NoError(t, err)
	return resp.Total
}

func TestCicdWebhook_Aud043_ScopeBoundToIntegration(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	svcCicd := cicd.NewService(ctxSvc)
	ctx := context.Background()

	created, err := svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		GameID: "demo", Env: "prod", Kind: "generic", Name: "bound-ci",
		Endpoint: "http://ci.internal",
		Token:    "hook-secret",
		Extra:    map[string]string{"statusUrl": "http://ci.internal/s/{id}"},
	})
	require.NoError(t, err)
	id := created.Integration.ID

	base := func(gameID, env any) map[string]any {
		payload := map[string]any{
			"externalId": "aud043-b1", "status": "passed",
			"version": "2.0.0", "pipeline": "release",
		}
		if gameID != nil {
			payload["gameId"] = gameID
		}
		if env != nil {
			payload["env"] = env
		}
		return payload
	}

	// (1) payload without a scope -> allowed through; the scope derives from the
	// integration.
	w := aud043PostWebhook(t, svcCicd, id, base(nil, nil), "hook-secret")
	require.Equal(t, http.StatusOK, w.Code, "payload 省略作用域必须放行: %s", w.Body.String())
	var out struct {
		Build cicd.Build `json:"build"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "demo", out.Build.GameID, "落库 game_id 必须取 integration 配置")
	assert.Equal(t, "prod", out.Build.Env, "落库 env 必须取 integration 配置")

	// (2) payload exactly matching the integration config -> allowed (same externalId,
	// idempotent update).
	w = aud043PostWebhook(t, svcCicd, id, base("demo", "prod"), "hook-secret")
	require.Equal(t, http.StatusOK, w.Code, "payload 与配置一致必须放行: %s", w.Body.String())

	// (3) Cross-scope write-back (game_id conflict) -> 400 scope_mismatch, rejected and
	// leaving no trace.
	before := aud043BuildCount(t, svcCicd, id)
	w = aud043PostWebhook(t, svcCicd, id, base("other-game", nil), "hook-secret")
	require.Equal(t, http.StatusBadRequest, w.Code, "跨 scope 回写必须 4xx 拒绝: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "scope_mismatch", "稳定错误码 scope_mismatch")
	assert.Equal(t, before, aud043BuildCount(t, svcCicd, id), "被拒投递不得落库或改写既有构建")

	// (4) Cross-scope write-back (env conflict) -> rejected the same way.
	w = aud043PostWebhook(t, svcCicd, id, base("demo", "dev"), "hook-secret")
	require.Equal(t, http.StatusBadRequest, w.Code, "env 冲突必须拒绝: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "scope_mismatch")
}

func TestCicdWebhook_Aud043_NoScopeInjectionWhenIntegrationUnconfigured(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	svcCicd := cicd.NewService(ctxSvc)
	ctx := context.Background()

	// An integration that declares no scope: the payload must not inject a scope on its
	// behalf.
	created, err := svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		Kind: "generic", Name: "no-scope-ci",
		Endpoint: "http://ci.internal",
		Token:    "hook-secret",
		Extra:    map[string]string{"statusUrl": "http://ci.internal/s/{id}"},
	})
	require.NoError(t, err)
	id := created.Integration.ID

	payload := map[string]any{
		"externalId": "aud043-b2", "status": "passed",
		"gameId": "demo", "env": "prod",
	}
	w := aud043PostWebhook(t, svcCicd, id, payload, "hook-secret")
	require.Equal(t, http.StatusBadRequest, w.Code, "未配置作用域的 integration 不得由 payload 注入 scope: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "scope_mismatch")
	assert.Equal(t, int64(0), aud043BuildCount(t, svcCicd, id), "被拒投递不得落库")

	// A delivery that omits the scope is still allowed through (legacy behavior kept:
	// the build record's scope stays empty).
	delete(payload, "gameId")
	delete(payload, "env")
	w = aud043PostWebhook(t, svcCicd, id, payload, "hook-secret")
	require.Equal(t, http.StatusOK, w.Code, "payload 省略作用域必须放行: %s", w.Body.String())
}
