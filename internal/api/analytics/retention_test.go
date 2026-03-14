package analytics

import (
	"encoding/json"
	"testing"
	"time"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/model"
)

func TestRetentionRangeHelpers(t *testing.T) {
	t.Parallel()

	start, end, err := resolveRetentionRange("2026-03-15", "2026-03-14")
	if err != nil {
		t.Fatalf("resolveRetentionRange() error = %v", err)
	}
	if end.Before(start) {
		t.Fatalf("expected normalized range, got start=%v end=%v", start, end)
	}

	levelStart, levelEnd, err := resolveLevelRange("", "")
	if err != nil {
		t.Fatalf("resolveLevelRange() error = %v", err)
	}
	if levelEnd.Before(levelStart) {
		t.Fatalf("expected valid level range, got start=%v end=%v", levelStart, levelEnd)
	}
}

func TestParseTimeAndRetentionValues(t *testing.T) {
	t.Parallel()

	if _, err := parseTime("2026-03-14"); err != nil {
		t.Fatalf("parseTime(date) error = %v", err)
	}
	if _, err := parseTime(time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)); err != nil {
		t.Fatalf("parseTime(rfc3339) error = %v", err)
	}
	if _, err := parseTime("bad-time"); err == nil {
		t.Fatal("expected parse error for invalid time")
	}

	values := parseRetentionValues(datatypes.JSON([]byte(`[1,0.5,0.25]`)))
	if len(values) != 3 || values[1] != 0.5 {
		t.Fatalf("unexpected retention values: %#v", values)
	}
	if got := parseRetentionValues(datatypes.JSON([]byte(`bad`))); len(got) != 0 {
		t.Fatalf("expected empty values on invalid json, got %#v", got)
	}
}

func TestBehaviorEventValueHelpers(t *testing.T) {
	t.Parallel()

	payload, _ := json.Marshal(map[string]interface{}{
		"levelId":   "l1",
		"progress":  "1.5",
		"completed": "true",
	})
	ev := model.BehaviorEvent{
		UserID: "u1",
		Data:   datatypes.JSONMap{},
	}
	_ = json.Unmarshal(payload, &ev.Data)

	if got := eventString(ev, "levelId"); got != "l1" {
		t.Fatalf("unexpected eventString: %q", got)
	}
	if got := eventFloat(ev, "progress"); got != 1.5 {
		t.Fatalf("unexpected eventFloat: %v", got)
	}
	if !eventBool(ev, "completed") {
		t.Fatal("expected eventBool true")
	}
	if _, ok := toFloat("2.5"); !ok {
		t.Fatal("expected toFloat ok for string")
	}
}

func TestSortHelpersAndMapPoints(t *testing.T) {
	t.Parallel()

	levels := []LevelMetrics{
		{LevelId: "b", Attempts: 1},
		{LevelId: "a", Attempts: 3},
	}
	sortLevelMetrics(levels)
	if levels[0].LevelId != "a" {
		t.Fatalf("expected highest attempts first, got %#v", levels)
	}

	episodes := []EpisodeMetrics{
		{EpisodeId: "b", Players: 1},
		{EpisodeId: "a", Players: 2},
	}
	sortEpisodeMetrics(episodes)
	if episodes[0].EpisodeId != "a" {
		t.Fatalf("expected highest players first, got %#v", episodes)
	}

	maps := []MapMetrics{
		{MapId: "a", HeatMap: []map[string]float64{{"x": 1}}},
		{MapId: "b", HeatMap: []map[string]float64{{"x": 1}, {"x": 2}}},
	}
	sortMapMetrics(maps)
	if maps[0].MapId != "b" {
		t.Fatalf("expected most heat points first, got %#v", maps)
	}

	if got := mapPoints([]map[string]float64{{"x": 1}}); got != 1 {
		t.Fatalf("unexpected mapPoints result: %d", got)
	}
	if got := maxInt(1, 2); got != 2 {
		t.Fatalf("unexpected maxInt result: %d", got)
	}
}
