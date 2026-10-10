package incident

import (
	"time"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// 报表 API 的 HTTP 入口（docs/design/incident-reports.md §5/§7）。挂在独立
// 组 /incident-reports，避开 /incidents/:id 通配段。

// GetSummary handles GET /incident-reports/summary?period=week&key=2026-W41。
func (h *Handler) GetSummary(c *gin.Context) {
	periodType := parsePeriodQuery(c)
	resp, err := h.service.ReportSummary(c.Request.Context(), periodType, c.Query("key"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// GetTrend handles GET /incident-reports/trend?bucket=day&from=...&to=...。
func (h *Handler) GetTrend(c *gin.Context) {
	bucket := c.Query("bucket")
	var from, to *time.Time
	if v := c.Query("from"); v != "" {
		if t, err := parseTime(v); err == nil {
			from = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := parseTime(v); err == nil {
			to = &t
		}
	}
	resp, err := h.service.ReportTrend(c.Request.Context(), bucket, from, to)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// GetLeaderboard handles GET /incident-reports/leaderboard?period=week&view=responsible&board=incidents。
func (h *Handler) GetLeaderboard(c *gin.Context) {
	resp, err := h.service.ReportLeaderboard(c.Request.Context(),
		parsePeriodQuery(c), c.Query("key"), c.Query("view"), c.Query("board"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// GetResponsibility handles GET /incident-reports/responsibility?period=week&key=2026-W41（仅周/月档）。
func (h *Handler) GetResponsibility(c *gin.Context) {
	resp, err := h.service.ReportResponsibility(c.Request.Context(), parsePeriodQuery(c), c.Query("key"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RepushReport handles POST /incident-reports/:id/repush（§6 手动重推：
// 按存库 payload 同 event_id 重发，不改 payload；回执覆盖 PushStatus）。
func (h *Handler) RepushReport(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.RepushReport(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ListStoredReports handles GET /incident-reports/stored?period=week&page=1&pageSize=20。
func (h *Handler) ListStoredReports(c *gin.Context) {
	resp, err := h.service.ListStoredReports(c.Request.Context(), c.Query("period"),
		int(parseUintQuery(c, "page")), int(parseUintQuery(c, "pageSize")))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// parsePeriodQuery 档位 query 解析（缺省 week）。
func parsePeriodQuery(c *gin.Context) string {
	if v := c.Query("period"); v != "" {
		return v
	}
	return PeriodWeek
}
