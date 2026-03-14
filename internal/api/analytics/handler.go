package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/common/requestbind"
	"github.com/cuihairu/croupier/internal/common/response"
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
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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
	// 支持从查询参数或 JSON body 获取参数
	var req RealtimeRequest
	if c.Request.Method == "GET" {
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误: " + err.Error()})
			return
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误: " + err.Error()})
			return
		}
	}

	// 设置 SSE 响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// 监听客户端断开连接
	go func() {
		<-ctx.Done()
		slog.InfoContext(ctx, "SSE client disconnected", "path", "/analytics/realtime")
	}()

	// 定时发送数据（每2秒一次）
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// 发送初始连接成功消息
	fmt.Fprintf(c.Writer, "event: connected\n")
	fmt.Fprintf(c.Writer, "data: {\"status\":\"connected\"}\n\n")
	c.Writer.Flush()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "SSE connection closed")
			return
		case <-ticker.C:
			// 获取实时数据
			resp, err := h.service.Realtime(ctx, &req)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to get realtime data", "error", err)
				// 发送错误事件，但不关闭连接
				fmt.Fprintf(c.Writer, "event: error\n")
				errorData, _ := json.Marshal(map[string]string{"error": err.Error()})
				fmt.Fprintf(c.Writer, "data: %s\n\n", errorData)
				c.Writer.Flush()
				continue
			}

			// 发送数据事件
			data, err := json.Marshal(resp)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to marshal realtime data", "error", err)
				continue
			}

			fmt.Fprintf(c.Writer, "event: message\n")
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
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
