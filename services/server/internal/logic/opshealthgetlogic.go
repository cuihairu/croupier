package logic

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsHealthGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsHealthGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthGetLogic {
	return &OpsHealthGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthGetLogic) OpsHealthGet() (*types.OpsHealthResponse, error) {
	checks, statuses := l.svcCtx.HealthSnapshot()
	resp := &types.OpsHealthResponse{
		Checks: make([]types.HealthCheck, 0, len(checks)),
		Status: make([]types.HealthStatus, 0, len(statuses)),
	}
	for _, c := range checks {
		resp.Checks = append(resp.Checks, types.HealthCheck{
			Id:          c.ID,
			Kind:        c.Kind,
			Target:      c.Target,
			Expect:      c.Expect,
			IntervalSec: c.IntervalSec,
			TimeoutMs:   c.TimeoutMs,
			Region:      c.Region,
		})
	}
	for _, st := range statuses {
		resp.Status = append(resp.Status, types.HealthStatus{
			Id:        st.ID,
			Ok:        st.OK,
			LatencyMs: st.LatencyMs,
			Error:     st.Error,
			CheckedAt: st.CheckedAt.Format(time.RFC3339),
		})
	}
	return resp, nil
}
