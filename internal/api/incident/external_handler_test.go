package incident

// external_handler.go HTTP 端点补测：Bearer 鉴权中止（缺失/未知/禁用）、
// 限流 429、事故登记 created/幂等重放/坏请求体、查询参数解析、bug 登记、
// 令牌管理 CRUD（明文仅创建时返回一次、空更新 400）与 RequireAdminRole 守卫。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newExtTestRouter(svc *ExternalService, execLog *executionlog.Writer, tokenRoles []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewExternalHandler(svc, execLog)
	router := gin.New()
	router.POST("/ext/incidents", h.RegisterIncident)
	router.GET("/ext/incidents", h.ListIncidents)
	router.POST("/ext/bugs", h.CreateBug)
	tokens := router.Group("/ext/tokens", func(c *gin.Context) {
		c.Set("username", "tester")
		c.Set("roles", tokenRoles)
		c.Next()
	}, RequireAdminRole())
	tokens.POST("", func(c *gin.Context) { h.CreateToken(c) })
	tokens.GET("", func(c *gin.Context) { h.ListTokens(c) })
	tokens.PUT("/:id", func(c *gin.Context) { h.UpdateToken(c) })
	tokens.DELETE("/:id", func(c *gin.Context) { h.DeleteToken(c) })
	return router
}

func setupExternalRouter(t *testing.T, tokenRoles []string) (*gin.Engine, *ExternalService, *gorm.DB, string) {
	return setupExternalRouterWithLimit(t, 2, tokenRoles)
}

func setupExternalRouterWithLimit(t *testing.T, rpm int, tokenRoles []string) (*gin.Engine, *ExternalService, *gorm.DB, string) {
	t.Helper()
	svc, db := setupExternalWithLimit(t, rpm)
	// 接真实留痕 writer 覆盖 audit 分支；未启动 Run，队列满静默丢弃，Stop 安全
	w := executionlog.NewWriter(db, executionlog.Config{})
	t.Cleanup(w.Stop)
	plain := "ext-plain-token-1"
	createTokenRow(t, db, "probe", plain, true, model.JSON("null"))
	return newExtTestRouter(svc, w, tokenRoles), svc, db, plain
}

func extReq(t *testing.T, router *gin.Engine, method, path, body, bearer, idemKey string) *httptest.ResponseRecorder {
	t.Helper()
	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, payload)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestBearerToken_Parse(t *testing.T) {
	require.Empty(t, bearerToken(""))
	require.Equal(t, "raw-key", bearerToken("raw-key"))
	require.Equal(t, "abc", bearerToken("Bearer abc"))
	// 非 Bearer scheme 只取首段
	require.Equal(t, "basic", bearerToken("basic xyz"))
}

func TestExternalRoutes_AuthFailures(t *testing.T) {
	router, _, db, _ := setupExternalRouter(t, []string{"admin"})
	createTokenRow(t, db, "off", "ext-plain-token-2", false, model.JSON("null"))

	// 缺失 Authorization
	w := extReq(t, router, http.MethodPost, "/ext/incidents", `{"title":"x"}`, "", "")
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	// 未知 token
	w = extReq(t, router, http.MethodGet, "/ext/incidents", "", "no-such-token", "")
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	// 已禁用 token
	w = extReq(t, router, http.MethodGet, "/ext/incidents", "", "ext-plain-token-2", "")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestExternalRoutes_RateLimited(t *testing.T) {
	svc, db := setupExternalWithLimit(t, 1)
	plain := "ext-plain-token-3"
	createTokenRow(t, db, "solo", plain, true, model.JSON("null"))
	router := newExtTestRouter(svc, nil, []string{"admin"})

	w := extReq(t, router, http.MethodGet, "/ext/incidents", "", plain, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = extReq(t, router, http.MethodGet, "/ext/incidents", "", plain, "")
	require.Equal(t, http.StatusTooManyRequests, w.Code, w.Body.String())
}

func TestExternalRoutes_RegisterIncident_Flow(t *testing.T) {
	// rpm=10：本用例连发 4 个请求，默认桶容量 2 会被限流
	router, svc, _, plain := setupExternalRouterWithLimit(t, 10, []string{"admin"})
	qa := mustCategory(t, svc.incSvc, model.IncidentCatQA)
	catID := strconv.FormatUint(uint64(qa.ID), 10)

	// 坏请求体
	w := extReq(t, router, http.MethodPost, "/ext/incidents", "not-json", plain, "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "请求体格式错误")

	// 正常创建（带幂等键）→ 201
	body := `{"title":"登录崩溃","categoryId":` + catID + `,"severity":"critical"}`
	w = extReq(t, router, http.MethodPost, "/ext/incidents", body, plain, "evt-9001")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "登录崩溃")
	id := createdIncidentID(t, w.Body.String())

	// 同幂等键重放 → 200 且同一事故
	w = extReq(t, router, http.MethodPost, "/ext/incidents", body, plain, "evt-9001")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, id, createdIncidentID(t, w.Body.String()))

	// 不存在的分类 → 业务错误
	w = extReq(t, router, http.MethodPost, "/ext/incidents", `{"title":"x","categoryId":999999}`, plain, "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestExternalRoutes_ListIncidents_QueryParams(t *testing.T) {
	router, svc, _, plain := setupExternalRouterWithLimit(t, 10, []string{"admin"})
	qa := mustCategory(t, svc.incSvc, model.IncidentCatQA)
	w := extReq(t, router, http.MethodPost, "/ext/incidents",
		`{"title":"查询样本","categoryId":`+strconv.FormatUint(uint64(qa.ID), 10)+`,"severity":"critical"}`, plain, "evt-9002")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = extReq(t, router, http.MethodGet, "/ext/incidents?page=1&pageSize=5&severity=critical", "", plain, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"total":1`)

	w = extReq(t, router, http.MethodGet, "/ext/incidents?status=resolved", "", plain, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"total":0`)

	// 非法 from/to 与非正分页被忽略，不报错
	w = extReq(t, router, http.MethodGet, "/ext/incidents?from=bad&to=bad&page=0&pageSize=-3", "", plain, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"total":1`)
}

func TestExternalRoutes_CreateBug(t *testing.T) {
	router, _, _, plain := setupExternalRouter(t, []string{"admin"})

	w := extReq(t, router, http.MethodPost, "/ext/bugs", "not-json", plain, "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "请求体格式错误")

	w = extReq(t, router, http.MethodPost, "/ext/bugs", `{"title":"外部缺陷"}`, plain, "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "外部缺陷")
}

func TestExternalTokenAdmin_Lifecycle(t *testing.T) {
	router, svc, _, _ := setupExternalRouter(t, []string{"admin"})

	// 空名 / 坏请求体
	w := extReq(t, router, http.MethodPost, "/ext/tokens", `{}`, "", "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "name 不能为空")
	w = extReq(t, router, http.MethodPost, "/ext/tokens", "not-json", "", "")
	require.GreaterOrEqual(t, w.Code, 400)

	// 创建成功：明文仅此一次返回
	w = extReq(t, router, http.MethodPost, "/ext/tokens", `{"name":"ci-bot","scope":{"categories":["qa"]}}`, "", "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct {
		ID        uint   `json:"id"`
		PlainText string `json:"plainText"`
		CreatedBy string `json:"createdBy"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created), w.Body.String())
	require.NotZero(t, created.ID)
	require.NotEmpty(t, created.PlainText)
	require.Equal(t, "tester", created.CreatedBy)
	tok, err := svc.ResolveToken(context.Background(), created.PlainText)
	require.NoError(t, err)
	require.Equal(t, "ci-bot", tok.Name)

	// 显式 enabled:true 分支
	w = extReq(t, router, http.MethodPost, "/ext/tokens", `{"name":"on-bot","enabled":true}`, "", "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"enabled":true`)

	// 列表不泄露明文
	w = extReq(t, router, http.MethodGet, "/ext/tokens", "", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), `"plainText"`)

	id := strconv.FormatUint(uint64(created.ID), 10)

	// 坏 id / 坏请求体 / 空更新
	w = extReq(t, router, http.MethodPut, "/ext/tokens/abc", `{"name":"x"}`, "", "")
	require.GreaterOrEqual(t, w.Code, 400)
	w = extReq(t, router, http.MethodPut, "/ext/tokens/"+id, "not-json", "", "")
	require.GreaterOrEqual(t, w.Code, 400)
	w = extReq(t, router, http.MethodPut, "/ext/tokens/"+id, `{}`, "", "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "无可更新字段")

	// 停用 → 204 → 明文失效
	w = extReq(t, router, http.MethodPut, "/ext/tokens/"+id, `{"enabled":false}`, "", "")
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	_, err = svc.ResolveToken(context.Background(), created.PlainText)
	require.Error(t, err, "disabled token must stop resolving")

	// 吊销 → 204 → 明文无效
	w = extReq(t, router, http.MethodDelete, "/ext/tokens/"+id, "", "", "")
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	_, err = svc.ResolveToken(context.Background(), created.PlainText)
	require.Error(t, err, "revoked token must stop resolving")
}

func TestExternalTokenAdmin_RequiresAdminRole(t *testing.T) {
	router, _, _, _ := setupExternalRouter(t, []string{"ops"})
	w := extReq(t, router, http.MethodGet, "/ext/tokens", "", "", "")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "需要 admin 角色")

	// 角色匹配忽略大小写与空白
	router, _, _, _ = setupExternalRouter(t, []string{"ops", " ADMIN "})
	w = extReq(t, router, http.MethodGet, "/ext/tokens", "", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
