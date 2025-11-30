package analytics_behavior

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
)

type segmentSummary struct {
	TotalUsers int
	Regions    map[string]int
	Platforms  map[string]int
	Roles      map[string]int
	Actions    map[string]int
}

func breakdownBySegment(events []model.BehaviorEvent) segmentSummary {
	uniqueUsers := make(map[string]struct{})
	regions := make(map[string]int)
	platforms := make(map[string]int)
	roles := make(map[string]int)
	actions := make(map[string]int)

	for _, event := range events {
		user := strings.TrimSpace(event.UserID)
		if user == "" {
			continue
		}
		if _, exists := uniqueUsers[user]; !exists {
			uniqueUsers[user] = struct{}{}
		}
		meta := map[string]interface{}(event.Data)
		if region, ok := metaValue(meta, "region"); ok {
			regions[region]++
		}
		if platform, ok := metaValue(meta, "platform"); ok {
			platforms[platform]++
		}
		if role, ok := metaValue(meta, "role"); ok {
			roles[role]++
		}
		if step := strings.TrimSpace(event.EventType); step != "" {
			actions[step]++
		}
	}

	return segmentSummary{
		TotalUsers: len(uniqueUsers),
		Regions:    regions,
		Platforms:  platforms,
		Roles:      roles,
		Actions:    actions,
	}
}

func metaValue(meta map[string]interface{}, key string) (string, bool) {
	if meta == nil {
		return "", false
	}
	if val, ok := meta[key]; ok {
		v := strings.TrimSpace(stringify(val))
		if v != "" {
			return v, true
		}
	}
	return "", false
}

func stringify(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

type timeSeriesPoint struct {
	Timestamp string `json:"timestamp"`
	Count     int    `json:"count"`
}

func breakdownByTime(events []model.BehaviorEvent, start, end time.Time) []timeSeriesPoint {
	if len(events) == 0 {
		return []timeSeriesPoint{}
	}
	if start.IsZero() {
		start = events[len(events)-1].OccurredAt
	}
	if end.IsZero() {
		end = events[0].OccurredAt
	}
	if end.Before(start) {
		start, end = end, start
	}

	interval := 24 * time.Hour
	totalDuration := end.Sub(start)
	if totalDuration <= 7*24*time.Hour {
		interval = 6 * time.Hour
	} else if totalDuration >= 60*24*time.Hour {
		interval = 24 * time.Hour
	}

	points := make([]timeSeriesPoint, 0, int(totalDuration/interval)+1)
	buckets := make(map[int]int)

	for _, ev := range events {
		if ev.OccurredAt.Before(start) || ev.OccurredAt.After(end) {
			continue
		}
		offset := int(ev.OccurredAt.Sub(start) / interval)
		buckets[offset]++
	}

	for ts := start; ts.Before(end) || ts.Equal(end); ts = ts.Add(interval) {
		offset := int(ts.Sub(start) / interval)
		points = append(points, timeSeriesPoint{
			Timestamp: utils.FormatTimestamp(ts),
			Count:     buckets[offset],
		})
	}
	return points
}

func topMapPairs(m map[string]int, limit int) []map[string]interface{} {
	type pair struct {
		Key   string
		Value int
	}
	list := make([]pair, 0, len(m))
	for k, v := range m {
		if strings.TrimSpace(k) == "" || v <= 0 {
			continue
		}
		list = append(list, pair{Key: k, Value: v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Value == list[j].Value {
			return list[i].Key < list[j].Key
		}
		return list[i].Value > list[j].Value
	})
	if len(list) > limit {
		list = list[:limit]
	}
	results := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		results = append(results, map[string]interface{}{
			"label": item.Key,
			"value": item.Value,
		})
	}
	return results
}
