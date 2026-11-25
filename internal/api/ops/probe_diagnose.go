package ops

// Manual diagnostic probe (agent-health-recovery-review §4.2/§4.3, the R3
// server-side kernel wiring): probe a target already configured on the server's
// local probe surface once, synchronously, and return the result. Shares the
// same probe instance and state machine as periodic probing — manual diagnosis
// is not a bypass second source of truth.

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
)

// ProbeDiagnoseRequest is a manual diagnosis request.
type ProbeDiagnoseRequest struct {
	// Target is a target name configured on the server's probe surface.
	Target string `json:"target"`
}

// ProbeDiagnoseResponse is a single probe result (the API projection of the kernel Result).
type ProbeDiagnoseResponse struct {
	Target    string            `json:"target"`
	Available bool              `json:"available"`
	OK        bool              `json:"ok"`
	LatencyMs int64             `json:"latencyMs"`
	Reason    string            `json:"reason,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
	// WindowOpen reports whether the window is open after the probe (reaching the
	// threshold is what counts — a single failure only increments the count, it
	// does not open the window).
	WindowOpen bool `json:"windowOpen"`
}

// ProbeDiagnose is the manual diagnosis: target not configured → 404; probe
// surface not wired → 404 (distinguished by different messages); empty target
// → 422.
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
