package incident

// Additional HTTP endpoint tests for external_handler.go: Bearer auth abort
// (missing/unknown/disabled), rate-limit 429, incident registration
// created/idempotent-replay/bad request body, query parameter parsing, bug
// registration, token management CRUD (plaintext returned only once at creation,
// empty update 400) and the RequireAdminRole guard.

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
	// Wire up the real audit-trail writer to cover the audit branch; Run is not
	// started, a full queue silently drops entries, and Stop is safe
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
	// Non-Bearer scheme: take only the first segment
	require.Equal(t, "basic", bearerToken("basic xyz"))
}

func TestExternalRoutes_AuthFailures(t *testing.T) {
	router, _, db, _ := setupExternalRouter(t, []string{"admin"})
	createTokenRow(t, db, "off", "ext-plain-token-2", false, model.JSON("null"))

	// Missing Authorization
	w := extReq(t, router, http.MethodPost, "/ext/incidents", `{"title":"x"}`, "", "")
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	// Unknown token
	w = extReq(t, router, http.MethodGet, "/ext/incidents", "", "no-such-token", "")
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	// Disabled token
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
	// rpm=10: this test fires 4 requests back-to-back; with the default bucket
	// capacity 2 they would be rate limited
	router, svc, _, plain := setupExternalRouterWithLimit(t, 10, []string{"admin"})
	qa := mustCategory(t, svc.incSvc, model.IncidentCatQA)
	catID := strconv.FormatUint(uint64(qa.ID), 10)

	// Bad request body
	w := extReq(t, router, http.MethodPost, "/ext/incidents", "not-json", plain, "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "请求体格式错误")

	// Normal creation (with idempotency key) -> 201
	body := `{"title":"登录崩溃","categoryId":` + catID + `,"severity":"critical"}`
	w = extReq(t, router, http.MethodPost, "/ext/incidents", body, plain, "evt-9001")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "登录崩溃")
	id := createdIncidentID(t, w.Body.String())

	// Replay with the same idempotency key -> 200 and the same incident
	w = extReq(t, router, http.MethodPost, "/ext/incidents", body, plain, "evt-9001")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, id, createdIncidentID(t, w.Body.String()))

	// Non-existent category -> business error
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

	// Invalid from/to and non-positive pagination are ignored, without error
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

	// Empty name / bad request body
	w := extReq(t, router, http.MethodPost, "/ext/tokens", `{}`, "", "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "name 不能为空")
	w = extReq(t, router, http.MethodPost, "/ext/tokens", "not-json", "", "")
	require.GreaterOrEqual(t, w.Code, 400)

	// Created successfully: the plaintext is returned only this once
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

	// Explicit enabled:true branch
	w = extReq(t, router, http.MethodPost, "/ext/tokens", `{"name":"on-bot","enabled":true}`, "", "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"enabled":true`)

	// The list does not leak the plaintext
	w = extReq(t, router, http.MethodGet, "/ext/tokens", "", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), `"plainText"`)

	id := strconv.FormatUint(uint64(created.ID), 10)

	// Bad id / bad request body / empty update
	w = extReq(t, router, http.MethodPut, "/ext/tokens/abc", `{"name":"x"}`, "", "")
	require.GreaterOrEqual(t, w.Code, 400)
	w = extReq(t, router, http.MethodPut, "/ext/tokens/"+id, "not-json", "", "")
	require.GreaterOrEqual(t, w.Code, 400)
	w = extReq(t, router, http.MethodPut, "/ext/tokens/"+id, `{}`, "", "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "无可更新字段")

	// Disable -> 204 -> plaintext no longer resolves
	w = extReq(t, router, http.MethodPut, "/ext/tokens/"+id, `{"enabled":false}`, "", "")
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	_, err = svc.ResolveToken(context.Background(), created.PlainText)
	require.Error(t, err, "disabled token must stop resolving")

	// Revoke -> 204 -> plaintext invalid
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

	// Role matching ignores case and whitespace
	router, _, _, _ = setupExternalRouter(t, []string{"ops", " ADMIN "})
	w = extReq(t, router, http.MethodGet, "/ext/tokens", "", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
