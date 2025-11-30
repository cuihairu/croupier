// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsMaintenanceUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新维护模式
func NewOpsMaintenanceUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMaintenanceUpdateLogic {
	return &OpsMaintenanceUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMaintenanceUpdateLogic) OpsMaintenanceUpdate(req *types.OpsMaintenanceUpdateRequest) (*types.OpsMaintenanceUpdateResponse, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}
	windows := make([]svc.OpsMaintenanceWindow, 0, len(req.Windows))
	seen := map[string]struct{}{}
	for _, win := range req.Windows {
		id := strings.TrimSpace(win.ID)
		if id == "" {
			return nil, errors.New("维护窗口ID不能为空")
		}
		if _, exists := seen[id]; exists {
			return nil, errors.New("维护窗口ID重复: " + id)
		}
		seen[id] = struct{}{}
		windows = append(windows, svc.OpsMaintenanceWindow{
			ID:          id,
			GameID:      strings.TrimSpace(win.GameID),
			Env:         strings.TrimSpace(win.Env),
			Start:       strings.TrimSpace(win.Start),
			End:         strings.TrimSpace(win.End),
			Message:     strings.TrimSpace(win.Message),
			BlockWrites: win.BlockWrites,
		})
	}

	state, err := updateOpsState(l.svcCtx, func(st *svc.OpsState) {
		st.Maintenance.Windows = windows
		st.Maintenance.UpdatedAt = time.Now().UTC()
	})
	if err != nil {
		return nil, err
	}

	return &types.OpsMaintenanceUpdateResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"windows":   state.Maintenance.Windows,
			"updatedAt": state.Maintenance.UpdatedAt,
		},
	}, nil
}
