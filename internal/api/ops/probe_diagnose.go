package ops

// 手动诊断探针（agent-health-recovery-review §4.2/§4.3，R3 server 侧内核
// 装配）：对 server 本机探针面已配置的目标即时探一次，同步返回结果。与
// 周期探测共用同一探针实例与状态机——手动诊断不是旁路第二套事实。

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
)

// ProbeDiagnoseRequest 手动诊断请求。
type ProbeDiagnoseRequest struct {
	// Target server 探针面已配置的目标名。
	Target string `json:"target"`
}

// ProbeDiagnoseResponse 单次探测结果（内核 Result 的 API 投影）。
type ProbeDiagnoseResponse struct {
	Target    string            `json:"target"`
	Available bool              `json:"available"`
	OK        bool              `json:"ok"`
	LatencyMs int64             `json:"latencyMs"`
	Reason    string            `json:"reason,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
	// WindowOpen 探测后窗口是否开着（达阈值才算——单次失败只计数不开窗）。
	WindowOpen bool `json:"windowOpen"`
}

// ProbeDiagnose 手动诊断：目标未配置 404；探测面未装配 404（不同 message
// 区分）；空目标 422。
func (s *Service) ProbeDiagnose(ctx context.Context, req *ProbeDiagnoseRequest) (*ProbeDiagnoseResponse, error) {
	if req == nil || strings.TrimSpace(req.Target) == "" {
		return nil, errorx.NewValidationError("target 不能为空")
	}
	if s.svcCtx == nil || s.svcCtx.ServerProbes == nil {
		return nil, errorx.NewNotFound("server 探测面未配置（probe.targets 为空）")
	}
	res, ok := s.svcCtx.ServerProbes.ProbeOnceByName(strings.TrimSpace(req.Target))
	if !ok {
		return nil, errorx.NewNotFound("目标未配置为 server 探针")
	}
	return &ProbeDiagnoseResponse{
		Target:     res.Target,
		Available:  res.Available,
		OK:         res.OK,
		LatencyMs:  res.LatencyMS,
		Reason:     res.Err,
		Details:    res.Details,
		WindowOpen: res.WindowOpen,
	}, nil
}

// ProbeDiagnoseHandler handles POST /api/v1/ops/probe/diagnose.
func (h *Handler) ProbeDiagnoseHandler(c *gin.Context) {
	var req ProbeDiagnoseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.ProbeDiagnose(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
