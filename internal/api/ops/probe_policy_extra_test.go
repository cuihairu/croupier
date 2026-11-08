package ops

// probe_policy.go handler 面补测：五个 gin handler 的往返（GET 空形态、PUT
// 发布递增、dry-run 预检、回滚、版本历史）与坏载荷 400 面。service 语义
// 分支由 probe_policy_test.go 覆盖。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProbePolicyHandlersRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s, _ := newProbePolicyService(t)
	h := NewHandler(s)

	mkCtx := func(t *testing.T, method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
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

	// GET 未发布 → 空形态
	c, w := mkCtx(t, http.MethodGet, "/ops/probe/policy", "")
	h.ProbePolicyGet(c)
	require.Equal(t, http.StatusOK, w.Code)
	var got ProbePolicyGetResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "probe.policy", got.Key)
	require.Equal(t, 0, got.Version)

	// PUT 坏 JSON → 400
	c, w = mkCtx(t, http.MethodPut, "/ops/probe/policy", "{bad")
	h.ProbePolicyPut(c)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// PUT 发布 → 200 + 版本号
	c, w = mkCtx(t, http.MethodPut, "/ops/probe/policy",
		`{"content":"{\"interval\":\"30s\"}","message":"first"}`)
	h.ProbePolicyPut(c)
	require.Equal(t, http.StatusOK, w.Code)
	var put ProbePolicyPutResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &put))
	require.Equal(t, 1, put.Version)

	// Validate 坏 JSON → 400；合法 → 200
	c, w = mkCtx(t, http.MethodPost, "/ops/probe/policy/validate", "{bad")
	h.ProbePolicyValidate(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	c, w = mkCtx(t, http.MethodPost, "/ops/probe/policy/validate",
		`{"content":"{\"interval\":\"30s\",\"targets\":[{\"target\":\"db-primary\",\"interval\":\"10s\"}],\"windows\":[{\"start\":\"19:00\",\"end\":\"23:00\",\"interval\":\"8s\"}]}"}`)
	h.ProbePolicyValidate(c)
	require.Equal(t, http.StatusOK, w.Code)
	var val ProbePolicyValidateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &val))
	require.True(t, val.Valid)
	require.Equal(t, 1, val.Targets)

	// Rollback 坏 JSON → 400；合法（回滚到 v1）→ 200 新版本
	c, w = mkCtx(t, http.MethodPost, "/ops/probe/policy/rollback", "{bad")
	h.ProbePolicyRollback(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	c, w = mkCtx(t, http.MethodPost, "/ops/probe/policy/rollback", `{"version":1}`)
	h.ProbePolicyRollback(c)
	require.Equal(t, http.StatusOK, w.Code)

	// Versions → 200 两条历史
	c, w = mkCtx(t, http.MethodGet, "/ops/probe/policy/versions", "")
	h.ProbePolicyVersionsGet(c)
	require.Equal(t, http.StatusOK, w.Code)
	var versions []ProbePolicyVersionItem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &versions))
	require.Len(t, versions, 2)
}
