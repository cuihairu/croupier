// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_retention

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LevelsEpisodesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type episodeStats struct {
	players        map[string]struct{}
	completedUsers map[string]struct{}
	progressSum    float64
	progressCount  int
}

// 获取章节分析
func NewLevelsEpisodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LevelsEpisodesLogic {
	return &LevelsEpisodesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LevelsEpisodesLogic) LevelsEpisodes(req *types.LevelsEpisodesRequest) (*types.LevelsEpisodesResponse, error) {
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolveLevelRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	events, err := loadBehaviorEvents(l.ctx, l.svcCtx, req.GameId, req.Env, start, end, []string{"episode_progress", "episode_complete"}, maxBehaviorPageSize)
	if err != nil {
		return nil, err
	}

	stats := map[string]*episodeStats{}
	order := []string{}

	for _, ev := range events {
		episodeID := eventString(ev, "episodeId", "episode_id", "episode", "chapter_id", "chapterId")
		if episodeID == "" {
			continue
		}
		stat := stats[episodeID]
		if stat == nil {
			stat = &episodeStats{
				players:        map[string]struct{}{},
				completedUsers: map[string]struct{}{},
			}
			stats[episodeID] = stat
			order = append(order, episodeID)
		}
		userID := strings.TrimSpace(ev.UserID)
		if userID != "" {
			stat.players[userID] = struct{}{}
		}

		if progress := eventFloat(ev, "progress", "completionRate", "completion_rate"); progress > 0 {
			stat.progressSum += progress
			stat.progressCount++
		}

		eventName := strings.ToLower(ev.EventType)
		if strings.Contains(eventName, "complete") || eventBool(ev, "completed", "finished") {
			if userID != "" {
				stat.completedUsers[userID] = struct{}{}
			}
		}
	}

	episodes := make([]types.EpisodeMetrics, 0, len(order))
	for _, id := range order {
		stat := stats[id]
		players := len(stat.players)
		completed := len(stat.completedUsers)
		if players == 0 && completed > 0 {
			players = completed
		}
		episodes = append(episodes, types.EpisodeMetrics{
			EpisodeId:      id,
			Players:        players,
			CompletionRate: safeDivide(float64(completed), float64(maxInt(players, 1))),
			AvgProgress:    safeDivide(stat.progressSum, float64(maxInt(stat.progressCount, 1))),
		})
	}

	sortEpisodeMetrics(episodes)
	if len(episodes) > maxLevelEntries {
		episodes = episodes[:maxLevelEntries]
	}

	return &types.LevelsEpisodesResponse{
		Episodes: episodes,
	}, nil
}
