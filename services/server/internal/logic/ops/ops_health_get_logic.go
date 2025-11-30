// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsHealthGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取健康状态
func NewOpsHealthGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthGetLogic {
	return &OpsHealthGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthGetLogic) OpsHealthGet(req *types.OpsHealthGetRequest) (*types.OpsHealthGetResponse, error) {
	state := snapshotOpsState(l.svcCtx)
	status := make([]interface{}, 0, len(state.Health.Status))
	for _, st := range state.Health.Status {
		status = append(status, map[string]interface{}{
			"id":         st.ID,
			"ok":         st.OK,
			"latency_ms": st.LatencyMS,
			"error":      st.Error,
			"checked_at": utils.FormatTimestamp(st.CheckedAt),
		})
	}

	return &types.OpsHealthGetResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"checks": state.Health.Checks,
			"status": status,
		},
	}, nil
}
