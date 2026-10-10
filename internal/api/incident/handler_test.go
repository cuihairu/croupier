package incident

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
)

func setupRouter(t *testing.T) (*gin.Engine, *Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := setupService(t)
	router := gin.New()
	// 注入 JWT username 的最小中间件（CreateIncident 取 c.GetString("username")）
	router.Use(func(c *gin.Context) {
		c.Set("username", "tester")
		c.Next()
	})
	g := router.Group("/incident-categories")
	g.GET("", func(c *gin.Context) { NewHandler(s).ListCategories(c) })
	g.POST("", func(c *gin.Context) { NewHandler(s).CreateCategory(c) })
	g.DELETE("/:id", func(c *gin.Context) { NewHandler(s).DeleteCategory(c) })
	ig := router.Group("/incidents")
	ig.GET("", func(c *gin.Context) { NewHandler(s).ListIncidents(c) })
	ig.POST("", func(c *gin.Context) { NewHandler(s).CreateIncident(c) })
	ig.POST("/:id/status", func(c *gin.Context) { NewHandler(s).TransitionIncident(c) })
	return router, s
}

func doJSON(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestHandler_CreateIncident_CreatedVsReplay(t *testing.T) {
	router, s := setupRouter(t)
	qa := mustCategory(t, s, model.IncidentCatQA)
	body := `{"title":"漏测","categoryId":` + strconv.FormatUint(uint64(qa.ID), 10) + `,"incidentKey":"qa:1"}`
	w := doJSON(t, router, http.MethodPost, "/incidents", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create = %d, want 201: %s", w.Code, w.Body.String())
	}
	w = doJSON(t, router, http.MethodPost, "/incidents", body)
	if w.Code != http.StatusOK {
		t.Fatalf("replay = %d, want 200: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListCategories(t *testing.T) {
	router, _ := setupRouter(t)
	w := doJSON(t, router, http.MethodGet, "/incident-categories", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !strings.Contains(got, `"slug":"uncategorized"`) {
		t.Fatalf("missing uncategorized seed in %s", got)
	}
}

func TestHandler_BadRequestPaths(t *testing.T) {
	router, _ := setupRouter(t)
	// 非数字 id
	if w := doJSON(t, router, http.MethodDelete, "/incident-categories/abc", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id = %d, want 400", w.Code)
	}
	// 非法 JSON body
	if w := doJSON(t, router, http.MethodPost, "/incidents", "{not json"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad body = %d, want 400", w.Code)
	}
	// 非法状态
	if w := doJSON(t, router, http.MethodPost, "/incidents/1/status", `{"status":"bogus"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad status = %d, want 400: %s", w.Code, w.Body.String())
	}
}

// TestHandler_ListIncidents_UnixSecondsFrom：query from 兼容 unix 秒形态
// （RangePicker 走 RFC3339，脚本调用常给秒）。
func TestHandler_ListIncidents_UnixSecondsFrom(t *testing.T) {
	router, _ := setupRouter(t)
	w := doJSON(t, router, http.MethodGet, "/incidents?from=1700000000", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", w.Code, w.Body.String())
	}
	// 不可解析的时间被忽略（不报错），列表照常返回
	w = doJSON(t, router, http.MethodGet, "/incidents?from=not-a-time", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list with bad from = %d: %s", w.Code, w.Body.String())
	}
}
