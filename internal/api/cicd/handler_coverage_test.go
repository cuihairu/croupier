package cicd_test

// CI/CD HTTP handler-layer coverage (the OPEN-ISSUES #58 landing only added
// service-layer cases and handler.go was at 0% throughout): the success paths of
// the nine endpoints, path-id rejection, binding failures, and service error
// pass-through.
//
// The routes are replicated in the production registration order
// (registerCicdRoutes) - anchored by function name rather than line number, so this
// file does not drift silently when routes are added or removed upstream.

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

// newCicdRouter replicates the production registration order (registerCicdRoutes): six
// integrations endpoints + two builds endpoints.
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

// createIntegration creates a jenkins integration through the real endpoint and
// returns its id.
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

// CRUD main chain: create -> list -> update -> test connectivity -> delete, with the
// credential mask never echoing the plaintext.
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
	// Masked-string round-trip protection: an update without a token keeps the original
	// credential
	assert.Contains(t, w.Body.String(), "****1234")

	// Connectivity test: the endpoint is unreachable -> ok:false but HTTP 200 (the
	// reachability conclusion is in the body)
	w = doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/test", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"ok":false`)

	w = doCicd(r, http.MethodDelete, "/cicd/integrations/"+itoa(id), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "删除成功")

	// After the cascading delete the list is empty (items is non-null, so the frontend
	// table can render directly)
	w = doCicd(r, http.MethodGet, "/cicd/integrations", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"items":[]`)
}

// Invalid path ids: non-numeric and 0 must both yield 400 and must not reach the
// service (400 rather than 404/500).
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
		// Overflow: a path id above uint32 must be rejected at pathID (bitSize 32) rather
		// than silently truncated by uint() after 64-bit parsing (CodeQL
		// go/incorrect-integer-conversion; 2^32 -> 0 on 32-bit platforms).
		{http.MethodPut, "/cicd/integrations/4294967296", `{"name":"x"}`},
		{http.MethodDelete, "/cicd/integrations/4294967296", ""},
		{http.MethodPost, "/cicd/builds/4294967296/refresh", ""},
	}
	// Boundary acceptance: the uint32 upper edge (4294967295) parses within bitSize 32
	// and is not rejected by pathID (a non-existent record is handled by the service
	// layer as 404).
	w := doCicd(r, http.MethodDelete, "/cicd/integrations/4294967295", "")
	assert.NotEqual(t, http.StatusBadRequest, w.Code, "uint32 上边界不应被 pathID 拒绝：%s", w.Body.String())
	for _, tc := range cases {
		w := doCicd(r, tc.method, tc.target, tc.body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s %s → %d %s", tc.method, tc.target, w.Code, w.Body.String())
	}
}

// Binding failures: JSON body type mismatches (bool/map receiving a string/number) and
// query integer fields receiving non-numbers.
func TestCicdHandler_BindErrors(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := createIntegration(t, r, "http://ci.internal")

	cases := []struct{ method, target, body string }{
		{http.MethodPost, "/cicd/integrations", `{"kind":"jenkins","enabled":"yes-please"}`},
		{http.MethodPost, "/cicd/integrations", `{"kind":"jenkins","extra":123}`},
		{http.MethodPost, "/cicd/integrations", `{"kind":`}, // truncated JSON
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

// Service error pass-through: a missing record -> 404; a missing table (storage
// failure) -> 500.
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
		// Missing integrations table: both the read and write sides get 500 (a missing
		// table errors immediately and has no side effects)
		require.NoError(t, db.Migrator().DropTable(&model.CicdIntegration{}))
		for _, tc := range []struct{ method, target, body string }{
			{http.MethodGet, "/cicd/integrations", ""},
			{http.MethodPost, "/cicd/integrations", `{"kind":"jenkins","name":"n","endpoint":"http://ci.internal"}`},
		} {
			w := doCicd(r, tc.method, tc.target, tc.body)
			assert.Equal(t, http.StatusInternalServerError, w.Code, "%s %s → %d %s", tc.method, tc.target, w.Code, w.Body.String())
		}

		// Missing builds table -> the builds list gets 500
		ctxSvc2, db2 := setupCtx(t)
		r2 := newCicdRouter(cicd.NewService(ctxSvc2))
		require.NoError(t, db2.Migrator().DropTable(&model.CicdBuild{}))
		w := doCicd(r2, http.MethodGet, "/cicd/builds", "")
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})
}

// Business validation errors at creation are decided by the service; the handler
// faithfully passes the 400 through.
func TestCicdHandler_CreateValidationErrors(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))

	w := doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"teamcity","name":"x","endpoint":"http://ci.internal"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"jenkins","name":"x","endpoint":"ftp://ci.internal"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// Missing required provider key -> 400 at construction
	w = doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"gitlab-ci","name":"x","endpoint":"http://ci.internal"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// Trigger full chain (fake CI server): a queued build record lands; a disabled
// integration -> 400.
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

	// List filter by version hits
	w = doCicd(r, http.MethodGet, "/cicd/builds?gameId=demo&env=prod&version=3.1.4", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"total":1`)
	w = doCicd(r, http.MethodGet, "/cicd/builds?version=9.9.9", "")
	assert.Contains(t, w.Body.String(), `"total":0`)

	// Refresh: pulled back as running from the provider
	w = doCicd(r, http.MethodPost, "/cicd/builds/"+itoa(tr.Build.ID)+"/refresh", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"status":"running"`)

	// Trigger rejected after the integration is disabled
	w = doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{"enabled":false}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger", `{"pipeline":"pack"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "停用")
}

// Three wings of the connectivity test: any HTTP response (including 4xx) counts as
// reachable; a transport failure is unreachable.
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

// Closed DB state: read endpoints uniformly return 500 and do not panic.
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

// TestCicdHandler_ListBindBranchRegistered registers the ShouldBindQuery error branch
// (response.Error; return) of handler.go List as defensively unreachable:
// IntegrationListRequest has only the two `form:"..."` string fields gameId/env, and
// gin form binding has no failure path for all-string fields - no required-missing
// and no type-conversion errors exist, so any legal query binds successfully. No
// synthetic cases are built and no defense branch is deleted.
//
// This case proves the locked premise that "binding never fails": if that DTO later
// gains required or strongly-typed (int/bool/int64) fields, the branch becomes
// reachable and a real error-path case should then be added and this proof deleted.
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
