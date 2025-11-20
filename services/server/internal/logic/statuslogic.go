package logic

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type StatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StatusLogic {
	return &StatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StatusLogic) Status() (*types.OpsStatusResponse, error) {
	active := l.svcCtx.ActiveMaintenance(time.Now())
	resp := &types.OpsStatusResponse{
		Maintenance: make([]types.OpsMaintenanceWindow, 0, len(active)),
	}
	for _, w := range active {
		resp.Maintenance = append(resp.Maintenance, types.OpsMaintenanceWindow{
			Id:          w.ID,
			GameId:      w.GameID,
			Env:         w.Env,
			Start:       formatRFC3339(w.Start),
			End:         formatRFC3339(w.End),
			Message:     w.Message,
			BlockWrites: w.BlockWrites,
		})
	}
	return resp, nil
}
