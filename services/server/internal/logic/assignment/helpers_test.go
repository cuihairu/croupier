package assignment

import (
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/svc"
)

func TestAssignmentHistoryAppendAndLoad(t *testing.T) {
	base := t.TempDir()
	ctx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{BaseDir: base},
			Registry:      config.RegistryConfig{AssignmentsPath: "assignments.json"},
		},
	}

	if err := appendAssignmentHistory(ctx, assignmentHistoryEntry{
		GameID:     "game-a",
		Env:        "prod",
		FunctionID: "all",
		Action:     "assign",
		Count:      2,
		OperatedBy: "tester",
	}); err != nil {
		t.Fatalf("append first history failed: %v", err)
	}
	if err := appendAssignmentHistory(ctx, assignmentHistoryEntry{
		GameID:     "game-a",
		Env:        "prod",
		FunctionID: "all",
		Action:     "assign",
		Count:      3,
		OperatedBy: "tester",
	}); err != nil {
		t.Fatalf("append second history failed: %v", err)
	}

	entries, err := loadAssignmentHistory(assignmentHistoryPath(ctx))
	if err != nil {
		t.Fatalf("load history failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(entries))
	}
	if entries[0].Count != 3 {
		t.Fatalf("expected newest entry first, got count=%d", entries[0].Count)
	}
	if entries[1].Count != 2 {
		t.Fatalf("expected oldest entry second, got count=%d", entries[1].Count)
	}
}

func TestAssignmentHistoryPathFollowsAssignmentsPath(t *testing.T) {
	base := t.TempDir()
	ctx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{BaseDir: base},
			Registry:      config.RegistryConfig{AssignmentsPath: filepath.Join("x", "assignments.json")},
		},
	}
	historyPath := assignmentHistoryPath(ctx)
	if filepath.Base(historyPath) != "assignments_history.json" {
		t.Fatalf("unexpected history filename: %s", historyPath)
	}
	if filepath.Dir(historyPath) != filepath.Join(base, "x") {
		t.Fatalf("unexpected history directory: %s", filepath.Dir(historyPath))
	}
}
