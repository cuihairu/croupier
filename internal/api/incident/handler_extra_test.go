package incident

// Handler 端点补测：UpdateCategory / CategoryUsage / GetIncident /
// UpdateIncident 及坏 JSON / 坏 id 参数错误分支，此前无用例触达。
// 复用 setupService，路由注册在本文件内（含 setupRouter 未覆盖的四个端点）。

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupExtraRouter(t *testing.T) (*gin.Engine, *Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := setupService(t)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "tester")
		c.Next()
	})
	h := NewHandler(s)
	g := router.Group("/incident-categories")
	g.GET("", func(c *gin.Context) { h.ListCategories(c) })
	g.POST("", func(c *gin.Context) { h.CreateCategory(c) })
	g.PUT("/:id", func(c *gin.Context) { h.UpdateCategory(c) })
	g.GET("/:id/usage", func(c *gin.Context) { h.CategoryUsage(c) })
	g.DELETE("/:id", func(c *gin.Context) { h.DeleteCategory(c) })
	ig := router.Group("/incidents")
	ig.GET("", func(c *gin.Context) { h.ListIncidents(c) })
	ig.POST("", func(c *gin.Context) { h.CreateIncident(c) })
	ig.GET("/:id", func(c *gin.Context) { h.GetIncident(c) })
	ig.PUT("/:id", func(c *gin.Context) { h.UpdateIncident(c) })
	ig.POST("/:id/status", func(c *gin.Context) { h.TransitionIncident(c) })
	return router, s
}

func createdIncidentID(t *testing.T, body string) uint64 {
	t.Helper()
	var dto struct {
		ID uint64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &dto), "body: %s", body)
	require.NotZero(t, dto.ID)
	return dto.ID
}

func TestHandler_UpdateCategory_RoundTrip(t *testing.T) {
	router, s := setupExtraRouter(t)

	w := doJSON(t, router, http.MethodPost, "/incident-categories", `{"name":"网络","slug":"net"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	qa := mustCategory(t, s, model.IncidentCatQA)
	id := strconv.FormatUint(uint64(qa.ID), 10)
	w = doJSON(t, router, http.MethodPut, "/incident-categories/"+id, `{"name":"问答改","leader":"op"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "问答改")

	w = doJSON(t, router, http.MethodPut, "/incident-categories/"+id, `not-json`)
	require.GreaterOrEqual(t, w.Code, 400)
	w = doJSON(t, router, http.MethodPut, "/incident-categories/abc", `{"name":"x"}`)
	require.GreaterOrEqual(t, w.Code, 400, "non-numeric id must error")
}

func TestHandler_CategoryUsage_CountsIncidentsAndBugs(t *testing.T) {
	router, s := setupExtraRouter(t)
	qa := mustCategory(t, s, model.IncidentCatQA)
	id := strconv.FormatUint(uint64(qa.ID), 10)

	w := doJSON(t, router, http.MethodPost, "/incidents", `{"title":"漏测","categoryId":`+id+`}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = doJSON(t, router, http.MethodGet, "/incident-categories/"+id+"/usage", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"incidents":1`)

	w = doJSON(t, router, http.MethodGet, "/incident-categories/abc/usage", "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestHandler_GetIncident_FoundMissingAndBadID(t *testing.T) {
	router, s := setupExtraRouter(t)
	qa := mustCategory(t, s, model.IncidentCatQA)
	w := doJSON(t, router, http.MethodPost, "/incidents", `{"title":"详情","categoryId":`+strconv.FormatUint(uint64(qa.ID), 10)+`,"incidentKey":"g:1"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	id := strconv.FormatUint(createdIncidentID(t, w.Body.String()), 10)
	w = doJSON(t, router, http.MethodGet, "/incidents/"+id, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "详情")

	w = doJSON(t, router, http.MethodGet, "/incidents/999999", "")
	require.GreaterOrEqual(t, w.Code, 400, "missing incident must error")
	w = doJSON(t, router, http.MethodGet, "/incidents/xyz", "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestHandler_UpdateIncident_PartialFieldsAndErrors(t *testing.T) {
	router, s := setupExtraRouter(t)
	qa := mustCategory(t, s, model.IncidentCatQA)
	w := doJSON(t, router, http.MethodPost, "/incidents", `{"title":"初版","categoryId":`+strconv.FormatUint(uint64(qa.ID), 10)+`}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id := strconv.FormatUint(createdIncidentID(t, w.Body.String()), 10)

	w = doJSON(t, router, http.MethodPut, "/incidents/"+id, `{"title":"改题","severity":"critical"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "改题")

	w = doJSON(t, router, http.MethodPut, "/incidents/"+id, `not-json`)
	require.GreaterOrEqual(t, w.Code, 400)
	w = doJSON(t, router, http.MethodPut, "/incidents/abc", `{"title":"x"}`)
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestHandler_CreateCategory_RejectsMissingName(t *testing.T) {
	router, _ := setupExtraRouter(t)
	w := doJSON(t, router, http.MethodPost, "/incident-categories", `{}`)
	require.GreaterOrEqual(t, w.Code, 400, "empty name must be rejected")
	w = doJSON(t, router, http.MethodPost, "/incident-categories", `not-json`)
	require.GreaterOrEqual(t, w.Code, 400)
}
