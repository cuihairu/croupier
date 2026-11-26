package analytics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DATA-024: behavior aggregations must cover the FULL query window.
//
// The old implementation passed NewPagination(1, 3000/5000) to ListEvents,
// which Normalize() clamped to a single page of 100 rows and discarded the
// total: every aggregate silently described only the newest 100 events.
// Aggregations now page through the whole window and disclose the covered
// scale in the response `sample` field.
// ---------------------------------------------------------------------------

// seedBehaviorWindow inserts n events with distinct users/action names. Times
// run backwards from base so the newest events are the last inserted.
func seedBehaviorWindow(t *testing.T, svcCtxModel *model.BehaviorModel, gameID string, events []model.BehaviorEvent) {
	t.Helper()
	for i := range events {
		ev := events[i]
		require.NoError(t, svcCtxModel.RecordEvent(context.Background(), &ev))
	}
}

func behaviorWindowEvents(n int, eventType, gameID string, base time.Time, data map[string]interface{}) []model.BehaviorEvent {
	events := make([]model.BehaviorEvent, 0, n)
	for i := 0; i < n; i++ {
		events = append(events, createTestEvent(
			eventType,
			fmt.Sprintf("user-%d", i),
			gameID,
			"prod",
			base.Add(-time.Duration(i)*time.Minute),
			data,
		))
	}
	return events
}

// Summary and heatmap must aggregate every event in the window, not the newest
// 100: the oldest bucket must still carry its events.
func TestBehaviorAnalyticsCoversFullWindowV24(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	// 250 events spread over 3 days: 120 recent + 130 older than the first
	// page of 100. Old events carry a distinct region so their absence is
	// visible in the breakdown.
	recent := behaviorWindowEvents(120, "login", "game1", now.Add(-10*time.Minute), map[string]interface{}{"region": "US"})
	older := behaviorWindowEvents(130, "login", "game1", now.Add(-60*time.Hour), map[string]interface{}{"region": "EU"})
	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1", append(recent, older...))

	resp, err := behaviorAnalytics(context.Background(), svcCtx, &BehaviorRequest{
		GameId:    "game1",
		Env:       "prod",
		StartDate: now.Add(-96 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	})
	require.NoError(t, err)

	require.NotNil(t, resp.Sample)
	assert.EqualValues(t, 250, resp.Sample["totalEvents"])
	assert.EqualValues(t, 250, resp.Sample["scannedEvents"])
	assert.Equal(t, false, resp.Sample["sampled"])
	assert.Equal(t, "complete", resp.Sample["coverage"])

	// Both regions were aggregated; the old page-one-only read saw only US.
	regions := map[string]int{}
	for _, pair := range resp.UserFlows["regions"].([]map[string]interface{}) {
		regions[pair["label"].(string)] = pair["value"].(int)
	}
	assert.Equal(t, 120, regions["US"], "recent region count")
	assert.Equal(t, 130, regions["EU"], "region beyond the first page must be counted")

	// Heatmap totals must add up to the whole window as well.
	points := resp.HeatMap["points"].([]timeSeriesPoint)
	sum := 0
	for _, p := range points {
		sum += p.Count
	}
	assert.Equal(t, 250, sum, "heatmap must cover every event in the window")
}

// The funnel must cover steps that sit on later pages: a user's first steps are
// older than >100 newer events, so a page-one-only read loses the whole funnel.
func TestBehaviorFunnelCoversStepsAcrossPagesV24(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	events := []model.BehaviorEvent{
		createTestEvent("view", "funnel-user", "game1", "prod", now.Add(-4*time.Hour), nil),
		createTestEvent("click", "funnel-user", "game1", "prod", now.Add(-3*time.Hour), nil),
		createTestEvent("pay", "funnel-user", "game1", "prod", now.Add(-2*time.Hour), nil),
	}
	// 120 filler events newer than every funnel step fill the first page(s).
	events = append(events, behaviorWindowEvents(120, "filler", "game1", now.Add(-30*time.Minute), nil)...)
	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1", events)

	resp, err := behaviorFunnel(context.Background(), svcCtx, &BehaviorFunnelRequest{
		GameId:    "game1",
		Env:       "prod",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		Steps:     []string{"view", "click", "pay"},
	})
	require.NoError(t, err)
	require.Len(t, resp.Steps, 3)
	assert.Equal(t, 1, resp.Steps[0].Users, "view on an older page")
	assert.Equal(t, 1, resp.Steps[1].Users, "click on an older page")
	assert.Equal(t, 1, resp.Steps[2].Users, "pay on an older page")
	assert.EqualValues(t, 123, resp.Sample["totalEvents"])
	assert.Equal(t, false, resp.Sample["sampled"])
}

// Paths must be built from the whole window too.
func TestBehaviorPathsCoversFullWindowV24(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	events := []model.BehaviorEvent{
		createTestEvent("view", "path-user", "game1", "prod", now.Add(-4*time.Hour), nil),
		createTestEvent("click", "path-user", "game1", "prod", now.Add(-3*time.Hour), nil),
		createTestEvent("pay", "path-user", "game1", "prod", now.Add(-2*time.Hour), nil),
	}
	events = append(events, behaviorWindowEvents(120, "filler", "game1", now.Add(-30*time.Minute), nil)...)
	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1", events)

	resp, err := behaviorPaths(context.Background(), svcCtx, &BehaviorPathsRequest{
		GameId:    "game1",
		Env:       "prod",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		Depth:     5,
	})
	require.NoError(t, err)

	items, ok := resp.Paths["items"].([]map[string]interface{})
	require.True(t, ok)
	found := false
	for _, item := range items {
		path, _ := item["path"].([]string)
		if len(path) == 3 && path[0] == "view" && path[1] == "click" && path[2] == "pay" {
			found = true
			assert.Equal(t, 1, item["count"])
		}
	}
	assert.True(t, found, "path built from older pages must appear")
	sample, ok := resp.Paths["sample"].(map[string]interface{})
	require.True(t, ok)
	assert.EqualValues(t, 123, sample["totalEvents"])
}

// Adoption breakdown must count every event of the featured type in the window.
func TestBehaviorAdoptionBreakdownCoversFullWindowV24(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	recent := behaviorWindowEvents(70, "feat", "game1", now.Add(-10*time.Minute), map[string]interface{}{"region": "US"})
	older := behaviorWindowEvents(70, "feat", "game1", now.Add(-60*time.Hour), map[string]interface{}{"region": "EU"})
	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1", append(recent, older...))

	resp, err := behaviorAdoptionBreakdown(context.Background(), svcCtx, &BehaviorAdoptionBreakdownRequest{
		GameId:    "game1",
		Env:       "prod",
		Feature:   "feat",
		StartDate: now.Add(-96 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	})
	require.NoError(t, err)

	regions := resp.BySegment["regions"].(map[string]int)
	assert.Equal(t, 70, regions["US"])
	assert.Equal(t, 70, regions["EU"], "events beyond the first page must be counted")
	assert.EqualValues(t, 140, resp.Sample["totalEvents"])
}

// Levels (and by the same helper episodes/maps) must aggregate attempts that
// spill past the first 100-row page.
func TestLevelsAggregatesBeyondFirstPageV24(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	events := behaviorWindowEvents(120, "level_attempt", "game1", now.Add(-time.Hour), map[string]interface{}{"levelId": "L1"})
	events = append(events, behaviorWindowEvents(120, "level_complete", "game1", now.Add(-time.Hour), map[string]interface{}{"levelId": "L1"})...)
	// The completion events must belong to the same level; distinct users keep
	// them separate rows.
	for i := range events[120:] {
		events[120+i].UserID = fmt.Sprintf("done-%d", i)
	}
	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1", events)

	resp, err := levels(context.Background(), svcCtx, &LevelsRequest{
		GameId:    "game1",
		Env:       "prod",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	})
	require.NoError(t, err)
	require.Len(t, resp.Levels, 1)
	assert.Equal(t, 120, resp.Levels[0].Attempts, "attempts beyond the first page")
	assert.Equal(t, 120, resp.Levels[0].Completions, "completions beyond the first page")
	assert.EqualValues(t, 240, resp.Sample["totalEvents"])
}

// Episodes: players and completions past the first page are counted.
func TestLevelsEpisodesAggregatesBeyondFirstPageV24(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	events := behaviorWindowEvents(120, "episode_progress", "game1", now.Add(-time.Hour), map[string]interface{}{"episodeId": "E1"})
	events = append(events, behaviorWindowEvents(120, "episode_complete", "game1", now.Add(-time.Hour), map[string]interface{}{"episodeId": "E1"})...)
	for i := range events[120:] {
		events[120+i].UserID = fmt.Sprintf("done-%d", i)
	}
	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1", events)

	resp, err := levelsEpisodes(context.Background(), svcCtx, &LevelsEpisodesRequest{
		GameId:    "game1",
		Env:       "prod",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	})
	require.NoError(t, err)
	require.Len(t, resp.Episodes, 1)
	assert.Equal(t, 240, resp.Episodes[0].Players, "players past the first page")
	assert.Equal(t, 0.5, resp.Episodes[0].CompletionRate)
}

// Maps: heat points past the first page are kept (bounded by maxMapPoints).
func TestLevelsMapsAggregatesBeyondFirstPageV24(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	events := behaviorWindowEvents(150, "map_heat", "game1", now.Add(-time.Hour), map[string]interface{}{
		"mapId": "M1", "x": 1.0, "y": 2.0,
	})
	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1", events)

	resp, err := levelsMaps(context.Background(), svcCtx, &LevelsMapsRequest{
		GameId:    "game1",
		Env:       "prod",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	})
	require.NoError(t, err)
	require.Len(t, resp.Maps, 1)
	assert.Len(t, resp.Maps[0].HeatMap, 150, "heat points past the first page")
	assert.EqualValues(t, 150, resp.Sample["totalEvents"])
}

// A window larger than the aggregation bound must disclose a partial sample
// instead of presenting an excerpt as complete.
func TestBehaviorWindowsDisclosePartialSampleV24(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1",
		behaviorWindowEvents(30, "login", "game1", now.Add(-time.Minute), nil))

	prev := maxWindowEvents
	maxWindowEvents = 10
	defer func() { maxWindowEvents = prev }()

	resp, err := behaviorAnalytics(context.Background(), svcCtx, &BehaviorRequest{
		GameId:    "game1",
		Env:       "prod",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Sample)
	assert.EqualValues(t, 30, resp.Sample["totalEvents"])
	assert.EqualValues(t, 10, resp.Sample["scannedEvents"])
	assert.Equal(t, true, resp.Sample["sampled"])
	assert.Equal(t, "partial", resp.Sample["coverage"])
}

// scanBehaviorEvents must count every type in a multi-type scan and page
// through each type independently.
func TestScanBehaviorEventsMultiTypeTotalsV24(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1", behaviorWindowEvents(120, "level_attempt", "game1", now.Add(-time.Minute), nil))
	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1", behaviorWindowEvents(120, "level_complete", "game1", now.Add(-time.Minute), nil))

	events, sample, err := scanBehaviorEvents(context.Background(), svcCtx, "game1", "prod",
		now.Add(-24*time.Hour), now, []string{"level_attempt", "level_complete"}, 100)
	require.NoError(t, err)
	assert.Len(t, events, 240)
	assert.EqualValues(t, 240, sample.Total)
	assert.Equal(t, 240, sample.Scanned)
	assert.False(t, sample.Truncated)
}

// The interactive events endpoint keeps its own page + total semantics; the
// change must not turn it into a full-window aggregate.
func TestBehaviorEventsEndpointKeepsPagedSemanticsV24(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	seedBehaviorWindow(t, svcCtx.BehaviorModel, "game1",
		behaviorWindowEvents(150, "login", "game1", now.Add(-time.Minute), nil))

	resp, err := behaviorEvents(context.Background(), svcCtx, &BehaviorEventsRequest{
		GameId:    "game1",
		Env:       "prod",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		Limit:     10,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 10)
	assert.Equal(t, 150, resp.Total)
}
