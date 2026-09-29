package cicd_test

// CI/CD HTTP handler 层覆盖（OPEN-ISSUES #58 落地时只补了 service 层用例，
// handler.go 整段 0%）：九个端点的成功路径、路径 id 拒绝、绑定失败与
// service 错误透传。
//
// 路由按生产注册序复刻（registerCicdRoutes）——用函数名锚定而非行号，上游
// 增删路由时本文件不会静默漂移。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/api/cicd"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCicdRouter 复刻生产注册序（registerCicdRoutes）：integrations 六端点
// + builds 两端点。
func newCicdRouter(s *cicd.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := cicd.NewHandler(s)
	g := r.Group("/cicd")
	g.GET("/integrations", h.List)
	g.POST("/integrations", h.Create)
	g.PUT("/integrations/:id", h.Update)
	g.DELETE("/integrations/:id", h.Delete)
	g.POST("/integrations/:id/test", h.Test)
	g.POST("/integrations/:id/trigger", h.Trigger)
	g.GET("/builds", h.Builds)
	g.POST("/builds/:id/refresh", h.RefreshBuild)
	return r
}

func doCicd(r http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// createIntegration 走真实端点建一条 jenkins 接入，返回其 id。
func createIntegration(t *testing.T, r http.Handler, endpoint string) uint {
	t.Helper()
	w := doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"gameId":"demo","env":"prod","kind":"jenkins","name":"ci","endpoint":"`+endpoint+`","token":"secret-token-1234"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Integration cicd.Integration `json:"integration"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotZero(t, resp.Integration.ID)
	return resp.Integration.ID
}

// CRUD 主链：建→列→改→测连通→删，凭据掩码不回传明文。
func TestCicdHandler_CRUDRoundTrip(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))

	id := createIntegration(t, r, "http://ci.internal")
	w := doCicd(r, http.MethodGet, "/cicd/integrations?gameId=demo&env=prod", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "****1234", "响应只回掩码，不带明文凭据")
	assert.NotContains(t, w.Body.String(), "secret-token-1234")

	w = doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{"name":"ci-2","enabled":false}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"name":"ci-2"`)
	assert.Contains(t, w.Body.String(), `"enabled":false`)
	// 掩码串回存保护：update 不带 token 时原凭据保留
	assert.Contains(t, w.Body.String(), "****1234")

	// 连通性测试：端点不可达 → ok:false 但 HTTP 200（可达性结论在体里）
	w = doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/test", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"ok":false`)

	w = doCicd(r, http.MethodDelete, "/cicd/integrations/"+itoa(id), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "删除成功")

	// 级联删除后列表为空（items 非 null，供前端表格直接渲染）
	w = doCicd(r, http.MethodGet, "/cicd/integrations", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"items":[]`)
}

// 路径 id 非法：非数字与 0 都必须 400，且不得触达 service（400 而非 404/500）。
func TestCicdHandler_RejectsBadPathID(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))

	cases := []struct{ method, target, body string }{
		{http.MethodPut, "/cicd/integrations/abc", `{"name":"x"}`},
		{http.MethodPut, "/cicd/integrations/0", `{"name":"x"}`},
		{http.MethodDelete, "/cicd/integrations/abc", ""},
		{http.MethodDelete, "/cicd/integrations/0", ""},
		{http.MethodPost, "/cicd/integrations/abc/test", ""},
		{http.MethodPost, "/cicd/integrations/0/test", ""},
		{http.MethodPost, "/cicd/integrations/abc/trigger", `{"pipeline":"p"}`},
		{http.MethodPost, "/cicd/integrations/0/trigger", `{"pipeline":"p"}`},
		{http.MethodPost, "/cicd/builds/abc/refresh", ""},
		{http.MethodPost, "/cicd/builds/0/refresh", ""},
	}
	for _, tc := range cases {
		w := doCicd(r, tc.method, tc.target, tc.body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s %s → %d %s", tc.method, tc.target, w.Code, w.Body.String())
	}
}

// 绑定失败：JSON 体类型不符（bool/map 收字符串/数字）与 query 整型收非数字。
func TestCicdHandler_BindErrors(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := createIntegration(t, r, "http://ci.internal")

	cases := []struct{ method, target, body string }{
		{http.MethodPost, "/cicd/integrations", `{"kind":"jenkins","enabled":"yes-please"}`},
		{http.MethodPost, "/cicd/integrations", `{"kind":"jenkins","extra":123}`},
		{http.MethodPost, "/cicd/integrations", `{"kind":`}, // 截断 JSON
		{http.MethodPut, "/cicd/integrations/" + itoa(id), `{"enabled":"nope"}`},
		{http.MethodPost, "/cicd/integrations/" + itoa(id) + "/trigger", `{"params":"not-a-map"}`},
		{http.MethodGet, "/cicd/builds?limit=abc", ""},
		{http.MethodGet, "/cicd/builds?integrationId=xyz", ""},
	}
	for _, tc := range cases {
		w := doCicd(r, tc.method, tc.target, tc.body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s %s → %d %s", tc.method, tc.target, w.Code, w.Body.String())
	}
}

// service 错误透传：记录不存在 → 404；表缺失（存储故障）→ 500。
func TestCicdHandler_ServiceErrors(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		ctxSvc, _ := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc))
		const missing = "/cicd/integrations/999"
		for _, tc := range []struct{ method, target, body string }{
			{http.MethodPut, missing, `{"name":"x"}`},
			{http.MethodDelete, missing, ""},
			{http.MethodPost, missing + "/test", ""},
			{http.MethodPost, missing + "/trigger", `{"pipeline":"p"}`},
			{http.MethodPost, "/cicd/builds/999/refresh", ""},
		} {
			w := doCicd(r, tc.method, tc.target, tc.body)
			assert.Equal(t, http.StatusNotFound, w.Code, "%s %s → %d %s", tc.method, tc.target, w.Code, w.Body.String())
		}
	})

	t.Run("store failure", func(t *testing.T) {
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc))
		// 接入表缺失：读写两端皆 500（缺表立即报错且无副作用）
		require.NoError(t, db.Migrator().DropTable(&model.CicdIntegration{}))
		for _, tc := range []struct{ method, target, body string }{
			{http.MethodGet, "/cicd/integrations", ""},
			{http.MethodPost, "/cicd/integrations", `{"kind":"jenkins","name":"n","endpoint":"http://ci.internal"}`},
		} {
			w := doCicd(r, tc.method, tc.target, tc.body)
			assert.Equal(t, http.StatusInternalServerError, w.Code, "%s %s → %d %s", tc.method, tc.target, w.Code, w.Body.String())
		}

		// 构建表缺失 → builds 列表 500
		ctxSvc2, db2 := setupCtx(t)
		r2 := newCicdRouter(cicd.NewService(ctxSvc2))
		require.NoError(t, db2.Migrator().DropTable(&model.CicdBuild{}))
		w := doCicd(r2, http.MethodGet, "/cicd/builds", "")
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})
}

// 创建时的业务校验错误由 service 判定，handler 忠实透传 400。
func TestCicdHandler_CreateValidationErrors(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))

	w := doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"teamcity","name":"x","endpoint":"http://ci.internal"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"jenkins","name":"x","endpoint":"ftp://ci.internal"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// provider 必填键缺失 → 构造期 400
	w = doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"gitlab-ci","name":"x","endpoint":"http://ci.internal"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// 触发全链（假 CI 服务器）：落 queued 构建记录；停用接入 → 400。
func TestCicdHandler_TriggerAndRefresh(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/job/pack/build", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/queue/item/7/")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/queue/item/7/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"executable": map[string]any{"number": 7}})
	})
	mux.HandleFunc("/job/pack/7/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"number": 7, "building": true, "url": "http://ci.internal/job/pack/7/",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
	id := createIntegration(t, r, srv.URL)

	w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger",
		`{"pipeline":"pack","version":"3.1.4"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"status":"queued"`)
	assert.Contains(t, w.Body.String(), `"version":"3.1.4"`)

	var tr struct {
		Build cicd.Build `json:"build"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &tr))
	require.NotZero(t, tr.Build.ID)

	// 列表按 version 过滤命中
	w = doCicd(r, http.MethodGet, "/cicd/builds?gameId=demo&env=prod&version=3.1.4", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"total":1`)
	w = doCicd(r, http.MethodGet, "/cicd/builds?version=9.9.9", "")
	assert.Contains(t, w.Body.String(), `"total":0`)

	// 刷新：从 provider 拉回 running
	w = doCicd(r, http.MethodPost, "/cicd/builds/"+itoa(tr.Build.ID)+"/refresh", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"status":"running"`)

	// 停用后拒绝触发
	w = doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{"enabled":false}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger", `{"pipeline":"pack"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "停用")
}

// 连通性测试三翼：任意 HTTP 响应（含 4xx）算可达；传输失败不可达。
func TestCicdHandler_TestConnectionReachability(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
	id := createIntegration(t, r, srv.URL)

	w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/test", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"ok":true`)
	assert.Contains(t, w.Body.String(), "HTTP 404", "4xx 也算可达，凭据有效性不由本探测判定")
}

// DB 关闭态：读端点统一 500，不 panic。
func TestCicdHandler_ClosedDBReturnsError(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	for _, target := range []string{"/cicd/integrations", "/cicd/builds"} {
		w := doCicd(r, http.MethodGet, target, "")
		assert.Equal(t, http.StatusInternalServerError, w.Code, "%s → %d %s", target, w.Code, w.Body.String())
	}
}

// TestCicdHandler_ListBindBranchRegistered 登记 handler.go List 的
// ShouldBindQuery 错误分支（response.Error; return）为防御性不可达：
// IntegrationListRequest 只有 gameId/env 两个 `form:"..."` string 字段，
// gin form 绑定对全字符串字段不存在失败路径——无 required 缺失、无类型转换
// 错误，任何合法 query 都绑定成功。不造假用例、不删防御分支。
//
// 本用例以证明锁定「绑定永不失败」这一前提：若未来该 DTO 引入 required 或
// 强类型（int/bool/int64）字段，该分支变为可达，届时应补真实错误路径用例
// 并删除本证明。
func TestCicdHandler_ListBindBranchRegistered(t *testing.T) {
	for _, q := range []string{
		"",
		"?gameId=",
		"?gameId=demo&env=prod",
		"?unknown=1&gameId=%E6%B8%B8%E6%88%8F",
		"?gameId=demo&gameId=other&env=prod",
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/cicd/integrations"+q, nil)
		var probe cicd.IntegrationListRequest
		require.NoError(t, c.ShouldBindQuery(&probe), "query %q 不应产生绑定错误（分支不可达前提）", q)
	}
}
