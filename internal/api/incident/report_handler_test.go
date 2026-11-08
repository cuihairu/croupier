package incident

// report_handler.go HTTP 端点补测：summary/trend/leaderboard/responsibility
// 的默认档位与坏参数校验、repush 坏 id/缺失报表、stored 空列表。

import (
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupReportRouter(t *testing.T) (*gin.Engine, *Handler) {
	t.Helper()
	s := setupService(t)
	require.NoError(t, s.svcCtx.DB.AutoMigrate(&model.IncidentReport{}))
	gin.SetMode(gin.TestMode)
	h := NewHandler(s)
	router := gin.New()
	rp := router.Group("/incident-reports")
	rp.GET("/summary", h.GetSummary)
	rp.GET("/trend", h.GetTrend)
	rp.GET("/leaderboard", h.GetLeaderboard)
	rp.GET("/responsibility", h.GetResponsibility)
	rp.POST("/:id/repush", h.RepushReport)
	rp.GET("/stored", h.ListStoredReports)
	return router, h
}

func TestReportHandler_SummaryAndTrend(t *testing.T) {
	router, _ := setupReportRouter(t)

	// 默认/显式周档
	w := doJSON(t, router, http.MethodGet, "/incident-reports/summary", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doJSON(t, router, http.MethodGet, "/incident-reports/summary?period=week", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	// 坏档位
	w = doJSON(t, router, http.MethodGet, "/incident-reports/summary?period=day", "")
	require.GreaterOrEqual(t, w.Code, 400)

	// trend 显式窗口（RFC3339）+ 非法 from 被忽略
	w = doJSON(t, router, http.MethodGet,
		"/incident-reports/trend?bucket=day&from=2026-11-01T00:00:00Z&to=2026-11-08T00:00:00Z", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doJSON(t, router, http.MethodGet, "/incident-reports/trend?bucket=day&from=bad", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	// 坏桶粒度
	w = doJSON(t, router, http.MethodGet, "/incident-reports/trend?bucket=year", "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestReportHandler_LeaderboardAndResponsibility(t *testing.T) {
	router, _ := setupReportRouter(t)

	w := doJSON(t, router, http.MethodGet, "/incident-reports/leaderboard", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doJSON(t, router, http.MethodGet, "/incident-reports/leaderboard?view=bogus", "")
	require.GreaterOrEqual(t, w.Code, 400)
	w = doJSON(t, router, http.MethodGet, "/incident-reports/leaderboard?board=bogus", "")
	require.GreaterOrEqual(t, w.Code, 400)

	// 责任人报告仅 week/month 档
	w = doJSON(t, router, http.MethodGet, "/incident-reports/responsibility?period=week", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doJSON(t, router, http.MethodGet, "/incident-reports/responsibility?period=month", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doJSON(t, router, http.MethodGet, "/incident-reports/responsibility?period=year", "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestReportHandler_RepushAndStored(t *testing.T) {
	router, _ := setupReportRouter(t)

	// 坏 id / 不存在的报表
	w := doJSON(t, router, http.MethodPost, "/incident-reports/abc/repush", "")
	require.GreaterOrEqual(t, w.Code, 400)
	w = doJSON(t, router, http.MethodPost, "/incident-reports/999999/repush", "")
	require.GreaterOrEqual(t, w.Code, 400)

	// stored 空列表
	w = doJSON(t, router, http.MethodGet, "/incident-reports/stored?period=week&page=1&pageSize=10", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
