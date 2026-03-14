package analytics

import (
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

func TestSummarizeStatsHelpers(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	behavior := summarizeBehaviorStats([]model.BehaviorDailyStat{
		{Day: day, ActiveUsers: 10, Events: 20},
	}, func(stat model.BehaviorDailyStat) int64 { return stat.ActiveUsers })
	if len(behavior) != 1 || behavior[0]["value"] != int64(10) {
		t.Fatalf("unexpected behavior stats: %#v", behavior)
	}

	revenue := summarizeRevenueStats([]model.DailyRevenueStat{
		{Day: day, Revenue: 100, Transactions: 4},
	})
	if len(revenue) != 1 || revenue[0]["avgTicket"] != 25.0 {
		t.Fatalf("unexpected revenue stats: %#v", revenue)
	}

	players := summarizePlayerStats([]model.DailyNewPlayerStat{
		{Day: day, Count: 8},
	})
	if len(players) != 1 || players[0]["value"] != int64(8) {
		t.Fatalf("unexpected player stats: %#v", players)
	}
}

func TestRealtimeWindowHelpers(t *testing.T) {
	t.Parallel()

	if got := resolveRealtimeInterval(""); got != time.Minute {
		t.Fatalf("expected 1m, got %v", got)
	}
	if got := resolveRealtimeInterval("5m"); got != 5*time.Minute {
		t.Fatalf("expected 5m, got %v", got)
	}
	if got := clampRealtimeDuration(-1); got != 60*time.Minute {
		t.Fatalf("expected default 60m, got %v", got)
	}
	if got := clampRealtimeDuration(maxRealtimeDurationMin + 10); got != time.Duration(maxRealtimeDurationMin)*time.Minute {
		t.Fatalf("expected clamped duration, got %v", got)
	}
}

func TestBuildRealtimeSeriesPointsAndTopEvents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	points := buildRealtimeSeriesPoints([]bucketPoint{{Timestamp: now, Value: 12}})
	if len(points) != 1 || points[0]["value"] != int64(12) {
		t.Fatalf("unexpected series points: %#v", points)
	}

	events := mapTopEvents([]model.EventTypeCount{{EventType: "login", Total: 3}})
	if len(events) != 1 || events[0]["event"] != "login" {
		t.Fatalf("unexpected top events: %#v", events)
	}
}

func TestDecodeEventsPayloadAndBehaviorEventBuilders(t *testing.T) {
	t.Parallel()

	list, err := decodeEventsPayload([]map[string]interface{}{
		{"eventType": "login", "userId": "u1"},
	})
	if err != nil {
		t.Fatalf("decodeEventsPayload() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 event, got %d", len(list))
	}

	fallback := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	event, err := buildBehaviorEvent(map[string]interface{}{
		"eventType": "login",
		"userId":    "u1",
		"source":    "web",
	}, "tower", "prod", fallback)
	if err != nil {
		t.Fatalf("buildBehaviorEvent() error = %v", err)
	}
	if event.EventType != "login" || event.UserID != "u1" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event.Data["source"] != "web" {
		t.Fatalf("expected payload source, got %#v", event.Data)
	}

	if _, err := buildBehaviorEvent(map[string]interface{}{"userId": "u1"}, "tower", "prod", fallback); err == nil {
		t.Fatal("expected missing eventType error")
	}
	if _, err := buildBehaviorEvent(map[string]interface{}{"eventType": "login"}, "tower", "prod", fallback); err == nil {
		t.Fatal("expected missing userId error")
	}
}

func TestPickStringFormatAndFilterHelpers(t *testing.T) {
	t.Parallel()

	entry := map[string]interface{}{"gameId": 123, "name": " tower "}
	if got := pickString(entry, "name"); got != "tower" {
		t.Fatalf("unexpected pickString for string: %q", got)
	}
	if got := pickString(entry, "gameId"); got != "123" {
		t.Fatalf("unexpected pickString for number: %q", got)
	}
	if got := formatAny(12.5); got != "12.5" {
		t.Fatalf("unexpected formatAny result: %q", got)
	}

	items := []AnalyticsFilters{
		{GameId: "tower", Filters: map[string]interface{}{"env": "prod"}},
		{GameId: "other", Filters: map[string]interface{}{"env": "dev"}},
	}
	filtered := filterAnalyticsFilters(items, "tower")
	if len(filtered) != 1 || filtered[0].GameId != "tower" {
		t.Fatalf("unexpected filtered result: %#v", filtered)
	}

	updated := upsertAnalyticsFilter(items, "tower", map[string]interface{}{"env": "stage"})
	if len(updated) != 2 {
		t.Fatalf("expected replace in place, got %#v", updated)
	}
	added := upsertAnalyticsFilter(nil, "new", map[string]interface{}{"env": "prod"})
	if len(added) != 1 || added[0].GameId != "new" {
		t.Fatalf("unexpected upsert add result: %#v", added)
	}
}
