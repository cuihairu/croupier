package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNotificationsGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsNotificationsGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNotificationsGetLogic {
	return &OpsNotificationsGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNotificationsGetLogic) OpsNotificationsGet() (*types.OpsNotificationsResponse, error) {
	chs, rules := l.svcCtx.NotificationsSnapshot()
	resp := &types.OpsNotificationsResponse{
		Channels: make([]types.OpsNotificationChannel, 0, len(chs)),
		Rules:    make([]types.OpsNotificationRule, 0, len(rules)),
	}
	for _, ch := range chs {
		resp.Channels = append(resp.Channels, types.OpsNotificationChannel{
			Id:       ch.ID,
			Type:     ch.Type,
			Url:      ch.URL,
			Secret:   ch.Secret,
			Provider: ch.Provider,
			Account:  ch.Account,
			From:     ch.From,
			To:       ch.To,
		})
	}
	for _, rule := range rules {
		resp.Rules = append(resp.Rules, types.OpsNotificationRule{
			Event:         rule.Event,
			Channels:      append([]string{}, rule.Channels...),
			ThresholdDays: rule.ThresholdDays,
		})
	}
	return resp, nil
}
