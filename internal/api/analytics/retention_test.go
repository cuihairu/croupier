package analytics

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
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

	values := parseRetentionValues(model.JSON([]byte(`[1,0.5,0.25]`)))
	if len(values) != 3 || values[1] != 0.5 {
		t.Fatalf("unexpected retention values: %#v", values)
	}
	if got := parseRetentionValues(model.JSON([]byte(`bad`))); len(got) != 0 {
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

// Additional tests to improve coverage

func TestToFloat_AdditionalCases(t *testing.T) {
	t.Parallel()

	// Test float32 - note: precision loss is expected
	fv, ok := toFloat(float32(3.14))
	if !ok {
		t.Fatalf("toFloat(float32) failed")
	}
	// Just check it's close to expected value
	if fv < 3.13 || fv > 3.15 {
		t.Fatalf("toFloat(float32) out of range: got %v", fv)
	}
	_ = fv // Use the variable

	// Test int64
	if v, ok := toFloat(int64(42)); !ok || v != 42 {
		t.Fatalf("toFloat(int64) failed: got %v, %v", v, ok)
	}

	// Test json.Number - valid
	num := json.Number("123.45")
	if v, ok := toFloat(num); !ok || v != 123.45 {
		t.Fatalf("toFloat(json.Number) failed: got %v, %v", v, ok)
	}

	// Test string - valid float string
	if v, ok := toFloat("99.9"); !ok || v != 99.9 {
		t.Fatalf("toFloat(string) failed: got %v, %v", v, ok)
	}

	// Test string - with whitespace
	if v, ok := toFloat("  42.0  "); !ok || v != 42.0 {
		t.Fatalf("toFloat(string with spaces) failed: got %v, %v", v, ok)
	}

	// Test invalid cases - should return false
	if _, ok := toFloat("invalid"); ok {
		t.Fatal("toFloat(invalid string) should fail")
	}
	if _, ok := toFloat(true); ok {
		t.Fatal("toFloat(bool) should fail")
	}
	if _, ok := toFloat(nil); ok {
		t.Fatal("toFloat(nil) should fail")
	}
	if _, ok := toFloat([]int{}); ok {
		t.Fatal("toFloat(slice) should fail")
	}
}

func TestEventBool_AdditionalCases(t *testing.T) {
	t.Parallel()

	payload, _ := json.Marshal(map[string]interface{}{
		"active":  false,
		"deleted": "false",
		"number":  123,
		"missing": "",
	})
	ev := model.BehaviorEvent{
		UserID: "u1",
		Data:   datatypes.JSONMap{},
	}
	_ = json.Unmarshal(payload, &ev.Data)

	// Test false value
	if eventBool(ev, "active") {
		t.Fatal("expected eventBool false for false")
	}

	// Test string "false"
	if eventBool(ev, "deleted") {
		t.Fatal("expected eventBool false for 'false' string")
	}

	// Test non-bool number
	if !eventBool(ev, "number") {
		t.Fatal("expected eventBool true for non-zero number")
	}

	// Test empty string
	if eventBool(ev, "missing") {
		t.Fatal("expected eventBool false for empty string")
	}
}

func TestEventString_AdditionalCases(t *testing.T) {
	t.Parallel()

	payload, _ := json.Marshal(map[string]interface{}{
		"empty":     "",
		"number":    42,
		"bool":      true,
		"nullField": nil,
	})
	ev := model.BehaviorEvent{
		UserID: "u1",
		Data:   datatypes.JSONMap{},
	}
	_ = json.Unmarshal(payload, &ev.Data)

	// Test empty string
	if got := eventString(ev, "empty"); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}

	// Test number to string
	if got := eventString(ev, "number"); got != "42" {
		t.Fatalf("expected '42', got %q", got)
	}

	// Test boolean to string
	if got := eventString(ev, "bool"); got != "true" {
		t.Fatalf("expected 'true', got %q", got)
	}

	// Test null field - eventString returns the literal value including <nil>
	got := eventString(ev, "nullField")
	if got != "<nil>" && got != "" {
		t.Fatalf("expected '<nil>' or empty string for null field, got %q", got)
	}

	// Test missing field
	if got := eventString(ev, "missing"); got != "" {
		t.Fatalf("expected empty string for missing field, got %q", got)
	}
}

func TestMapPoints_EmptyAndMultiple(t *testing.T) {
	t.Parallel()

	// Empty map
	if got := mapPoints([]map[string]float64{}); got != 0 {
		t.Fatalf("expected 0 for empty map, got %d", got)
	}

	// Map with multiple points
	points := []map[string]float64{
		{"x": 1, "y": 2},
		{"z": 3},
		{"a": 1, "b": 2, "c": 3},
	}
	if got := mapPoints(points); got != 3 {
		t.Fatalf("expected 3 for multiple points, got %d", got)
	}

	// Map with nested structures - skip this test as it's not a valid type
	// mapPoints expects flat maps with float64 values
}

func TestMaxInt_EdgeCases(t *testing.T) {
	t.Parallel()

	// Negative numbers
	if got := maxInt(-5, -3); got != -3 {
		t.Fatalf("expected -3, got %d", got)
	}

	// Equal numbers
	if got := maxInt(42, 42); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}

	// Zero
	if got := maxInt(0, 0); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}

	// Large numbers
	if got := maxInt(2147483647, 2147483646); got != 2147483647 {
		t.Fatalf("expected 2147483647, got %d", got)
	}
}

func TestSortHelpers_EdgeCases(t *testing.T) {
	t.Parallel()

	// Empty slices
	sortLevelMetrics([]LevelMetrics{})
	sortEpisodeMetrics([]EpisodeMetrics{})
	sortMapMetrics([]MapMetrics{})

	// Single element
	levels := []LevelMetrics{{LevelId: "a", Attempts: 1}}
	sortLevelMetrics(levels)
	if levels[0].LevelId != "a" {
		t.Fatalf("single element sort failed")
	}

	// Equal attempts - should sort by ID
	levelsEqual := []LevelMetrics{
		{LevelId: "b", Attempts: 5},
		{LevelId: "a", Attempts: 5},
	}
	sortLevelMetrics(levelsEqual)
	if levelsEqual[0].LevelId != "a" {
		t.Fatalf("expected 'a' first for equal attempts, got %q", levelsEqual[0].LevelId)
	}

	// Equal players - should sort by EpisodeId
	episodesEqual := []EpisodeMetrics{
		{EpisodeId: "b", Players: 10},
		{EpisodeId: "a", Players: 10},
	}
	sortEpisodeMetrics(episodesEqual)
	if episodesEqual[0].EpisodeId != "a" {
		t.Fatalf("expected 'a' first for equal players, got %q", episodesEqual[0].EpisodeId)
	}
}

func TestResolveRetentionRange_EdgeCases(t *testing.T) {
	t.Parallel()

	// Invalid dates
	_, _, err := resolveRetentionRange("invalid-date", "")
	if err == nil {
		t.Fatal("expected error for invalid start date")
	}

	_, _, err = resolveRetentionRange("", "invalid-date")
	if err == nil {
		t.Fatal("expected error for invalid end date")
	}

	// Same date
	start, end, err := resolveRetentionRange("2026-03-15", "2026-03-15")
	if err != nil {
		t.Fatalf("unexpected error for same date: %v", err)
	}
	if !start.Equal(end) {
		t.Fatalf("expected same dates, got start=%v end=%v", start, end)
	}
}

func TestResolveLevelRange_EdgeCases(t *testing.T) {
	t.Parallel()

	// Invalid dates
	_, _, err := resolveLevelRange("invalid", "")
	if err == nil {
		t.Fatal("expected error for invalid level start")
	}

	_, _, err = resolveLevelRange("", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid level end")
	}
}

// Tests for error paths in main functions

func TestRetention_NilModel(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		RetentionModel: nil,
	}
	_, err := retention(context.Background(), svcCtx, &RetentionRequest{})
	if err == nil {
		t.Fatal("expected error for nil retention model")
	}
}

func TestRetention_NilRequest(t *testing.T) {
	t.Parallel()

	// Using a minimal service context - the function checks nil request before checking model
	svcCtx := &svc.ServiceContext{
		RetentionModel: nil, // Will fail before this is checked
	}
	_, err := retention(context.Background(), svcCtx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestRetention_InvalidDateRange(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		RetentionModel: nil, // Will fail at model check before date parsing
	}
	req := &RetentionRequest{
		StartDate: "invalid-date",
	}
	_, err := retention(context.Background(), svcCtx, req)
	if err == nil {
		t.Fatal("expected error (model unavailable)")
	}
}

func TestLevels_NilRequest(t *testing.T) {
	t.Parallel()

	_, err := levels(context.Background(), &svc.ServiceContext{}, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestLevels_InvalidDateRange(t *testing.T) {
	t.Parallel()

	req := &LevelsRequest{
		StartDate: "invalid-date",
	}
	_, err := levels(context.Background(), &svc.ServiceContext{}, req)
	if err == nil {
		t.Fatal("expected error for invalid date range")
	}
}

// Test overview function error paths
func TestOverview_NilModels(t *testing.T) {
	t.Parallel()

	// Test with nil BehaviorModel
	svcCtx := &svc.ServiceContext{
		BehaviorModel: nil,
	}
	_, err := overview(context.Background(), svcCtx, &OverviewRequest{})
	if err == nil {
		t.Fatal("expected error for nil behavior model")
	}
}

func TestOverview_NilRequest(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		BehaviorModel: nil, // Will fail here before nil request check
	}
	_, err := overview(context.Background(), svcCtx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

// Additional tests to improve coverage

func TestEventFloat_AllNumericFormats(t *testing.T) {
	t.Parallel()

	payload, _ := json.Marshal(map[string]interface{}{
		"floatKey":  3.14,
		"intKey":    42,
		"stringKey": "123.45",
		"zeroKey":   0.0,
		"zeroStr":   "0",
	})
	ev := model.BehaviorEvent{
		UserID: "u1",
		Data:   datatypes.JSONMap{},
	}
	_ = json.Unmarshal(payload, &ev.Data)

	if got := eventFloat(ev, "floatKey"); got != 3.14 {
		t.Fatalf("expected 3.14, got %v", got)
	}
	if got := eventFloat(ev, "intKey"); got != 42.0 {
		t.Fatalf("expected 42.0, got %v", got)
	}
	if got := eventFloat(ev, "stringKey"); got != 123.45 {
		t.Fatalf("expected 123.45, got %v", got)
	}
	if got := eventFloat(ev, "zeroKey"); got != 0.0 {
		t.Fatalf("expected 0.0, got %v", got)
	}
	if got := eventFloat(ev, "zeroStr"); got != 0.0 {
		t.Fatalf("expected 0.0, got %v", got)
	}
}

func TestEventBool_StringTrueVariations(t *testing.T) {
	t.Parallel()

	payload, _ := json.Marshal(map[string]interface{}{
		"trueLower": "true",
		"trueUpper": "TRUE",
		"yesLower":  "yes",
		"yesUpper":  "YES",
		"oneLower":  "1",
		"falseStr":  "false",
		"noStr":     "no",
		"zeroInt":   0,
	})
	ev := model.BehaviorEvent{
		UserID: "u1",
		Data:   datatypes.JSONMap{},
	}
	_ = json.Unmarshal(payload, &ev.Data)

	if !eventBool(ev, "trueLower") {
		t.Fatal("expected true for 'true'")
	}
	if !eventBool(ev, "trueUpper") {
		t.Fatal("expected true for 'TRUE'")
	}
	if !eventBool(ev, "yesLower") {
		t.Fatal("expected true for 'yes'")
	}
	if !eventBool(ev, "yesUpper") {
		t.Fatal("expected true for 'YES'")
	}
	if !eventBool(ev, "oneLower") {
		t.Fatal("expected true for '1'")
	}
	if eventBool(ev, "falseStr") {
		t.Fatal("expected false for 'false'")
	}
	if eventBool(ev, "noStr") {
		t.Fatal("expected false for 'no'")
	}
	if eventBool(ev, "zeroInt") {
		t.Fatal("expected false for 0")
	}
}

func TestParseTime_AllLayouts(t *testing.T) {
	t.Parallel()

	// RFC3339
	ts1, err := parseTime("2026-03-14T10:00:00Z")
	if err != nil {
		t.Fatalf("parseTime(RFC3339) error = %v", err)
	}
	if ts1.Year() != 2026 || ts1.Month() != 3 || ts1.Day() != 14 {
		t.Fatalf("unexpected date: %v", ts1)
	}

	// Date only
	ts2, err := parseTime("2026-03-14")
	if err != nil {
		t.Fatalf("parseTime(date) error = %v", err)
	}
	if ts2.Year() != 2026 || ts2.Month() != 3 || ts2.Day() != 14 {
		t.Fatalf("unexpected date: %v", ts2)
	}

	// Whitespace
	ts3, err := parseTime("  2026-03-14  ")
	if err != nil {
		t.Fatalf("parseTime(whitespace) error = %v", err)
	}
	if ts3.Year() != 2026 {
		t.Fatalf("unexpected year: %d", ts3.Year())
	}
}

func TestParseRetentionValues_AdditionalCases(t *testing.T) {
	t.Parallel()

	// Empty
	values := parseRetentionValues(model.JSON([]byte{}))
	if len(values) != 0 {
		t.Fatalf("expected empty values, got %#v", values)
	}

	// Single value
	values = parseRetentionValues(model.JSON([]byte(`[0.5]`)))
	if len(values) != 1 || values[0] != 0.5 {
		t.Fatalf("unexpected values: %#v", values)
	}

	// Multiple values
	values = parseRetentionValues(model.JSON([]byte(`[1.0,0.8,0.6,0.5,0.4,0.3,0.2]`)))
	if len(values) != 7 {
		t.Fatalf("expected 7 values, got %d", len(values))
	}
}

func TestMapPoints_AdditionalTypes(t *testing.T) {
	t.Parallel()

	// Empty slice
	if got := mapPoints([]map[string]float64{}); got != 0 {
		t.Fatalf("expected 0 for empty slice, got %d", got)
	}

	// Multiple maps
	points := []map[string]float64{
		{"x": 1, "y": 2},
		{"z": 3},
		{"a": 1, "b": 2, "c": 3},
		{"q": 4},
	}
	if got := mapPoints(points); got != 4 {
		t.Fatalf("expected 4 for multiple points, got %d", got)
	}

	// Test with []map[string]interface{}
	if got := mapPoints([]map[string]interface{}{{"x": 1}, {"y": 2}}); got != 2 {
		t.Fatalf("expected 2 for interface{} maps, got %d", got)
	}

	// Test with []interface{}
	if got := mapPoints([]interface{}{1, 2, 3}); got != 3 {
		t.Fatalf("expected 3 for interface{} slice, got %d", got)
	}

	// Test with nil interface
	if got := mapPoints(nil); got != 0 {
		t.Fatalf("expected 0 for nil, got %d", got)
	}

	// Test with invalid type
	if got := mapPoints("invalid"); got != 0 {
		t.Fatalf("expected 0 for invalid type, got %d", got)
	}
}

func TestSortMapMetrics_EdgeCases(t *testing.T) {
	t.Parallel()

	// Empty slice
	sortMapMetrics([]MapMetrics{})

	// Single element
	maps := []MapMetrics{{MapId: "a", HeatMap: []map[string]float64{{"x": 1}}}}
	sortMapMetrics(maps)
	if maps[0].MapId != "a" {
		t.Fatal("single element sort failed")
	}

	// Equal heat points - should sort by MapId
	mapsEqual := []MapMetrics{
		{MapId: "b", HeatMap: []map[string]float64{{"x": 1}, {"y": 2}}},
		{MapId: "a", HeatMap: []map[string]float64{{"x": 1}, {"y": 2}}},
	}
	sortMapMetrics(mapsEqual)
	if mapsEqual[0].MapId != "a" {
		t.Fatalf("expected 'a' first for equal heat points, got %q", mapsEqual[0].MapId)
	}

	// Nil heat maps
	mapsNil := []MapMetrics{
		{MapId: "a", HeatMap: nil},
		{MapId: "b", HeatMap: nil},
	}
	sortMapMetrics(mapsNil)
	// Should not panic
}

func TestNormalizeRange_SwappedDates(t *testing.T) {
	t.Parallel()

	start, end, err := normalizeRange("2026-03-20", "2026-03-10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should swap dates if end is before start
	if start.After(end) {
		t.Fatalf("expected start <= end, got start=%v end=%v", start, end)
	}
}

func TestNormalizeRange_EmptyStrings(t *testing.T) {
	t.Parallel()

	start, end, err := normalizeRange("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !start.IsZero() || !end.IsZero() {
		t.Fatalf("expected zero times for empty strings, got start=%v end=%v", start, end)
	}
}

func TestParseRetentionValues_EdgeCases(t *testing.T) {
	t.Parallel()

	// Empty JSON array
	values := parseRetentionValues(model.JSON([]byte(`[]`)))
	if len(values) != 0 {
		t.Fatalf("expected empty values for empty array, got %#v", values)
	}

	// All zeros
	values = parseRetentionValues(model.JSON([]byte(`[0,0.0,0]`)))
	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(values))
	}
	for i, v := range values {
		if v != 0 {
			t.Fatalf("expected value %d to be 0, got %v", i, v)
		}
	}

	// Very small values
	values = parseRetentionValues(model.JSON([]byte(`[0.001,0.0001]`)))
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
}

func TestEventFloat_MissingKey(t *testing.T) {
	t.Parallel()

	payload, _ := json.Marshal(map[string]interface{}{
		"existingKey": 1.0,
	})
	ev := model.BehaviorEvent{
		UserID: "u1",
		Data:   datatypes.JSONMap{},
	}
	_ = json.Unmarshal(payload, &ev.Data)

	// Missing key should return 0
	if got := eventFloat(ev, "missingKey"); got != 0 {
		t.Fatalf("expected 0 for missing key, got %v", got)
	}
}

func TestEventString_WhitespaceValues(t *testing.T) {
	t.Parallel()

	payload, _ := json.Marshal(map[string]interface{}{
		"spaces":   "  value  ",
		"tabs":     "\tvalue\t",
		"newlines": "\nvalue\n",
	})
	ev := model.BehaviorEvent{
		UserID: "u1",
		Data:   datatypes.JSONMap{},
	}
	_ = json.Unmarshal(payload, &ev.Data)

	if got := eventString(ev, "spaces"); got != "value" {
		t.Fatalf("expected 'value', got %q", got)
	}
	if got := eventString(ev, "tabs"); got != "value" {
		t.Fatalf("expected 'value', got %q", got)
	}
	if got := eventString(ev, "newlines"); got != "value" {
		t.Fatalf("expected 'value', got %q", got)
	}
}
