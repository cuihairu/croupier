// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNotificationsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新通知配置
func NewOpsNotificationsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNotificationsUpdateLogic {
	return &OpsNotificationsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNotificationsUpdateLogic) OpsNotificationsUpdate(req *types.OpsNotificationsUpdateRequest) (*types.OpsNotificationsUpdateResponse, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}

	channels := make([]svc.OpsNotificationChannel, 0, len(req.Channels))
	seenChannels := map[string]struct{}{}
	for _, ch := range req.Channels {
		id := strings.TrimSpace(ch.ID)
		if id == "" {
			return nil, errors.New("渠道ID不能为空")
		}
		if _, ok := seenChannels[id]; ok {
			return nil, fmt.Errorf("渠道ID重复: %s", id)
		}
		seenChannels[id] = struct{}{}
		channels = append(channels, svc.OpsNotificationChannel{
			ID:     id,
			Type:   strings.TrimSpace(ch.Type),
			URL:    strings.TrimSpace(ch.URL),
			Secret: strings.TrimSpace(ch.Secret),
		})
	}

	rules := make([]svc.OpsNotificationRule, 0, len(req.Rules))
	for _, rule := range req.Rules {
		ev := strings.TrimSpace(rule.Event)
		if ev == "" {
			return nil, errors.New("通知事件不能为空")
		}
		ruleChannels := make([]string, 0, len(rule.Channels))
		for _, id := range rule.Channels {
			cid := strings.TrimSpace(id)
			if cid == "" {
				continue
			}
			if _, ok := seenChannels[cid]; !ok {
				return nil, fmt.Errorf("引用了未定义的渠道: %s", cid)
			}
			ruleChannels = append(ruleChannels, cid)
		}
		if len(ruleChannels) == 0 {
			return nil, fmt.Errorf("事件 %s 至少需要一个渠道", ev)
		}
		rules = append(rules, svc.OpsNotificationRule{
			Event:         ev,
			Channels:      ruleChannels,
			ThresholdDays: rule.ThresholdDays,
		})
	}

	state, err := updateOpsState(l.svcCtx, func(st *svc.OpsState) {
		st.Notifications.Channels = channels
		st.Notifications.Rules = rules
		st.Notifications.UpdatedAt = time.Now().UTC()
	})
	if err != nil {
		return nil, err
	}

	return &types.OpsNotificationsUpdateResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"channels":  state.Notifications.Channels,
			"rules":     state.Notifications.Rules,
			"updatedAt": state.Notifications.UpdatedAt,
		},
	}, nil
}
