package analytics_retention

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

const (
	defaultRetentionWindowDays = 30
	defaultLevelWindowDays     = 14
	maxBehaviorPageSize        = 3000
	maxLevelEntries            = 100
	maxMapPoints               = 200
)

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

func parseRetentionValues(raw datatypes.JSON) []float64 {
	if len(raw) == 0 {
		return []float64{}
	}
	var values []float64
	if err := json.Unmarshal(raw, &values); err != nil {
		return []float64{}
	}
	return values
}

func safeDivide(num float64, denom float64) float64 {
	if denom == 0 {
		return 0
	}
	return num / denom
}

func loadBehaviorEvents(ctx context.Context, svcCtx *svc.ServiceContext, gameID, env string, start, end time.Time, eventTypes []string, pageSize int) ([]model.BehaviorEvent, error) {
	if svcCtx == nil || svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if pageSize <= 0 {
		pageSize = maxBehaviorPageSize
	}
	appendEvents := func(eventType string) ([]model.BehaviorEvent, error) {
		opts := model.BehaviorEventOptions{
			PaginationOptions: model.PaginationOptions{
				Page:     1,
				PageSize: pageSize,
			},
			GameID:    strings.TrimSpace(gameID),
			Env:       strings.TrimSpace(env),
			EventType: strings.TrimSpace(eventType),
			StartTime: start,
			EndTime:   end,
		}
		events, _, err := svcCtx.BehaviorModel.ListEvents(ctx, opts)
		if err != nil {
			return nil, err
		}
		return events, nil
	}

	var all []model.BehaviorEvent
	if len(eventTypes) == 0 {
		events, err := appendEvents("")
		if err != nil {
			return nil, err
		}
		all = append(all, events...)
		return all, nil
	}

	for _, evt := range eventTypes {
		events, err := appendEvents(evt)
		if err != nil {
			return nil, err
		}
		all = append(all, events...)
	}
	return all, nil
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

func formatAny(v interface{}) string {
	switch value := v.(type) {
	case string:
		return value
	default:
		return fmt.Sprintf("%v", value)
	}
}

func sortLevelMetrics(levels []types.LevelMetrics) {
	sort.Slice(levels, func(i, j int) bool {
		if levels[i].Attempts == levels[j].Attempts {
			return levels[i].LevelId < levels[j].LevelId
		}
		return levels[i].Attempts > levels[j].Attempts
	})
}

func sortEpisodeMetrics(items []types.EpisodeMetrics) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Players == items[j].Players {
			return items[i].EpisodeId < items[j].EpisodeId
		}
		return items[i].Players > items[j].Players
	})
}

func sortMaps(items []types.MapMetrics) {
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
