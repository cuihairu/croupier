// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_behavior

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type BehaviorFunnelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为漏斗
func NewBehaviorFunnelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorFunnelLogic {
	return &BehaviorFunnelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorFunnelLogic) BehaviorFunnel(req *types.BehaviorFunnelRequest) (resp *types.BehaviorFunnelResponse, err error) {
	if l.svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}
	if len(req.Steps) == 0 {
		return nil, errors.New("需要至少一个漏斗步骤")
	}

	steps := make([]string, 0, len(req.Steps))
	seen := map[string]struct{}{}
	for _, step := range req.Steps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		if _, ok := seen[step]; ok {
			continue
		}
		seen[step] = struct{}{}
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		return nil, errors.New("漏斗步骤不能为空")
	}

	start, end, err := utils.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.BehaviorEventOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: 5000,
		},
		GameID:    strings.TrimSpace(req.GameId),
		Env:       strings.TrimSpace(req.Env),
		StartTime: start,
		EndTime:   end,
	}

	events, _, err := l.svcCtx.BehaviorModel.ListEvents(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	stepCounts := make([]int, len(steps))
	uniqueUsers := make(map[string]struct{})

	grouped := groupEventsByUserForFunnel(events, steps)
	for _, list := range grouped {
		progress := 0
		for _, ev := range list {
			if progress >= len(steps) {
				break
			}
			if strings.EqualFold(ev.EventType, steps[progress]) {
				stepCounts[progress]++
				progress++
			}
		}
	}

	for user := range grouped {
		uniqueUsers[user] = struct{}{}
	}

	responseSteps := make([]types.FunnelStep, len(steps))
	var prev int
	for i, step := range steps {
		count := stepCounts[i]
		var conversion float64
		var dropOff float64
		if i == 0 {
			total := len(uniqueUsers)
			if total > 0 {
				conversion = float64(count) / float64(total)
				dropOff = 1 - conversion
			}
			prev = count
		} else {
			if prev > 0 {
				conversion = float64(count) / float64(prev)
				dropOff = 1 - conversion
			}
			prev = count
		}
		responseSteps[i] = types.FunnelStep{
			Step:           step,
			Users:          count,
			ConversionRate: roundPercentage(conversion),
			DropOffRate:    roundPercentage(dropOff),
		}
	}

	return &types.BehaviorFunnelResponse{
		Steps: responseSteps,
	}, nil
}

func groupEventsByUserForFunnel(events []model.BehaviorEvent, filterSteps []string) map[string][]model.BehaviorEvent {
	stepSet := make(map[string]struct{}, len(filterSteps))
	for _, step := range filterSteps {
		stepSet[strings.ToLower(step)] = struct{}{}
	}

	grouped := make(map[string][]model.BehaviorEvent)
	for _, ev := range events {
		user := strings.TrimSpace(ev.UserID)
		if user == "" {
			continue
		}
		if _, ok := stepSet[strings.ToLower(ev.EventType)]; !ok {
			continue
		}
		grouped[user] = append(grouped[user], ev)
	}

	for user := range grouped {
		list := grouped[user]
		sort.Slice(list, func(i, j int) bool {
			return list[i].OccurredAt.Before(list[j].OccurredAt)
		})
		grouped[user] = list
	}
	return grouped
}

func roundPercentage(value float64) float64 {
	return math.Round(value*10000) / 100 // percent with two decimals
}
