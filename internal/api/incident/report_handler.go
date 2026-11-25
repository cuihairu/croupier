package incident

import (
	"time"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// HTTP entry points for the report APIs
// (docs/design/incident-reports.md §5/§7). Mounted on the dedicated
// /incident-reports group so it does not collide with the /incidents/:id
// wildcard segment.

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

// GetResponsibility handles GET /incident-reports/responsibility?period=week&key=2026-W41 (week/month tiers only).
func (h *Handler) GetResponsibility(c *gin.Context) {
	resp, err := h.service.ReportResponsibility(c.Request.Context(), parsePeriodQuery(c), c.Query("key"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RepushReport handles POST /incident-reports/:id/repush (§6 manual repush:
// re-sends the stored payload under the same event_id without altering it;
// the receipt overwrites PushStatus).
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
		parseIntQuery(c, "page"), parseIntQuery(c, "pageSize"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// parsePeriodQuery parses the period tier from the query (defaults to week).
func parsePeriodQuery(c *gin.Context) string {
	if v := c.Query("period"); v != "" {
		return v
	}
	return PeriodWeek
}
