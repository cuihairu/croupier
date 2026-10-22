package ops

import (
	"errors"
	"strconv"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

type SyntheticsResult struct {
	ID         uint   `json:"id"`
	AgentID    string `json:"agentId"`
	GameID     string `json:"gameId"`
	Env        string `json:"env"`
	Kind       string `json:"kind"`
	Target     string `json:"target"`
	Status     string `json:"status"`
	LatencyMS  int64  `json:"latencyMs"`
	ErrorClass string `json:"errorClass"`
	Error      string `json:"error"`
	CreatedAt  int64  `json:"createdAt"`
	// TargetCategory 执行时类别快照（目标分类设计 §4.4：历史记录保留执行时
	// 类别快照，取自落库 payload，不受当前目标配置改写）；旧记录无该位=空。
	TargetCategory string `json:"targetCategory,omitempty"`
}
type SyntheticsResultsResponse struct {
	Items []SyntheticsResult `json:"items"`
}

// SyntheticsResults returns durable probe records for the requested game/environment.
func (h *Handler) SyntheticsResults(c *gin.Context) {
	gameID, env := c.Query("gameId"), c.Query("env")
	if gameID == "" || env == "" {
		response.Error(c, errors.New("gameId and env are required"))
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	rows, err := model.NewProbeEventModel(h.service.svcCtx.DB).Recent(c.Request.Context(), gameID, env, c.Query("agentId"), limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	out := SyntheticsResultsResponse{Items: make([]SyntheticsResult, 0, len(rows))}
	for _, row := range rows {
		item := SyntheticsResult{ID: row.ID, AgentID: row.AgentID, GameID: row.GameID, Env: row.Env, Kind: row.Kind, Target: row.Target, Status: row.Status, LatencyMS: row.LatencyMS, ErrorClass: row.ErrorClass, CreatedAt: row.CreatedAt.UnixMilli()}
		ev := &opsv1.AlertEvent{}
		if proto.Unmarshal(row.Payload, ev) == nil {
			// 执行时快照优先：事件级字段缺失时退回消息体（detail/window 内也带）。
			item.TargetCategory = ev.GetTargetCategory()
			if d := ev.GetDetail(); d != nil {
				item.Error = d.GetErr()
				if item.TargetCategory == "" {
					item.TargetCategory = d.GetTargetCategory()
				}
			} else if w := ev.GetWindow(); w != nil {
				item.Error = w.GetLastReason()
				if item.TargetCategory == "" {
					item.TargetCategory = w.GetTargetCategory()
				}
			}
		}
		out.Items = append(out.Items, item)
	}
	response.Success(c, out)
}
