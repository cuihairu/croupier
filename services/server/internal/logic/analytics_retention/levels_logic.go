// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_retention

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type LevelsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type levelStats struct {
	Attempts      int
	Completions   int
	DurationSum   float64
	DurationCount int
	RetriesSum    float64
	RetriesCount  int
}

// 获取关卡分析
func NewLevelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LevelsLogic {
	return &LevelsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LevelsLogic) Levels(req *types.LevelsRequest) (*types.LevelsResponse, error) {
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolveLevelRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	events, err := loadBehaviorEvents(l.ctx, l.svcCtx, req.GameId, req.Env, start, end, []string{"level_attempt", "level_complete"}, maxBehaviorPageSize)
	if err != nil {
		return nil, err
	}

	stats := map[string]*levelStats{}
	order := []string{}

	for _, ev := range events {
		levelID := eventString(ev, "levelId", "level_id", "level", "stage", "stageId")
		if levelID == "" {
			continue
		}
		stat := stats[levelID]
		if stat == nil {
			stat = &levelStats{}
			stats[levelID] = stat
			order = append(order, levelID)
		}

		eventName := strings.ToLower(ev.EventType)
		if eventName == "" {
			eventName = strings.ToLower(eventString(ev, "event", "type"))
		}

		if strings.Contains(eventName, "attempt") || strings.Contains(eventName, "start") || eventString(ev, "status") == "attempt" {
			stat.Attempts++
			if duration := eventFloat(ev, "duration", "durationMs", "duration_ms", "time"); duration > 0 {
				stat.DurationSum += duration
				stat.DurationCount++
			}
			if retries := eventFloat(ev, "retries", "retryCount", "retry_count"); retries > 0 {
				stat.RetriesSum += retries
				stat.RetriesCount++
			}
		}

		if strings.Contains(eventName, "complete") || strings.Contains(eventName, "finish") || eventBool(ev, "completed", "success", "passed") {
			stat.Completions++
		}
	}

	levels := make([]types.LevelMetrics, 0, len(order))
	for _, id := range order {
		stat := stats[id]
		if stat.Attempts == 0 && stat.Completions == 0 {
			continue
		}
		levels = append(levels, types.LevelMetrics{
			LevelId:        id,
			Attempts:       stat.Attempts,
			Completions:    stat.Completions,
			CompletionRate: safeDivide(float64(stat.Completions), float64(maxInt(stat.Attempts, 1))),
			AvgDuration:    safeDivide(stat.DurationSum, float64(maxInt(stat.DurationCount, 1))),
			AvgRetries:     safeDivide(stat.RetriesSum, float64(maxInt(stat.RetriesCount, 1))),
		})
	}

	sortLevelMetrics(levels)
	if len(levels) > maxLevelEntries {
		levels = levels[:maxLevelEntries]
	}

	return &types.LevelsResponse{
		Levels: levels,
	}, nil
}
