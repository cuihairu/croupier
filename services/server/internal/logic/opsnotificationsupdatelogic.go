package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNotificationsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsNotificationsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNotificationsUpdateLogic {
	return &OpsNotificationsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNotificationsUpdateLogic) OpsNotificationsUpdate(req *types.OpsNotificationsUpdateRequest) (*types.GenericOkResponse, error) {
	if req == nil {
		return nil, ErrInvalidRequest
	}
	channels := make([]svc.NotifyChannel, 0, len(req.Channels))
	for _, ch := range req.Channels {
		channels = append(channels, svc.NotifyChannel{
			ID:       strings.TrimSpace(ch.Id),
			Type:     strings.ToLower(strings.TrimSpace(ch.Type)),
			URL:      strings.TrimSpace(ch.Url),
			Secret:   strings.TrimSpace(ch.Secret),
			Provider: strings.TrimSpace(ch.Provider),
			Account:  strings.TrimSpace(ch.Account),
			From:     strings.TrimSpace(ch.From),
			To:       strings.TrimSpace(ch.To),
		})
	}
	rules := make([]svc.NotifyRule, 0, len(req.Rules))
	for _, rule := range req.Rules {
		chIDs := make([]string, 0, len(rule.Channels))
		for _, id := range rule.Channels {
			chIDs = append(chIDs, strings.TrimSpace(id))
		}
		rules = append(rules, svc.NotifyRule{
			Event:         strings.TrimSpace(rule.Event),
			Channels:      chIDs,
			ThresholdDays: rule.ThresholdDays,
		})
	}
	if err := l.svcCtx.UpdateNotifications(channels, rules); err != nil {
		return nil, err
	}
	return &types.GenericOkResponse{Ok: true}, nil
}
