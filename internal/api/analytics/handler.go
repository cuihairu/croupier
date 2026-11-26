package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/common/requestbind"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/gin-gonic/gin"
)

func bindAnalyticsRequest(c *gin.Context, req interface{}) error {
	if c.Request.Method == "GET" {
		return requestbind.BindQueryCompat(c, req)
	}
	return c.ShouldBindJSON(req)
}

type Handler struct {
	service *Service
	sse     config.SSEConfig
	// facade is the aggregate-query facade (MOB-ANA-001): wired up together with the
	// handler; per-domain handler routing is unaffected.
	facade *Facade
}

func NewHandler(service *Service, sse config.SSEConfig) *Handler {
	return &Handler{service: service, sse: sse, facade: NewFacade(service)}
}

// Behavior analytics handlers

func (h *Handler) Behavior(c *gin.Context) {
	var req BehaviorRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Behavior(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) BehaviorEvents(c *gin.Context) {
	var req BehaviorEventsRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.BehaviorEvents(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) BehaviorAdoption(c *gin.Context) {
	var req BehaviorAdoptionRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.BehaviorAdoption(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) BehaviorAdoptionBreakdown(c *gin.Context) {
	var req BehaviorAdoptionBreakdownRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.BehaviorAdoptionBreakdown(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) BehaviorFunnel(c *gin.Context) {
	var req BehaviorFunnelRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.BehaviorFunnel(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) BehaviorPaths(c *gin.Context) {
	var req BehaviorPathsRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.BehaviorPaths(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Overview analytics handlers

func (h *Handler) Overview(c *gin.Context) {
	var req OverviewRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Overview(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Realtime(c *gin.Context) {
	var req RealtimeRequest
	// Use compat binder so `gameId` query param works with json tags.
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	// Set the SSE response headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a cancellable context
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Watch for client disconnect
	go func() {
		<-ctx.Done()
		slog.InfoContext(ctx, "SSE client disconnected", "path", "/analytics/realtime")
	}()

	// Push cadence follows the global SSE config (yaml sse.updateInterval,
	// default 60s). Realtime analytics does not need sub-minute freshness;
	// every push costs three aggregate queries against the behavior store.
	interval := time.Duration(h.sse.GetUpdateInterval()) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send the initial connected message
	_, _ = fmt.Fprintf(c.Writer, "event: connected\n")
	_, _ = fmt.Fprintf(c.Writer, "data: {\"status\":\"connected\"}\n\n")
	c.Writer.Flush()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "SSE connection closed")
			return
		case <-ticker.C:
			// Fetch realtime data
			resp, err := h.service.Realtime(ctx, &req)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to get realtime data", "error", err)
				// Send an error event, but do not close the connection
				_, _ = fmt.Fprintf(c.Writer, "event: error\n")
				errorData, _ := json.Marshal(map[string]string{"error": err.Error()})
				_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", errorData)
				c.Writer.Flush()
				continue
			}

			// Send the data event.
			// resp is an already-typed *RealtimeResponse: numeric fields all go through
			// safeDivide (a zero denominator returns 0, no NaN/Inf), and TopEvents is an
			// internally built []map[string]interface{} (string/int values), so Marshal
			// always succeeds — the error branch is unreachable and has been removed.
			data, _ := json.Marshal(resp)

			_, _ = fmt.Fprintf(c.Writer, "event: message\n")
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
		}
	}
}

func (h *Handler) RealtimeSeries(c *gin.Context) {
	var req RealtimeSeriesRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.RealtimeSeries(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Ingest(c *gin.Context) {
	var req IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Ingest(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FiltersGet(c *gin.Context) {
	var req FiltersGetRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FiltersGet(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FiltersUpdate(c *gin.Context) {
	var req FiltersUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FiltersUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Payments analytics handlers

func (h *Handler) Payments(c *gin.Context) {
	var req PaymentsRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Payments(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) PaymentsIngest(c *gin.Context) {
	var req PaymentsIngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.PaymentsIngest(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) PaymentsProductTrend(c *gin.Context) {
	var req PaymentsProductTrendRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.PaymentsProductTrend(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) PaymentsSummary(c *gin.Context) {
	var req PaymentsSummaryRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.PaymentsSummary(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) PaymentsTransactions(c *gin.Context) {
	var req PaymentsTransactionsRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.PaymentsTransactions(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Retention analytics handlers

func (h *Handler) Retention(c *gin.Context) {
	var req RetentionRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Retention(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Levels(c *gin.Context) {
	var req LevelsRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Levels(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) LevelsEpisodes(c *gin.Context) {
	var req LevelsEpisodesRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.LevelsEpisodes(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) LevelsMaps(c *gin.Context) {
	var req LevelsMapsRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.LevelsMaps(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// InvocationsTrend handles GET /analytics/invocations/trend.
func (h *Handler) InvocationsTrend(c *gin.Context) {
	var req InvocationsTrendRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.InvocationsTrend(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// InvocationsSummary handles GET /analytics/invocations/summary.
func (h *Handler) InvocationsSummary(c *gin.Context) {
	var req InvocationsSummaryRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.InvocationsSummary(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// InvocationsList handles GET /analytics/invocations.
func (h *Handler) InvocationsList(c *gin.Context) {
	var req InvocationsListRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.InvocationsList(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// warehouseError maps warehouse errors: disabled → 503, query failure → 500.
func warehouseError(c *gin.Context, err error) {
	if errors.Is(err, errWarehouseDisabled) {
		response.ServiceUnavailable(c, "分析仓库未启用")
		return
	}
	response.Error(c, err)
}

// WarehouseDAU handles GET /analytics/warehouse/dau.
func (h *Handler) WarehouseDAU(c *gin.Context) {
	var req WarehouseDAURequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.WarehouseDAU(c.Request.Context(), &req)
	if err != nil {
		warehouseError(c, err)
		return
	}
	response.Success(c, resp)
}

// WarehouseOnline handles GET /analytics/warehouse/online.
func (h *Handler) WarehouseOnline(c *gin.Context) {
	var req WarehouseOnlineRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.WarehouseOnline(c.Request.Context(), &req)
	if err != nil {
		warehouseError(c, err)
		return
	}
	response.Success(c, resp)
}

// WarehouseRevenue handles GET /analytics/warehouse/revenue.
func (h *Handler) WarehouseRevenue(c *gin.Context) {
	var req WarehouseRevenueRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.WarehouseRevenue(c.Request.Context(), &req)
	if err != nil {
		warehouseError(c, err)
		return
	}
	response.Success(c, resp)
}

// WarehouseSessions handles GET /analytics/warehouse/sessions (DATA-037).
func (h *Handler) WarehouseSessions(c *gin.Context) {
	var req WarehouseSessionsRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.WarehouseSessions(c.Request.Context(), &req)
	if err != nil {
		warehouseError(c, err)
		return
	}
	response.Success(c, resp)
}
