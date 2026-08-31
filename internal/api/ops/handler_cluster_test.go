package ops

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// HTTP handler 层测试：ClusterInfo + LBStatsQuery 的参数绑定/错误/成功分支。

func TestHandler_ClusterInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{}))
	r := gin.New()
	r.GET("/cluster", h.ClusterInfo)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/cluster", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "standalone")
}

func TestHandler_LBStatsQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{}))
	r := gin.New()
	r.POST("/lb-stats", h.LBStatsQuery)

	// nil body → ShouldBindJSON EOF → 400
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lb-stats", nil)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 正常 body 但未配置 → 500
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/lb-stats", strings.NewReader(`{"query":"haproxy_up"}`))
	req3.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusInternalServerError, w3.Code)
}
