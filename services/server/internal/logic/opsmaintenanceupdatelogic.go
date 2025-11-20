package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsMaintenanceUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsMaintenanceUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMaintenanceUpdateLogic {
	return &OpsMaintenanceUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMaintenanceUpdateLogic) OpsMaintenanceUpdate(req *types.OpsMaintenanceUpdateRequest) (*types.GenericOkResponse, error) {
	if req == nil {
		return nil, ErrInvalidRequest
	}
	windows := make([]svc.MaintenanceWindow, 0, len(req.Windows))
	for _, w := range req.Windows {
		start, err := parseRFC3339Flexible(w.Start)
		if err != nil {
			return nil, ErrInvalidRequest
		}
		end, err := parseRFC3339Flexible(w.End)
		if err != nil {
			return nil, ErrInvalidRequest
		}
		windows = append(windows, svc.MaintenanceWindow{
			ID:          strings.TrimSpace(w.Id),
			GameID:      strings.TrimSpace(w.GameId),
			Env:         strings.TrimSpace(w.Env),
			Start:       start,
			End:         end,
			Message:     w.Message,
			BlockWrites: w.BlockWrites,
		})
	}
	if err := l.svcCtx.UpdateMaintenance(windows); err != nil {
		return nil, err
	}
	return &types.GenericOkResponse{Ok: true}, nil
}
