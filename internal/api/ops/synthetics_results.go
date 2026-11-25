package ops

import (
	"strconv"

	"github.com/cuihairu/croupier/internal/common/errorx"
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
	// TargetCategory is the category snapshot captured at execution time
	// (target-category design §4.4: history records keep the execution-time
	// category snapshot taken from the landed payload, unaffected by later
	// target config rewrites); older records without this field are empty.
	TargetCategory string `json:"targetCategory,omitempty"`
}
type SyntheticsResultsResponse struct {
	Items []SyntheticsResult `json:"items"`
}

// SyntheticsResults returns durable probe records for the requested game/environment.
func (h *Handler) SyntheticsResults(c *gin.Context) {
	gameID, env := c.Query("gameId"), c.Query("env")
	if gameID == "" || env == "" {
		// AUD-047: missing params are a client input error, surfaced as 400
		// rather than falling into an opaque 500.
		response.Error(c, errorx.NewBadRequest("gameId and env are required"))
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
			// Execution-time snapshot takes precedence: when event-level fields
			// are missing, fall back to the message body (also carried inside
			// detail/window).
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
