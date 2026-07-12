package analytics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

func behaviorAnalytics(ctx context.Context, svcCtx *svc.ServiceContext, req *BehaviorRequest) (*BehaviorResponse, error) {
	if svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}

	start, end, err := helper.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.BehaviorEventOptions{
		PaginationOptions: model.NewPagination(1, 3000),
		GameID:    strings.TrimSpace(req.GameId),
		Env:       strings.TrimSpace(req.Env),
		StartTime: start,
		EndTime:   end,
	}

	events, _, err := svcCtx.BehaviorModel.ListEvents(ctx, opts)
	if err != nil {
		return nil, err
	}

	segments := breakdownBySegment(events)
	heatmap := breakdownByTime(events, start, end)

	return &BehaviorResponse{
		TopActions: topMapPairs(segments.Actions, 20),
		UserFlows: map[string]interface{}{
			"regions":   topMapPairs(segments.Regions, 10),
			"platforms": topMapPairs(segments.Platforms, 10),
		},
		HeatMap: map[string]interface{}{
			"points": heatmap,
		},
	}, nil
}

func behaviorEvents(ctx context.Context, svcCtx *svc.ServiceContext, req *BehaviorEventsRequest) (*BehaviorEventsResponse, error) {
	if svcCtx.BehaviorModel == nil {
		return nil, errors.New("analytics model unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	start, end, err := helper.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.BehaviorEventOptions{
		PaginationOptions: model.NewPagination(1, limit),
		GameID:    strings.TrimSpace(req.GameId),
		Env:       strings.TrimSpace(req.Env),
		EventType: strings.TrimSpace(req.EventType),
		StartTime: start,
		EndTime:   end,
	}

	events, total, err := svcCtx.BehaviorModel.ListEvents(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]BehaviorEvent, 0, len(events))
	for i := range events {
		ev := events[i]
		var payload interface{} = map[string]interface{}{}
		if ev.Data != nil {
			payload = map[string]interface{}(ev.Data)
		}
		items = append(items, BehaviorEvent{
			EventType: ev.EventType,
			UserId:    ev.UserID,
			Data:      payload,
			Timestamp: helper.FormatTimestamp(ev.OccurredAt),
		})
	}

	return &BehaviorEventsResponse{
		Items: items,
		Total: int(total),
	}, nil
}

func behaviorAdoption(ctx context.Context, svcCtx *svc.ServiceContext, req *BehaviorAdoptionRequest) (*BehaviorAdoptionResponse, error) {
	if svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	items, err := svcCtx.BehaviorModel.ListFeatureAdoptions(ctx, gameID, env)
	if err != nil {
		return nil, err
	}

	features := make([]FeatureAdoption, 0, len(items))
	for _, item := range items {
		if req.Feature != "" && !strings.EqualFold(item.Feature, req.Feature) {
			continue
		}
		features = append(features, FeatureAdoption{
			Feature:      item.Feature,
			Users:        item.Users,
			AdoptionRate: item.AdoptionRate,
			Frequency:    item.Frequency,
		})
	}

	return &BehaviorAdoptionResponse{
		Features: features,
	}, nil
}

func behaviorAdoptionBreakdown(ctx context.Context, svcCtx *svc.ServiceContext, req *BehaviorAdoptionBreakdownRequest) (*BehaviorAdoptionBreakdownResponse, error) {
	if svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}
	if strings.TrimSpace(req.Feature) == "" {
		return nil, errors.New("feature 参数不能为空")
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	feature := strings.TrimSpace(req.Feature)

	start, end, err := helper.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.BehaviorEventOptions{
		PaginationOptions: model.NewPagination(1, 5000),
		GameID:    gameID,
		Env:       env,
		EventType: feature,
		StartTime: start,
		EndTime:   end,
	}

	events, _, err := svcCtx.BehaviorModel.ListEvents(ctx, opts)
	if err != nil {
		return nil, err
	}

	segment := breakdownBySegment(events)
	series := breakdownByTime(events, start, end)

	return &BehaviorAdoptionBreakdownResponse{
		BySegment: map[string]interface{}{
			"totalUsers": segment.TotalUsers,
			"regions":    segment.Regions,
			"platforms":  segment.Platforms,
			"roles":      segment.Roles,
		},
		ByTime: map[string]interface{}{
			"intervals": series,
			"range": map[string]string{
				"start": helper.FormatTimestamp(start),
				"end":   helper.FormatTimestamp(end),
			},
		},
	}, nil
}

func behaviorFunnel(ctx context.Context, svcCtx *svc.ServiceContext, req *BehaviorFunnelRequest) (*BehaviorFunnelResponse, error) {
	if svcCtx.BehaviorModel == nil {
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

	start, end, err := helper.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.BehaviorEventOptions{
		PaginationOptions: model.NewPagination(1, 5000),
		GameID:    strings.TrimSpace(req.GameId),
		Env:       strings.TrimSpace(req.Env),
		StartTime: start,
		EndTime:   end,
	}

	events, _, err := svcCtx.BehaviorModel.ListEvents(ctx, opts)
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

	responseSteps := make([]FunnelStep, len(steps))
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
		responseSteps[i] = FunnelStep{
			Step:           step,
			Users:          count,
			ConversionRate: roundPercentage(conversion),
			DropOffRate:    roundPercentage(dropOff),
		}
	}

	return &BehaviorFunnelResponse{
		Steps: responseSteps,
	}, nil
}

func behaviorPaths(ctx context.Context, svcCtx *svc.ServiceContext, req *BehaviorPathsRequest) (*BehaviorPathsResponse, error) {
	if svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}

	depth := req.Depth
	if depth <= 0 {
		depth = 5
	}
	if depth > 10 {
		depth = 10
	}

	start, end, err := helper.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.BehaviorEventOptions{
		PaginationOptions: model.NewPagination(1, 5000),
		GameID:    strings.TrimSpace(req.GameId),
		Env:       strings.TrimSpace(req.Env),
		StartTime: start,
		EndTime:   end,
	}

	events, _, err := svcCtx.BehaviorModel.ListEvents(ctx, opts)
	if err != nil {
		return nil, err
	}

	paths := buildPaths(events, depth)

	return &BehaviorPathsResponse{
		Paths: map[string]interface{}{
			"depth": depth,
			"items": paths,
			"total": len(paths),
			"range": map[string]interface{}{
				"start": helper.FormatTimestamp(start),
				"end":   helper.FormatTimestamp(end),
			},
		},
	}, nil
}

// Helper types and functions for behavior analytics

type segmentSummary struct {
	TotalUsers int
	Regions    map[string]int
	Platforms  map[string]int
	Roles      map[string]int
	Actions    map[string]int
}

type pathNode struct {
	Count    int
	Children map[string]*pathNode
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
			Timestamp: helper.FormatTimestamp(ts),
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
	return math.Round(value*10000) / 100
}

func buildPaths(events []model.BehaviorEvent, depth int) []map[string]interface{} {
	byUser := groupEventsByUserForPaths(events)
	root := &pathNode{Children: map[string]*pathNode{}}

	for _, list := range byUser {
		if len(list) == 0 {
			continue
		}
		sequence := make([]string, 0, len(list))
		for _, ev := range list {
			name := strings.TrimSpace(ev.EventType)
			if name == "" {
				continue
			}
			sequence = append(sequence, name)
		}
		if len(sequence) == 0 {
			continue
		}
		addSequence(root, sequence, depth)
	}

	results := make([]map[string]interface{}, 0, 64)
	walkPaths(root, []string{}, &results, depth)

	sort.Slice(results, func(i, j int) bool {
		if results[i]["count"].(int) == results[j]["count"].(int) {
			return strings.Join(results[i]["path"].([]string), ">") < strings.Join(results[j]["path"].([]string), ">")
		}
		return results[i]["count"].(int) > results[j]["count"].(int)
	})

	if len(results) > 50 {
		results = results[:50]
	}
	return results
}

func groupEventsByUserForPaths(events []model.BehaviorEvent) map[string][]model.BehaviorEvent {
	grouped := make(map[string][]model.BehaviorEvent)
	for _, ev := range events {
		user := strings.TrimSpace(ev.UserID)
		if user == "" {
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

func addSequence(root *pathNode, sequence []string, depth int) {
	node := root
	for i := 0; i < len(sequence) && i < depth; i++ {
		step := sequence[i]
		if node.Children == nil {
			node.Children = make(map[string]*pathNode)
		}
		child, ok := node.Children[step]
		if !ok {
			child = &pathNode{Children: map[string]*pathNode{}}
			node.Children[step] = child
		}
		child.Count++
		node = child
	}
}

func walkPaths(node *pathNode, prefix []string, acc *[]map[string]interface{}, depth int) {
	for label, child := range node.Children {
		path := append(prefix, label)
		entry := map[string]interface{}{
			"path":  append([]string{}, path...),
			"count": child.Count,
		}
		*acc = append(*acc, entry)
		if len(path) < depth {
			walkPaths(child, path, acc, depth)
		}
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
	default:
		return fmt.Sprintf("%v", v)
	}
}
