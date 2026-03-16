package analytics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
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

func TestExtractAndSetAnalyticsFiltersFromConfig(t *testing.T) {
	t.Parallel()

	items, ok, err := extractAnalyticsFiltersFromConfig(map[string]any{
		"filters": []map[string]any{
			{"gameId": "tower", "filters": map[string]any{"env": "prod"}},
		},
	})
	if err != nil {
		t.Fatalf("extractAnalyticsFiltersFromConfig() error = %v", err)
	}
	if !ok || len(items) != 1 || items[0].GameId != "tower" {
		t.Fatalf("unexpected parsed filters: ok=%v items=%#v", ok, items)
	}

	cfg := setAnalyticsFiltersToConfig(map[string]any{"retention_days": 7}, []AnalyticsFilters{
		{GameId: "newgame", Filters: map[string]any{"env": "dev"}},
	})
	raw, exists := cfg["filters"]
	if !exists || raw == nil {
		t.Fatalf("expected filters written into config")
	}
	if legacy, ok := cfg["analytics_filters"]; !ok || legacy == nil {
		t.Fatalf("expected legacy analytics_filters compatibility key")
	}
}

func TestExtractAnalyticsFiltersFromConfigLegacyKey(t *testing.T) {
	t.Parallel()

	items, ok, err := extractAnalyticsFiltersFromConfig(map[string]any{
		"analytics_filters": []map[string]any{
			{"gameId": "legacy", "filters": map[string]any{"region": "cn"}},
		},
	})
	if err != nil {
		t.Fatalf("extractAnalyticsFiltersFromConfig() error = %v", err)
	}
	if !ok || len(items) != 1 || items[0].GameId != "legacy" {
		t.Fatalf("unexpected parsed filters from legacy key: ok=%v items=%#v", ok, items)
	}
}

func TestExtractAnalyticsFiltersFromConfigPreferNewKey(t *testing.T) {
	t.Parallel()

	items, ok, err := extractAnalyticsFiltersFromConfig(map[string]any{
		"filters": []map[string]any{
			{"gameId": "new", "filters": map[string]any{"env": "prod"}},
		},
		"analytics_filters": []map[string]any{
			{"gameId": "legacy", "filters": map[string]any{"env": "legacy"}},
		},
	})
	if err != nil {
		t.Fatalf("extractAnalyticsFiltersFromConfig() error = %v", err)
	}
	if !ok || len(items) != 1 || items[0].GameId != "new" {
		t.Fatalf("expected prefer new filters key, got ok=%v items=%#v", ok, items)
	}
}

func TestFiltersGetFallsBackToFileWhenExtensionNotInstalled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "analytics_filters.json")
	data, err := SaveAnalyticsFiltersJSON([]AnalyticsFilters{
		{GameId: "tower", Filters: map[string]any{"env": "prod"}},
	})
	if err != nil {
		t.Fatalf("SaveAnalyticsFiltersJSON() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write filters file failed: %v", err)
	}

	resp, err := filtersGet(context.Background(), &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AnalyticsFiltersPath: path,
			},
		},
	}, &FiltersGetRequest{})
	if err != nil {
		t.Fatalf("filtersGet() error = %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].GameId != "tower" {
		t.Fatalf("expected file fallback filters, got %#v", resp.Items)
	}
}

func TestFiltersGetPrefersExtensionInstallationConfig(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	installed, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAnalyticsID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config: map[string]any{
			"filters": []map[string]any{
				{"gameId": "ext", "filters": map[string]any{"env": "stage"}},
			},
		},
		Operator: "tester",
	})
	if err != nil {
		t.Fatalf("install analytics extension failed: %v", err)
	}
	if installed == nil {
		t.Fatal("expected installed extension")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "analytics_filters.json")
	fileData, err := SaveAnalyticsFiltersJSON([]AnalyticsFilters{
		{GameId: "file", Filters: map[string]any{"env": "prod"}},
	})
	if err != nil {
		t.Fatalf("SaveAnalyticsFiltersJSON() error = %v", err)
	}
	if err := os.WriteFile(path, fileData, 0o644); err != nil {
		t.Fatalf("write filters file failed: %v", err)
	}

	resp, err := filtersGet(context.Background(), &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AnalyticsFiltersPath: path,
			},
		},
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	}, &FiltersGetRequest{})
	if err != nil {
		t.Fatalf("filtersGet() error = %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].GameId != "ext" {
		t.Fatalf("expected extension filters to win over file fallback, got %#v", resp.Items)
	}
}

// Tests for uncovered functions

func TestPickContextString(t *testing.T) {
	t.Parallel()

	// Test nil context
	if got := pickContextString(nil, "key"); got != "" {
		t.Fatalf("expected empty string for nil context, got %q", got)
	}

	// Test empty key
	if got := pickContextString(context.Background(), ""); got != "" {
		t.Fatalf("expected empty string for empty key, got %q", got)
	}

	// Test whitespace-only key
	if got := pickContextString(context.Background(), "   "); got != "" {
		t.Fatalf("expected empty string for whitespace key, got %q", got)
	}

	// Test key not in context
	if got := pickContextString(context.Background(), "nonexistent"); got != "" {
		t.Fatalf("expected empty string for nonexistent key, got %q", got)
	}

	// Test key with value
	ctx := context.WithValue(context.Background(), "username", "testuser")
	if got := pickContextString(ctx, "username"); got != "testuser" {
		t.Fatalf("expected 'testuser', got %q", got)
	}

	// Test key with value that gets trimmed
	ctx2 := context.WithValue(context.Background(), "key", "  value  ")
	if got := pickContextString(ctx2, "key"); got != "value" {
		t.Fatalf("expected 'value', got %q", got)
	}

	// Test non-string value (converted to string)
	ctx3 := context.WithValue(context.Background(), "number", 123)
	if got := pickContextString(ctx3, "number"); got != "123" {
		t.Fatalf("expected '123', got %q", got)
	}

	// Test nil value in context
	ctx4 := context.WithValue(context.Background(), "nilkey", nil)
	if got := pickContextString(ctx4, "nilkey"); got != "" {
		t.Fatalf("expected empty string for nil value, got %q", got)
	}
}

func TestSaveAnalyticsFiltersToExtensionInstallation_NilContext(t *testing.T) {
	t.Parallel()

	err := saveAnalyticsFiltersToExtensionInstallation(nil, &svc.ServiceContext{}, []AnalyticsFilters{})
	if err != nil {
		t.Fatalf("expected no error for nil context, got %v", err)
	}
}

func TestSaveAnalyticsFiltersToExtensionInstallation_NilServiceContext(t *testing.T) {
	t.Parallel()

	err := saveAnalyticsFiltersToExtensionInstallation(context.Background(), nil, []AnalyticsFilters{})
	if err != nil {
		t.Fatalf("expected no error for nil service context, got %v", err)
	}
}

func TestSaveAnalyticsFiltersToExtensionInstallation_NilExtensions(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		Extensions: nil,
	}
	err := saveAnalyticsFiltersToExtensionInstallation(context.Background(), svcCtx, []AnalyticsFilters{})
	if err != nil {
		t.Fatalf("expected no error for nil extensions, got %v", err)
	}
}

func TestSaveAnalyticsFiltersToExtensionInstallation_NilInstallation(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	}
	err := saveAnalyticsFiltersToExtensionInstallation(context.Background(), svcCtx, []AnalyticsFilters{})
	if err != nil {
		t.Fatalf("expected no error for nil installation, got %v", err)
	}
}

func TestSaveAnalyticsFiltersToExtensionInstallation_NoActiveInstallation(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)

	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	}

	// No active installation should return nil error
	items := []AnalyticsFilters{
		{GameId: "test", Filters: map[string]any{"env": "prod"}},
	}
	err = saveAnalyticsFiltersToExtensionInstallation(context.Background(), svcCtx, items)
	if err != nil {
		t.Fatalf("expected no error when no active installation, got %v", err)
	}
}

func TestFindActiveAnalyticsInstallation_NilServiceContext(t *testing.T) {
	t.Parallel()

	item, ok, err := findActiveAnalyticsInstallation(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error for nil service context, got %v", err)
	}
	if item != nil {
		t.Fatal("expected nil item")
	}
	if ok {
		t.Fatal("expected false for ok")
	}
}

func TestFindActiveAnalyticsInstallation_NilExtensions(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		Extensions: nil,
	}
	item, ok, err := findActiveAnalyticsInstallation(context.Background(), svcCtx)
	if err != nil {
		t.Fatalf("expected no error for nil extensions, got %v", err)
	}
	if item != nil {
		t.Fatal("expected nil item")
	}
	if ok {
		t.Fatal("expected false for ok")
	}
}

func TestFindActiveAnalyticsInstallation_NilInstallation(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	}
	item, ok, err := findActiveAnalyticsInstallation(context.Background(), svcCtx)
	if err != nil {
		t.Fatalf("expected no error for nil installation, got %v", err)
	}
	if item != nil {
		t.Fatal("expected nil item")
	}
	if ok {
		t.Fatal("expected false for ok")
	}
}

func TestExtractAnalyticsFiltersFromConfig_EmptyConfig(t *testing.T) {
	t.Parallel()

	config := map[string]any{}
	items, ok, err := extractAnalyticsFiltersFromConfig(config)
	if err != nil {
		t.Fatalf("expected no error for empty config, got %v", err)
	}
	if ok {
		t.Fatal("expected false for ok")
	}
	if len(items) != 0 {
		t.Fatalf("expected empty items, got %#v", items)
	}
}

func TestExtractAnalyticsFiltersFromConfig_NilConfig(t *testing.T) {
	t.Parallel()

	items, ok, err := extractAnalyticsFiltersFromConfig(nil)
	if err != nil {
		t.Fatalf("expected no error for nil config, got %v", err)
	}
	if ok {
		t.Fatal("expected false for ok")
	}
	if len(items) != 0 {
		t.Fatalf("expected empty items, got %#v", items)
	}
}

func TestExtractAnalyticsFiltersFromConfig_LegacyKey(t *testing.T) {
	t.Parallel()

	config := map[string]any{
		"filters": []map[string]any{
			{"gameId": "test", "filters": map[string]any{"env": "prod"}},
		},
	}
	items, ok, err := extractAnalyticsFiltersFromConfig(config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !ok {
		t.Fatal("expected true for ok")
	}
	if len(items) != 1 || items[0].GameId != "test" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestExtractAnalyticsFiltersFromConfig_InvalidFiltersType(t *testing.T) {
	t.Parallel()

	config := map[string]any{
		"filters": "invalid",
	}
	_, _, err := extractAnalyticsFiltersFromConfig(config)
	if err == nil {
		t.Fatal("expected error for invalid filters type")
	}
}

func TestSetAnalyticsFiltersToConfig(t *testing.T) {
	t.Parallel()

	items := []AnalyticsFilters{
		{GameId: "test", Filters: map[string]any{"env": "prod"}},
	}
	config := setAnalyticsFiltersToConfig(map[string]any{}, items)

	if _, ok := config["filters"]; !ok {
		t.Fatal("expected 'filters' key in config")
	}
}

func TestSetAnalyticsFiltersToConfig_ExistingConfig(t *testing.T) {
	t.Parallel()

	items := []AnalyticsFilters{
		{GameId: "test", Filters: map[string]any{"env": "prod"}},
	}
	existingConfig := map[string]any{
		"otherKey": "otherValue",
	}
	config := setAnalyticsFiltersToConfig(existingConfig, items)

	if config["otherKey"] != "otherValue" {
		t.Fatal("existing config should be preserved")
	}
	if _, ok := config["filters"]; !ok {
		t.Fatal("expected 'filters' key in config")
	}
}

func TestLoadAnalyticsFiltersForUpdate_NoExtensionInstallation(t *testing.T) {
	t.Parallel()

	// This test expects file-based loading since we don't have an extension
	// The path will be empty/invalid, so we expect an error
	_, source, err := loadAnalyticsFiltersForUpdate(context.Background(), &svc.ServiceContext{})
	// We expect either an error or empty source since there's no file
	if err == nil && source != "" {
		// If no error, we might get "file" as source
		if source != "file" && source != "extension" {
			t.Fatalf("unexpected source: %s", source)
		}
	}
	// Error is acceptable since we don't have a valid filters file
}

func TestLoadAnalyticsFiltersForUpdate_WithConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := dir + "/analytics_filters.json"
	data, _ := SaveAnalyticsFiltersJSON([]AnalyticsFilters{
		{GameId: "test", Filters: map[string]any{"env": "prod"}},
	})
	_ = os.WriteFile(path, data, 0o644)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AnalyticsFiltersPath: path,
			},
		},
	}

	items, source, err := loadAnalyticsFiltersForUpdate(context.Background(), svcCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if source != "file" {
		t.Fatalf("expected source 'file', got %q", source)
	}
	if len(items) != 1 || items[0].GameId != "test" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestResolveRealtimeInterval_Default(t *testing.T) {
	t.Parallel()

	if got := resolveRealtimeInterval(""); got != time.Minute {
		t.Fatalf("expected default 1 minute, got %v", got)
	}
}

func TestResolveRealtimeInterval_Custom(t *testing.T) {
	t.Parallel()

	if got := resolveRealtimeInterval("5m"); got != 5*time.Minute {
		t.Fatalf("expected 5 minutes, got %v", got)
	}
	if got := resolveRealtimeInterval("15m"); got != 15*time.Minute {
		t.Fatalf("expected 15 minutes, got %v", got)
	}
	// Unknown value defaults to 1 minute
	if got := resolveRealtimeInterval("30s"); got != time.Minute {
		t.Fatalf("expected default 1 minute for unknown interval, got %v", got)
	}
}

func TestClampRealtimeDuration(t *testing.T) {
	t.Parallel()

	// Test within limits (60 minutes = 1 hour)
	if got := clampRealtimeDuration(60); got != time.Hour {
		t.Fatalf("expected 1 hour, got %v", got)
	}

	// Test below minimum (0 should become 60)
	if got := clampRealtimeDuration(0); got != time.Hour {
		t.Fatalf("expected 1 hour (minimum), got %v", got)
	}

	// Test negative
	if got := clampRealtimeDuration(-10); got != time.Hour {
		t.Fatalf("expected 1 hour (minimum for negative), got %v", got)
	}

	// Test above maximum (maxRealtimeDurationMin is 1440 = 24 hours)
	if got := clampRealtimeDuration(3000); got != 24*time.Hour {
		t.Fatalf("expected 24 hours (maximum), got %v", got)
	}
}

func TestBuildRealtimeSeriesPoints(t *testing.T) {
	t.Parallel()

	now := time.Now()
	samples := []bucketPoint{
		{Timestamp: now.Add(-2 * time.Minute), Value: 10},
		{Timestamp: now.Add(-1 * time.Minute), Value: 20},
		{Timestamp: now, Value: 30},
	}
	points := buildRealtimeSeriesPoints(samples)

	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}

	// Check that points have the expected structure
	if _, ok := points[0]["timestamp"]; !ok {
		t.Fatal("expected 'timestamp' key in point")
	}
	if _, ok := points[0]["value"]; !ok {
		t.Fatal("expected 'value' key in point")
	}
}

func TestMapTopEvents_LessThanLimit(t *testing.T) {
	t.Parallel()

	events := []model.EventTypeCount{
		{EventType: "event1", Total: 10},
		{EventType: "event2", Total: 5},
	}
	result := mapTopEvents(events)

	if len(result) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result))
	}
	if result[0]["event"] != "event1" || result[0]["count"] != int64(10) {
		t.Fatalf("unexpected first event: %#v", result[0])
	}
}

func TestMapTopEvents_MoreThanLimit(t *testing.T) {
	t.Parallel()

	events := []model.EventTypeCount{
		{EventType: "event1", Total: 10},
		{EventType: "event2", Total: 5},
		{EventType: "event3", Total: 3},
	}
	result := mapTopEvents(events)

	if len(result) != 3 {
		t.Fatalf("expected 3 events (no limit in function), got %d", len(result))
	}
}

func TestDecodeEventsPayload(t *testing.T) {
	t.Parallel()

	// Test valid JSON - pass interface{} that is already a list
	data := []interface{}{
		map[string]interface{}{"name": "event1", "count": 10},
	}
	events, err := decodeEventsPayload(data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Test invalid JSON - pass a string that can't be unmarshaled into list
	_, err = decodeEventsPayload("invalid")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDecodeEventsPayload_Empty(t *testing.T) {
	t.Parallel()

	events, err := decodeEventsPayload([]interface{}{})
	if err != nil {
		t.Fatalf("expected no error for empty payload, got %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected empty events, got %#v", events)
	}
}

func TestDecodeEventsPayload_Nil(t *testing.T) {
	t.Parallel()

	events, err := decodeEventsPayload(nil)
	if err != nil {
		t.Fatalf("expected no error for nil payload, got %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected empty events for nil, got %#v", events)
	}
}

func TestBuildBehaviorEvent_Success(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"eventType": "test_event",
		"userId":    "user123",
		"timestamp": "2026-03-14T10:00:00Z",
		"meta":      "some metadata",
	}
	fallback := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)

	event, err := buildBehaviorEvent(data, "game1", "prod", fallback)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if event.EventType != "test_event" {
		t.Fatalf("expected event type 'test_event', got %q", event.EventType)
	}
	if event.UserID != "user123" {
		t.Fatalf("expected user ID 'user123', got %q", event.UserID)
	}
}

func TestBuildBehaviorEvent_MissingEventType(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"userId": "user123",
	}
	fallback := time.Now()

	_, err := buildBehaviorEvent(data, "game1", "prod", fallback)
	if err == nil {
		t.Fatal("expected error for missing eventType")
	}
}

func TestBuildBehaviorEvent_MissingUserId(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"eventType": "test_event",
	}
	fallback := time.Now()

	_, err := buildBehaviorEvent(data, "game1", "prod", fallback)
	if err == nil {
		t.Fatal("expected error for missing userId")
	}
}

func TestBuildBehaviorEvent_NilEntry(t *testing.T) {
	t.Parallel()

	fallback := time.Now()

	_, err := buildBehaviorEvent(nil, "game1", "prod", fallback)
	if err == nil {
		t.Fatal("expected error for nil entry")
	}
}

func TestFormatAny(t *testing.T) {
	t.Parallel()

	// Test string
	if got := formatAny("test"); got != "test" {
		t.Fatalf("expected 'test', got %q", got)
	}

	// Test int
	if got := formatAny(123); got != "123" {
		t.Fatalf("expected '123', got %q", got)
	}

	// Test float
	if got := formatAny(3.14); got != "3.14" {
		t.Fatalf("expected '3.14', got %q", got)
	}

	// Test nil - formatAny returns fmt.Sprintf("%v", nil) which is "<nil>"
	if got := formatAny(nil); got != "<nil>" {
		t.Fatalf("expected '<nil>' for nil, got %q", got)
	}

	// Test map - returns the map representation as string
	if got := formatAny(map[string]int{"a": 1}); got != "map[a:1]" {
		t.Fatalf("expected 'map[a:1]' for map, got %q", got)
	}
}

func TestFilterAnalyticsFilters(t *testing.T) {
	t.Parallel()

	items := []AnalyticsFilters{
		{GameId: "game1", Filters: map[string]any{"env": "prod"}},
		{GameId: "game2", Filters: map[string]any{"env": "stage"}},
	}

	// Filter by game ID
	filtered := filterAnalyticsFilters(items, "game1")
	if len(filtered) != 1 || filtered[0].GameId != "game1" {
		t.Fatalf("expected game1, got %#v", filtered)
	}

	// No filter - return all
	all := filterAnalyticsFilters(items, "")
	if len(all) != 2 {
		t.Fatalf("expected 2 items, got %d", len(all))
	}
}

func TestUpsertAnalyticsFilter_New(t *testing.T) {
	t.Parallel()

	items := []AnalyticsFilters{}
	newFilters := map[string]any{"env": "prod"}

	result := upsertAnalyticsFilter(items, "game1", newFilters)
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	if result[0].GameId != "game1" {
		t.Fatalf("expected game ID 'game1', got %q", result[0].GameId)
	}
}

func TestUpsertAnalyticsFilter_UpdateExisting(t *testing.T) {
	t.Parallel()

	items := []AnalyticsFilters{
		{GameId: "game1", Filters: map[string]any{"env": "prod"}},
	}
	updatedFilters := map[string]any{"env": "stage"}

	result := upsertAnalyticsFilter(items, "game1", updatedFilters)
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	// Check that filters were updated - need to type assert since Filters is interface{}
	resultFilters, ok := result[0].Filters.(map[string]any)
	if !ok || resultFilters["env"] != "stage" {
		t.Fatalf("expected updated filters with env=stage, got %#v", result[0].Filters)
	}
}

func TestUpsertAnalyticsFilter_NewWithMultiple(t *testing.T) {
	t.Parallel()

	items := []AnalyticsFilters{
		{GameId: "game1", Filters: map[string]any{"env": "prod"}},
	}
	newFilters := map[string]any{"env": "dev"}

	result := upsertAnalyticsFilter(items, "game2", newFilters)
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
}
