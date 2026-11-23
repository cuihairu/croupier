package cicd_test

// AUD-043 回归：CICD webhook 回写 payload 的 game/env 绑定 integration 配置。
//
// 根因：IngestWebhook 此前以 firstNonEmpty(row.GameID, p.GameID) 落库——
// integration 已配作用域时 payload 被忽略，但 integration 未配作用域时
// payload 可注入任意 scope（跨域写入构建记录）。修复为 fail-closed 绑定：
// payload 提供作用域时必须与 integration 配置精确一致，否则 400
// scope_mismatch 且不落库；未提供时作用域仅由 integration 派生，payload 值
// 永不被采信写入。

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

// aud043PostWebhook 以指定 token 投递一次 webhook，返回响应记录器。
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

// aud043BuildCount 返回指定 integration 的落库构建数（拒绝的投递不得留痕）。
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

	// ① payload 不带作用域 → 放行，作用域由 integration 派生。
	w := aud043PostWebhook(t, svcCicd, id, base(nil, nil), "hook-secret")
	require.Equal(t, http.StatusOK, w.Code, "payload 省略作用域必须放行: %s", w.Body.String())
	var out struct {
		Build cicd.Build `json:"build"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "demo", out.Build.GameID, "落库 game_id 必须取 integration 配置")
	assert.Equal(t, "prod", out.Build.Env, "落库 env 必须取 integration 配置")

	// ② payload 与 integration 配置精确一致 → 放行（同 externalId 幂等更新）。
	w = aud043PostWebhook(t, svcCicd, id, base("demo", "prod"), "hook-secret")
	require.Equal(t, http.StatusOK, w.Code, "payload 与配置一致必须放行: %s", w.Body.String())

	// ③ 跨 scope 回写（game_id 冲突）→ 400 scope_mismatch，拒绝且不留痕。
	before := aud043BuildCount(t, svcCicd, id)
	w = aud043PostWebhook(t, svcCicd, id, base("other-game", nil), "hook-secret")
	require.Equal(t, http.StatusBadRequest, w.Code, "跨 scope 回写必须 4xx 拒绝: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "scope_mismatch", "稳定错误码 scope_mismatch")
	assert.Equal(t, before, aud043BuildCount(t, svcCicd, id), "被拒投递不得落库或改写既有构建")

	// ④ 跨 scope 回写（env 冲突）→ 同上拒绝。
	w = aud043PostWebhook(t, svcCicd, id, base("demo", "dev"), "hook-secret")
	require.Equal(t, http.StatusBadRequest, w.Code, "env 冲突必须拒绝: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "scope_mismatch")
}

func TestCicdWebhook_Aud043_NoScopeInjectionWhenIntegrationUnconfigured(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	svcCicd := cicd.NewService(ctxSvc)
	ctx := context.Background()

	// 未声明作用域的 integration：payload 不得替它注入 scope。
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

	// 省略作用域的投递仍放行（存量行为兼容：构建记录作用域为空）。
	delete(payload, "gameId")
	delete(payload, "env")
	w = aud043PostWebhook(t, svcCicd, id, payload, "hook-secret")
	require.Equal(t, http.StatusOK, w.Code, "payload 省略作用域必须放行: %s", w.Body.String())
}
