package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

const (
	defaultOverviewWindowDays = 7
	defaultRealtimeWindow     = 15 * time.Minute
	maxRealtimeDurationMin    = 24 * 60
	officialAnalyticsID       = "official.analytics"
	analyticsFiltersKey       = "filters"
	legacyAnalyticsFiltersKey = "analytics_filters"
)

func overview(ctx context.Context, svcCtx *svc.ServiceContext, req *OverviewRequest) (*OverviewResponse, error) {
	if svcCtx.BehaviorModel == nil || svcCtx.PaymentsModel == nil || svcCtx.PlayerModel == nil {
		return nil, errors.New("analytics models unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolveRange(req.StartDate, req.EndDate, defaultOverviewWindowDays)
	if err != nil {
		return nil, err
	}
	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)

	dauStart := end.Add(-24 * time.Hour)
	mauStart := end.Add(-30 * 24 * time.Hour)

	var (
		dau, mau, activeUsers, newUsers int64
	)

	if dau, err = svcCtx.BehaviorModel.CountDistinctUsers(ctx, gameID, env, dauStart, end); err != nil {
		return nil, err
	}
	if mau, err = svcCtx.BehaviorModel.CountDistinctUsers(ctx, gameID, env, mauStart, end); err != nil {
		return nil, err
	}
	if activeUsers, err = svcCtx.BehaviorModel.CountDistinctUsers(ctx, gameID, env, start, end); err != nil {
		return nil, err
	}
	if newUsers, err = svcCtx.PlayerModel.CountNewPlayers(ctx, gameID, start, end); err != nil {
		return nil, err
	}

	revenueAgg, err := svcCtx.PaymentsModel.AggregateRevenue(ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}

	arpu := safeDivide(revenueAgg.Revenue, float64(activeUsers))
	arppu := safeDivide(revenueAgg.Revenue, float64(revenueAgg.Payers))
	payingRate := safeDivide(float64(revenueAgg.Payers), float64(activeUsers))

	activityStats, err := svcCtx.BehaviorModel.DailyActivity(ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}
	revenueStats, err := svcCtx.PaymentsModel.DailyRevenue(ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}
	newPlayerStats, err := svcCtx.PlayerModel.DailyNewPlayers(ctx, gameID, start, end)
	if err != nil {
		return nil, err
	}

	trends := map[string]interface{}{
		"activeUsers": summarizeBehaviorStats(activityStats, func(stat model.BehaviorDailyStat) int64 { return stat.ActiveUsers }),
		"events":      summarizeBehaviorStats(activityStats, func(stat model.BehaviorDailyStat) int64 { return stat.Events }),
		"newUsers":    summarizePlayerStats(newPlayerStats),
		"revenue":     summarizeRevenueStats(revenueStats),
	}

	return &OverviewResponse{
		Metrics: OverviewMetrics{
			DAU:        int(dau),
			MAU:        int(mau),
			NewUsers:   int(newUsers),
			Revenue:    revenueAgg.Revenue,
			ARPU:       arpu,
			ARPPU:      arppu,
			PayingRate: payingRate,
		},
		Trends: trends,
	}, nil
}

func realtime(ctx context.Context, svcCtx *svc.ServiceContext, req *RealtimeRequest) (*RealtimeResponse, error) {
	if svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}

	gameID := ""
	env := ""
	if req != nil {
		gameID = strings.TrimSpace(req.GameId)
		env = strings.TrimSpace(req.Env)
	}

	end := time.Now().UTC()
	start := end.Add(-defaultRealtimeWindow)

	activeUsers, err := svcCtx.BehaviorModel.CountDistinctUsers(ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}
	eventsCount, err := svcCtx.BehaviorModel.CountEvents(ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}
	eventTypeCounts, err := svcCtx.BehaviorModel.EventTypeCounts(ctx, gameID, env, start, end, 5)
	if err != nil {
		return nil, err
	}

	qps := safeDivide(float64(eventsCount), defaultRealtimeWindow.Seconds())
	avgLatency, errorRate := aggregateAgentMetrics(svcCtx.RegistryStore, gameID, env)

	return &RealtimeResponse{
		RealtimeMetrics: RealtimeMetrics{
			OnlineUsers:    int(activeUsers),
			ActiveSessions: int(eventsCount),
			QPS:            qps,
			AvgLatency:     avgLatency,
			ErrorRate:      errorRate,
			TopEvents:      mapTopEvents(eventTypeCounts),
		},
		Timestamp: helper.FormatTimestamp(end),
	}, nil
}

func realtimeSeries(ctx context.Context, svcCtx *svc.ServiceContext, req *RealtimeSeriesRequest) (*RealtimeSeriesResponse, error) {
	if svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}

	gameID := ""
	env := ""
	if req != nil {
		gameID = strings.TrimSpace(req.GameId)
		env = strings.TrimSpace(req.Env)
	}

	interval := resolveRealtimeInterval("")
	window := clampRealtimeDuration(60)
	if req != nil {
		interval = resolveRealtimeInterval(req.Interval)
		window = clampRealtimeDuration(req.Duration)
	}

	end := time.Now().UTC()
	start := end.Add(-window)

	var eventsSeries, usersSeries []bucketPoint

	for cursor := start; cursor.Before(end); cursor = cursor.Add(interval) {
		bucketEnd := cursor.Add(interval)
		if bucketEnd.After(end) {
			bucketEnd = end
		}

		eventsCount, err := svcCtx.BehaviorModel.CountEvents(ctx, gameID, env, cursor, bucketEnd)
		if err != nil {
			return nil, err
		}
		usersCount, err := svcCtx.BehaviorModel.CountDistinctUsers(ctx, gameID, env, cursor, bucketEnd)
		if err != nil {
			return nil, err
		}

		eventsSeries = append(eventsSeries, bucketPoint{
			Timestamp: bucketEnd,
			Value:     eventsCount,
		})
		usersSeries = append(usersSeries, bucketPoint{
			Timestamp: bucketEnd,
			Value:     usersCount,
		})
	}

	series := map[string]interface{}{
		"events": buildRealtimeSeriesPoints(eventsSeries),
		"users":  buildRealtimeSeriesPoints(usersSeries),
	}

	return &RealtimeSeriesResponse{
		Series: series,
	}, nil
}

func ingest(ctx context.Context, svcCtx *svc.ServiceContext, req *IngestRequest) (*IngestResponse, error) {
	if svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errors.New("gameId 不能为空")
	}
	env := strings.TrimSpace(req.Env)

	rawEvents, err := decodeEventsPayload(req.Events)
	if err != nil {
		return nil, err
	}

	var accepted, rejected int
	for _, entry := range rawEvents {
		event, buildErr := buildBehaviorEvent(entry, gameID, env, time.Now().UTC())
		if buildErr != nil {
			rejected++
			continue
		}
		if err := svcCtx.BehaviorModel.RecordEvent(ctx, event); err != nil {
			rejected++
			continue
		}
		accepted++
	}

	return &IngestResponse{
		Accepted: accepted,
		Rejected: rejected,
		BatchId:  uuid.New().String(),
	}, nil
}

func filtersGet(ctx context.Context, svcCtx *svc.ServiceContext, req *FiltersGetRequest) (*FiltersGetResponse, error) {
	gameID := ""
	if req != nil {
		gameID = strings.TrimSpace(req.GameId)
	}

	if items, ok, err := loadAnalyticsFiltersFromExtensionInstallation(ctx, svcCtx); err != nil {
		return nil, err
	} else if ok {
		return &FiltersGetResponse{
			Items: filterAnalyticsFilters(items, gameID),
		}, nil
	}

	path := helper.ResolveAnalyticsFiltersPath(svcCtx.Config)

	var items []AnalyticsFilters
	var err error

	if lock := svcCtx.AnalyticsFiltersLock; lock != nil {
		lock.RLock()
		defer lock.RUnlock()
		var data []byte
		data, err = helper.ReadAnalyticsFiltersFile(path)
		if err != nil {
			return nil, err
		}
		items, err = LoadAnalyticsFilters(data)
	} else {
		var data []byte
		data, err = helper.ReadAnalyticsFiltersFile(path)
		if err != nil {
			return nil, err
		}
		items, err = LoadAnalyticsFilters(data)
	}
	if err != nil {
		return nil, err
	}

	filtered := filterAnalyticsFilters(items, gameID)

	return &FiltersGetResponse{
		Items: filtered,
	}, nil
}

func filtersUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *FiltersUpdateRequest) (*FiltersGetResponse, error) {
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errors.New("gameId 不能为空")
	}
	if req.Filters == nil {
		return nil, errors.New("filters 不能为空")
	}

	items, source, err := loadAnalyticsFiltersForUpdate(ctx, svcCtx)
	if err != nil {
		return nil, err
	}

	items = upsertAnalyticsFilter(items, gameID, req.Filters)

	if source == "extension" {
		if err := saveAnalyticsFiltersToExtensionInstallation(ctx, svcCtx, items); err != nil {
			return nil, err
		}
	} else {
		path := helper.ResolveAnalyticsFiltersPath(svcCtx.Config)
		jsonData, err := SaveAnalyticsFiltersJSON(items)
		if err != nil {
			return nil, err
		}
		if err := helper.WriteAnalyticsFiltersFile(path, jsonData); err != nil {
			return nil, err
		}
	}

	return &FiltersGetResponse{
		Items: filterAnalyticsFilters(items, gameID),
	}, nil
}

func loadAnalyticsFiltersForUpdate(ctx context.Context, svcCtx *svc.ServiceContext) ([]AnalyticsFilters, string, error) {
	if items, ok, err := loadAnalyticsFiltersFromExtensionInstallation(ctx, svcCtx); err != nil {
		return nil, "", err
	} else if ok {
		return items, "extension", nil
	}
	path := helper.ResolveAnalyticsFiltersPath(svcCtx.Config)
	var items []AnalyticsFilters
	var err error
	if lock := svcCtx.AnalyticsFiltersLock; lock != nil {
		lock.Lock()
		defer lock.Unlock()
		var data []byte
		data, err = helper.ReadAnalyticsFiltersFile(path)
		if err != nil {
			return nil, "", err
		}
		items, err = LoadAnalyticsFilters(data)
	} else {
		var data []byte
		data, err = helper.ReadAnalyticsFiltersFile(path)
		if err != nil {
			return nil, "", err
		}
		items, err = LoadAnalyticsFilters(data)
	}
	if err != nil {
		return nil, "", err
	}
	return items, "file", nil
}

func loadAnalyticsFiltersFromExtensionInstallation(ctx context.Context, svcCtx *svc.ServiceContext) ([]AnalyticsFilters, bool, error) {
	item, ok, err := findActiveAnalyticsInstallation(ctx, svcCtx)
	if err != nil || !ok || item == nil {
		return nil, false, err
	}
	config := map[string]any{}
	if len(bytes.TrimSpace(item.ConfigJSON)) > 0 {
		if err := json.Unmarshal(item.ConfigJSON, &config); err != nil {
			return nil, false, err
		}
	}
	items, ok, err := extractAnalyticsFiltersFromConfig(config)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return []AnalyticsFilters{}, true, nil
	}
	return items, true, nil
}

func saveAnalyticsFiltersToExtensionInstallation(ctx context.Context, svcCtx *svc.ServiceContext, items []AnalyticsFilters) error {
	item, ok, err := findActiveAnalyticsInstallation(ctx, svcCtx)
	if err != nil || !ok || item == nil {
		return err
	}
	config := map[string]any{}
	if len(bytes.TrimSpace(item.ConfigJSON)) > 0 {
		_ = json.Unmarshal(item.ConfigJSON, &config)
	}
	config = setAnalyticsFiltersToConfig(config, items)
	secretRefs := map[string]string{}
	if len(bytes.TrimSpace(item.SecretRefsJSON)) > 0 {
		_ = json.Unmarshal(item.SecretRefsJSON, &secretRefs)
	}
	operator := "system"
	if v := strings.TrimSpace(pickContextString(ctx, "username")); v != "" {
		operator = v
	}
	return svcCtx.Extensions.Installation.UpdateConfig(ctx, item.ID, config, secretRefs, operator)
}

func findActiveAnalyticsInstallation(ctx context.Context, svcCtx *svc.ServiceContext) (*model.ExtensionInstallation, bool, error) {
	if svcCtx == nil || svcCtx.Extensions == nil || svcCtx.Extensions.Installation == nil {
		return nil, false, nil
	}
	items, _, err := svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
		ExtensionID: officialAnalyticsID,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		return nil, false, err
	}
	for i := range items {
		item := items[i]
		if strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") ||
			strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled") {
			continue
		}
		return &item, true, nil
	}
	return nil, false, nil
}

func extractAnalyticsFiltersFromConfig(config map[string]any) ([]AnalyticsFilters, bool, error) {
	if config == nil {
		return nil, false, nil
	}
	raw, exists := config[analyticsFiltersKey]
	if !exists || raw == nil {
		raw, exists = config[legacyAnalyticsFiltersKey]
	}
	if !exists || raw == nil {
		return nil, false, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, false, err
	}
	items := []AnalyticsFilters{}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, false, err
	}
	return normalizeAnalyticsFilters(items), true, nil
}

func setAnalyticsFiltersToConfig(config map[string]any, items []AnalyticsFilters) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	normalized := normalizeAnalyticsFilters(items)
	config[analyticsFiltersKey] = normalized
	config[legacyAnalyticsFiltersKey] = normalized
	return config
}

func pickContextString(ctx context.Context, key string) string {
	if ctx == nil || strings.TrimSpace(key) == "" {
		return ""
	}
	if v := ctx.Value(key); v != nil {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

// Helper types and functions for overview analytics

type bucketPoint struct {
	Timestamp time.Time
	Value     int64
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
			"timestamp": helper.FormatTimestamp(sample.Timestamp),
			"value":     sample.Value,
		})
	}
	return points
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
	timestamp, err := helper.ParseDate(tsRaw)
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

func filterAnalyticsFilters(items []AnalyticsFilters, gameID string) []AnalyticsFilters {
	if gameID == "" {
		return items
	}
	filtered := make([]AnalyticsFilters, 0, 1)
	for _, item := range items {
		if strings.EqualFold(item.GameId, gameID) {
			filtered = append(filtered, item)
			break
		}
	}
	return filtered
}

func upsertAnalyticsFilter(items []AnalyticsFilters, gameID string, filters interface{}) []AnalyticsFilters {
	replaced := false
	for i := range items {
		if strings.EqualFold(items[i].GameId, gameID) {
			items[i].Filters = filters
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, AnalyticsFilters{
			GameId:  gameID,
			Filters: filters,
		})
	}
	return items
}
