package analytics_overview

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
)

const (
	defaultOverviewWindowDays = 7
	defaultRealtimeWindow     = 15 * time.Minute
	maxRealtimeDurationMin    = 24 * 60
)

func resolveRange(startRaw, endRaw string, fallbackDays int) (time.Time, time.Time, error) {
	start, end, err := utils.NormalizeDateRange(startRaw, endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	now := time.Now().UTC()
	if end.IsZero() {
		end = now
	}
	if start.IsZero() {
		if fallbackDays <= 0 {
			fallbackDays = defaultOverviewWindowDays
		}
		start = end.Add(-time.Duration(fallbackDays) * 24 * time.Hour)
	}
	return start, end, nil
}

func summarizeBehaviorStats(stats []model.BehaviorDailyStat, selector func(stat model.BehaviorDailyStat) int64) []map[string]interface{} {
	points := make([]map[string]interface{}, 0, len(stats))
	for _, stat := range stats {
		points = append(points, map[string]interface{}{
			"date":  stat.Day.Format("2006-01-02"),
			"value": selector(stat),
		})
	}
	return points
}

func summarizeRevenueStats(stats []model.DailyRevenueStat) []map[string]interface{} {
	points := make([]map[string]interface{}, 0, len(stats))
	for _, stat := range stats {
		points = append(points, map[string]interface{}{
			"date":         stat.Day.Format("2006-01-02"),
			"revenue":      stat.Revenue,
			"orders":       stat.Transactions,
			"avgTicket":    safeDivide(stat.Revenue, float64(stat.Transactions)),
			"transactions": stat.Transactions,
		})
	}
	return points
}

func summarizePlayerStats(stats []model.DailyNewPlayerStat) []map[string]interface{} {
	points := make([]map[string]interface{}, 0, len(stats))
	for _, stat := range stats {
		points = append(points, map[string]interface{}{
			"date":  stat.Day.Format("2006-01-02"),
			"value": stat.Count,
		})
	}
	return points
}

func safeDivide(num float64, denom float64) float64 {
	if denom == 0 {
		return 0
	}
	return num / denom
}

func aggregateAgentMetrics(store *reg.Store, gameID, env string) (avgLatency float64, errorRate float64) {
	if store == nil {
		return 0, 0
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	var (
		latencySum float64
		latencyCnt int
		errorSum   float64
		errorCnt   int
	)

	for _, agent := range store.AgentsUnsafe() {
		if agent == nil {
			continue
		}
		if strings.TrimSpace(gameID) != "" && agent.GameID != strings.TrimSpace(gameID) {
			continue
		}
		if strings.TrimSpace(env) != "" && agent.Env != strings.TrimSpace(env) {
			continue
		}

		if v, ok := parseFloat(agent.Labels["stats.avg_latency_ms"]); ok {
			latencySum += v
			latencyCnt++
		}
		if v, ok := parseFloat(agent.Labels["stats.error_rate"]); ok {
			errorSum += v
			errorCnt++
		}
	}

	if latencyCnt > 0 {
		avgLatency = latencySum / float64(latencyCnt)
	}
	if errorCnt > 0 {
		errorRate = errorSum / float64(errorCnt)
	}
	return math.Round(avgLatency*100) / 100, math.Round(errorRate*10000) / 10000
}

func parseFloat(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

func resolveRealtimeInterval(raw string) time.Duration {
	switch strings.TrimSpace(raw) {
	case "", "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	default:
		return time.Minute
	}
}

func clampRealtimeDuration(minutes int) time.Duration {
	if minutes <= 0 {
		minutes = 60
	}
	if minutes > maxRealtimeDurationMin {
		minutes = maxRealtimeDurationMin
	}
	return time.Duration(minutes) * time.Minute
}

func buildRealtimeSeriesPoints(samples []bucketPoint) []map[string]interface{} {
	points := make([]map[string]interface{}, 0, len(samples))
	for _, sample := range samples {
		points = append(points, map[string]interface{}{
			"timestamp": utils.FormatTimestamp(sample.Timestamp),
			"value":     sample.Value,
		})
	}
	return points
}

type bucketPoint struct {
	Timestamp time.Time
	Value     int64
}

func mapTopEvents(items []model.EventTypeCount) []map[string]interface{} {
	points := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		points = append(points, map[string]interface{}{
			"event": item.EventType,
			"count": item.Total,
		})
	}
	return points
}

func decodeEventsPayload(raw interface{}) ([]map[string]interface{}, error) {
	if raw == nil {
		return []map[string]interface{}{}, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func buildBehaviorEvent(entry map[string]interface{}, gameID, env string, fallback time.Time) (*model.BehaviorEvent, error) {
	if entry == nil {
		return nil, errors.New("empty event entry")
	}
	eventType := pickString(entry, "eventType", "event", "type")
	if eventType == "" {
		return nil, errors.New("eventType 不能为空")
	}
	userID := pickString(entry, "userId", "user_id", "playerId", "player_id")
	if userID == "" {
		return nil, errors.New("userId 不能为空")
	}
	tsRaw := pickString(entry, "timestamp", "occurredAt", "time")
	timestamp, err := utils.ParseDate(tsRaw)
	if err != nil || timestamp.IsZero() {
		timestamp = fallback
	}
	payload := make(map[string]interface{}, len(entry))
	for k, v := range entry {
		switch strings.ToLower(k) {
		case "eventtype", "event", "type", "userid", "user_id", "playerid", "player_id", "timestamp", "occurredat", "time":
			continue
		default:
			payload[k] = v
		}
	}

	return &model.BehaviorEvent{
		GameID:     gameID,
		Env:        env,
		EventType:  eventType,
		UserID:     userID,
		Data:       payload,
		OccurredAt: timestamp,
	}, nil
}

func pickString(entry map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := entry[key]; ok {
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

func formatAny(v interface{}) string {
	switch value := v.(type) {
	case string:
		return value
	default:
		return fmt.Sprintf("%v", value)
	}
}
