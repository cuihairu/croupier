package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

const (
	defaultRetentionWindowDays = 30
	defaultLevelWindowDays     = 14
	maxBehaviorPageSize        = 3000
	maxLevelEntries            = 100
	maxMapPoints               = 200
)

func retention(ctx context.Context, svcCtx *svc.ServiceContext, req *RetentionRequest) (*RetentionResponse, error) {
	if svcCtx.RetentionModel == nil {
		return nil, errors.New("retention model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolveRetentionRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	gameID := svc.ResolveGameID(ctx, req.GameId)
	env := svc.ResolveEnv(ctx, req.Env)
	cohortName := strings.TrimSpace(req.Cohort)

	cohorts, err := svcCtx.RetentionModel.ListCohorts(ctx, gameID, env, cohortName)
	if err != nil {
		return nil, err
	}

	items := make([]RetentionCohort, 0, len(cohorts))
	for _, cohort := range cohorts {
		if !start.IsZero() && cohort.WindowStart.Before(start) {
			continue
		}
		if !end.IsZero() && cohort.WindowStart.After(end) {
			continue
		}
		items = append(items, RetentionCohort{
			Cohort:    cohort.Cohort,
			Users:     cohort.Users,
			Retention: parseRetentionValues(cohort.Retention),
		})
	}

	return &RetentionResponse{
		Cohorts: items,
	}, nil
}

func levels(ctx context.Context, svcCtx *svc.ServiceContext, req *LevelsRequest) (*LevelsResponse, error) {
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolveLevelRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	events, err := loadBehaviorEvents(ctx, svcCtx, req.GameId, req.Env, start, end, []string{"level_attempt", "level_complete"}, maxBehaviorPageSize)
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

	levels := make([]LevelMetrics, 0, len(order))
	for _, id := range order {
		stat := stats[id]
		if stat.Attempts == 0 && stat.Completions == 0 {
			continue
		}
		levels = append(levels, LevelMetrics{
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

	return &LevelsResponse{
		Levels: levels,
	}, nil
}

func levelsEpisodes(ctx context.Context, svcCtx *svc.ServiceContext, req *LevelsEpisodesRequest) (*LevelsEpisodesResponse, error) {
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolveLevelRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	events, err := loadBehaviorEvents(ctx, svcCtx, req.GameId, req.Env, start, end, []string{"episode_progress", "episode_complete"}, maxBehaviorPageSize)
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

	episodes := make([]EpisodeMetrics, 0, len(order))
	for _, id := range order {
		stat := stats[id]
		players := len(stat.players)
		completed := len(stat.completedUsers)
		if players == 0 && completed > 0 {
			players = completed
		}
		episodes = append(episodes, EpisodeMetrics{
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

	return &LevelsEpisodesResponse{
		Episodes: episodes,
	}, nil
}

func levelsMaps(ctx context.Context, svcCtx *svc.ServiceContext, req *LevelsMapsRequest) (*LevelsMapsResponse, error) {
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolveLevelRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	eventTypes := []string{"map_event", "map_heat", "map_position", "map_death"}
	events, err := loadBehaviorEvents(ctx, svcCtx, req.GameId, req.Env, start, end, eventTypes, maxBehaviorPageSize)
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

	maps := make([]MapMetrics, 0, len(order))
	for _, id := range order {
		stat := stats[id]
		heat := make([]map[string]float64, len(stat.heatPoints))
		copy(heat, stat.heatPoints)
		deaths := make([]map[string]float64, len(stat.deathPoints))
		copy(deaths, stat.deathPoints)
		maps = append(maps, MapMetrics{
			MapId:      id,
			HeatMap:    heat,
			DeathSpots: deaths,
		})
	}

	sortMapMetrics(maps)

	return &LevelsMapsResponse{
		Maps: maps,
	}, nil
}

// Helper types and functions for retention analytics

type levelStats struct {
	Attempts      int
	Completions   int
	DurationSum   float64
	DurationCount int
	RetriesSum    float64
	RetriesCount  int
}

type episodeStats struct {
	players        map[string]struct{}
	completedUsers map[string]struct{}
	progressSum    float64
	progressCount  int
}

type mapStats struct {
	heatPoints  []map[string]float64
	deathPoints []map[string]float64
}

func resolveRetentionRange(startRaw, endRaw string) (time.Time, time.Time, error) {
	start, end, err := normalizeRange(startRaw, endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}
	if start.IsZero() {
		start = end.Add(-defaultRetentionWindowDays * 24 * time.Hour)
	}
	return start, end, nil
}

func resolveLevelRange(startRaw, endRaw string) (time.Time, time.Time, error) {
	start, end, err := normalizeRange(startRaw, endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}
	if start.IsZero() {
		start = end.Add(-defaultLevelWindowDays * 24 * time.Hour)
	}
	return start, end, nil
}

func normalizeRange(startRaw, endRaw string) (time.Time, time.Time, error) {
	start, err := parseTime(startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := parseTime(endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		start, end = end, start
	}
	return start, end, nil
}

func parseTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
	}
	var lastErr error
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}

func parseRetentionValues(raw model.JSON) []float64 {
	if len(raw) == 0 {
		return []float64{}
	}
	var values []float64
	if err := json.Unmarshal(raw, &values); err != nil {
		return []float64{}
	}
	return values
}

func eventString(ev model.BehaviorEvent, keys ...string) string {
	data := map[string]interface{}(ev.Data)
	for _, key := range keys {
		if val, ok := data[key]; ok {
			switch v := val.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			default:
				str := strings.TrimSpace(formatAny(v))
				if str != "" {
					return str
				}
			}
		}
	}
	return ""
}

func eventFloat(ev model.BehaviorEvent, keys ...string) float64 {
	data := map[string]interface{}(ev.Data)
	for _, key := range keys {
		if val, ok := data[key]; ok {
			if f, ok := toFloat(val); ok {
				return f
			}
		}
	}
	return 0
}

func eventBool(ev model.BehaviorEvent, keys ...string) bool {
	data := map[string]interface{}(ev.Data)
	for _, key := range keys {
		if val, ok := data[key]; ok {
			switch v := val.(type) {
			case bool:
				return v
			case string:
				trim := strings.TrimSpace(strings.ToLower(v))
				return trim == "true" || trim == "1" || trim == "yes"
			case float64:
				return v != 0
			case int:
				return v != 0
			}
		}
	}
	return false
}

func toFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		if err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func sortLevelMetrics(levels []LevelMetrics) {
	sort.Slice(levels, func(i, j int) bool {
		if levels[i].Attempts == levels[j].Attempts {
			return levels[i].LevelId < levels[j].LevelId
		}
		return levels[i].Attempts > levels[j].Attempts
	})
}

func sortEpisodeMetrics(items []EpisodeMetrics) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Players == items[j].Players {
			return items[i].EpisodeId < items[j].EpisodeId
		}
		return items[i].Players > items[j].Players
	})
}

func sortMapMetrics(items []MapMetrics) {
	sort.Slice(items, func(i, j int) bool {
		if mapPoints(items[i].HeatMap) == mapPoints(items[j].HeatMap) {
			return items[i].MapId < items[j].MapId
		}
		return mapPoints(items[i].HeatMap) > mapPoints(items[j].HeatMap)
	})
}

func mapPoints(value interface{}) int {
	switch v := value.(type) {
	case []map[string]float64:
		return len(v)
	case []map[string]interface{}:
		return len(v)
	case []interface{}:
		return len(v)
	default:
		return 0
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
