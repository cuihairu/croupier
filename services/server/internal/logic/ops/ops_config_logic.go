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

type OpsConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取运维配置
func NewOpsConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsConfigLogic {
	return &OpsConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsConfigLogic) OpsConfig(req *types.OpsConfigRequest) (*types.OpsConfigResponse, error) {
	state := snapshotOpsState(l.svcCtx)

	data := map[string]interface{}{
		"alertmanager_url":    state.Config.AlertmanagerURL,
		"grafana_explore_url": state.Config.GrafanaExploreURL,
		"jaeger_url":          state.Config.JaegerURL,
		"maintenance": map[string]interface{}{
			"windows":   state.Maintenance.Windows,
			"updatedAt": utils.FormatTimestamp(state.Maintenance.UpdatedAt),
		},
		"notifications": map[string]interface{}{
			"channels":  state.Notifications.Channels,
			"rules":     state.Notifications.Rules,
			"updatedAt": utils.FormatTimestamp(state.Notifications.UpdatedAt),
		},
		"alerts": map[string]interface{}{
			"silences":  state.Alerts.Silences,
			"updatedAt": utils.FormatTimestamp(state.Alerts.UpdatedAt),
		},
		"health": map[string]interface{}{
			"checks": state.Health.Checks,
			"status": state.Health.Status,
		},
		"mq": map[string]interface{}{
			"type":      state.MQ.Type,
			"redis":     state.MQ.Redis,
			"kafka":     state.MQ.Kafka,
			"lengths":   state.MQ.Lengths,
			"groups":    state.MQ.Groups,
			"updatedAt": utils.FormatTimestamp(state.MQ.UpdatedAt),
		},
	}

	return &types.OpsConfigResponse{
		Code:    0,
		Message: "OK",
		Data:    data,
	}, nil
}
