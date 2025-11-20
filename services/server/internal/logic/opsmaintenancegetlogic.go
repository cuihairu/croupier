package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsMaintenanceGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsMaintenanceGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMaintenanceGetLogic {
	return &OpsMaintenanceGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMaintenanceGetLogic) OpsMaintenanceGet() (*types.OpsMaintenanceResponse, error) {
	windows := l.svcCtx.MaintenanceSnapshot()
	resp := &types.OpsMaintenanceResponse{
		Windows: make([]types.OpsMaintenanceWindow, 0, len(windows)),
	}
	for _, w := range windows {
		resp.Windows = append(resp.Windows, types.OpsMaintenanceWindow{
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
