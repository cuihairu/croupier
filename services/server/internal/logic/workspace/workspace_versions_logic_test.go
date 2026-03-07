package workspace

import (
	"testing"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"gorm.io/gorm"
)

func TestParseWorkspaceVersionTimeRange(t *testing.T) {
	from, to, err := parseWorkspaceVersionTimeRange("2026-03-01T00:00:00Z", "2026-03-07T00:00:00Z")
	if err != nil {
		t.Fatalf("parse time range should succeed: %v", err)
	}
	if from == nil || to == nil {
		t.Fatalf("from/to should not be nil")
	}

	if _, _, err := parseWorkspaceVersionTimeRange("bad-time", ""); err == nil {
		t.Fatalf("invalid from should fail")
	}
	if _, _, err := parseWorkspaceVersionTimeRange("2026-03-07T00:00:00Z", "2026-03-01T00:00:00Z"); err == nil {
		t.Fatalf("from after to should fail")
	}
}

func TestIsWorkspaceVersionWithin(t *testing.T) {
	created := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	record := model.ConfigVersion{
		Model: gorm.Model{
			CreatedAt: created,
		},
	}

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	if !isWorkspaceVersionWithin(record, &from, &to) {
		t.Fatalf("record should be within range")
	}

	lateFrom := time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)
	if isWorkspaceVersionWithin(record, &lateFrom, &to) {
		t.Fatalf("record should be excluded by from")
	}
}

func TestMapWorkspaceVersionRecord(t *testing.T) {
	record := model.ConfigVersion{
		Version:   3,
		Value:     `{"objectKey":"player","title":"v3","layout":{"type":"tabs","tabs":[]}}`,
		CreatedBy: "tester",
		Message:   "save workspace config",
	}
	record.CreatedAt = time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)

	item, err := mapWorkspaceVersionRecord("player", record)
	if err != nil {
		t.Fatalf("map record should succeed: %v", err)
	}
	if item["id"] != "3" {
		t.Fatalf("expected id=3, got %v", item["id"])
	}
	if item["objectKey"] != "player" {
		t.Fatalf("expected objectKey=player, got %v", item["objectKey"])
	}
}

func TestIsPublishedWorkspaceVersion(t *testing.T) {
	if !isPublishedWorkspaceVersion(&types.WorkspaceConfig{Published: true, Status: "draft"}) {
		t.Fatalf("published=true should be treated as published")
	}
	if isPublishedWorkspaceVersion(&types.WorkspaceConfig{Published: false, Status: "draft"}) {
		t.Fatalf("draft should not be treated as published")
	}
}

func TestWithWorkspaceVersionState(t *testing.T) {
	items := []map[string]interface{}{
		{"version": 3},
		{"version": 2},
		{"version": 1},
	}
	got := withWorkspaceVersionState(items, 3, 2)
	if got[0]["isCurrentDraft"] != true {
		t.Fatalf("version 3 should be current draft")
	}
	if got[1]["isCurrentPublished"] != true {
		t.Fatalf("version 2 should be current published")
	}
}
