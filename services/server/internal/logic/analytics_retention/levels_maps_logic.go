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

type LevelsMapsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type mapStats struct {
	heatPoints  []map[string]float64
	deathPoints []map[string]float64
}

// 获取地图分析
func NewLevelsMapsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LevelsMapsLogic {
	return &LevelsMapsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LevelsMapsLogic) LevelsMaps(req *types.LevelsMapsRequest) (*types.LevelsMapsResponse, error) {
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolveLevelRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	eventTypes := []string{"map_event", "map_heat", "map_position", "map_death"}
	events, err := loadBehaviorEvents(l.ctx, l.svcCtx, req.GameId, req.Env, start, end, eventTypes, maxBehaviorPageSize)
	if err != nil {
		return nil, err
	}

	stats := map[string]*mapStats{}
	order := []string{}

	for _, ev := range events {
		mapID := eventString(ev, "mapId", "map_id", "map", "scene")
		if mapID == "" {
			continue
		}
		stat := stats[mapID]
		if stat == nil {
			stat = &mapStats{}
			stats[mapID] = stat
			order = append(order, mapID)
		}

		x := eventFloat(ev, "x", "posX", "lon", "longitude")
		y := eventFloat(ev, "y", "posY", "lat", "latitude")
		value := eventFloat(ev, "value", "count", "weight")
		if value == 0 {
			value = 1
		}

		point := map[string]float64{
			"x":     x,
			"y":     y,
			"value": value,
		}

		eventName := strings.ToLower(ev.EventType)
		isDeath := strings.Contains(eventName, "death") || eventBool(ev, "death", "isDeath")
		if isDeath {
			if len(stat.deathPoints) < maxMapPoints {
				stat.deathPoints = append(stat.deathPoints, point)
			}
			continue
		}
		if len(stat.heatPoints) < maxMapPoints {
			stat.heatPoints = append(stat.heatPoints, point)
		}
	}

	maps := make([]types.MapMetrics, 0, len(order))
	for _, id := range order {
		stat := stats[id]
		heat := make([]map[string]float64, len(stat.heatPoints))
		copy(heat, stat.heatPoints)
		deaths := make([]map[string]float64, len(stat.deathPoints))
		copy(deaths, stat.deathPoints)
		maps = append(maps, types.MapMetrics{
			MapId:      id,
			HeatMap:    heat,
			DeathSpots: deaths,
		})
	}

	sortMaps(maps)

	return &types.LevelsMapsResponse{
		Maps: maps,
	}, nil
}
