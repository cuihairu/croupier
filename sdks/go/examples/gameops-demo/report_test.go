package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGameOpsReportGenerateScenarios covers the E2 abnormal-state scenarios the
// task-mode report.generate contract exists to reproduce: async delay (result
// unknown until the handler returns), failure, idempotent-retry evidence via
// the run sequence, and context cancellation during the delay window.
func TestGameOpsReportGenerateScenarios(t *testing.T) {
	store := newGameOpsStore()

	// Success path: immediate run (delayMs=0), run sequence starts at 1.
	got := invokeHandler(t, store, "report.generate", map[string]any{"range": "daily", "delayMs": 0})
	if got["success"] != true {
		t.Fatalf("report.generate success=%v, want true", got["success"])
	}
	if got["reportId"] == nil || got["reportId"] == "" {
		t.Fatalf("report.generate missing reportId: %+v", got)
	}
	if seq, _ := got["runSeq"].(float64); int(seq) != 1 {
		t.Fatalf("first runSeq=%v, want 1", got["runSeq"])
	}
	if got["range"] != "daily" {
		t.Fatalf("report.generate range=%v, want daily", got["range"])
	}

	// Delay: a nonzero delayMs actually waits (result unknown during the window).
	start := time.Now()
	invokeHandler(t, store, "report.generate", map[string]any{"range": "weekly", "delayMs": 120})
	if elapsed := time.Since(start); elapsed < 120*time.Millisecond {
		t.Fatalf("delayMs=120 elapsed only %s, want >= 120ms", elapsed)
	}

	// Failure scenario: fail=true returns an error after the delay completes.
	if err := invokeHandlerErr(t, store, "report.generate", map[string]any{"range": "monthly", "fail": true, "delayMs": 0}); err == nil || !strings.Contains(err.Error(), "report_generate_failed") {
		t.Fatalf("fail=true should return report_generate_failed, got %v", err)
	}

	// Idempotent-retry observable: each actual execution bumps runSeq; a failure
	// that reached the handler also counts (the platform dedupes by idempotency
	// key BEFORE the handler, so a deduped retry leaves runSeq unchanged).
	runSeqAfterFail := func() int {
		raw, err := store.reportGenerate(contextTODO(), mustJSON(t, map[string]any{"range": "daily", "delayMs": 0}))
		if err != nil {
			t.Fatalf("report.generate unexpected error: %v", err)
		}
		return int(listResult(t, raw)["runSeq"].(float64))
	}
	// runSeq so far: success(1) + delay(2) + fail(3) → next real run is 4.
	if seq := runSeqAfterFail(); seq != 4 {
		t.Fatalf("runSeq after next run = %d, want 4 (each execution counted once)", seq)
	}

	// Cancellation during the delay window: task stops waiting and records no run.
	before, err := store.reportGenerate(context.TODO(), mustJSON(t, map[string]any{"range": "daily", "delayMs": 0}))
	if err != nil {
		t.Fatalf("seed run failed: %v", err)
	}
	beforeSeq := int(listResult(t, before)["runSeq"].(float64))

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() {
		_, err := store.reportGenerate(cancelCtx, mustJSON(t, map[string]any{"range": "daily", "delayMs": 5000}))
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "report_generate_cancelled") {
			t.Fatalf("cancelled run err = %v, want report_generate_cancelled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled report.generate did not return promptly")
	}
	if after, _ := store.reportGenerate(contextTODO(), mustJSON(t, map[string]any{"range": "daily", "delayMs": 0})); after != nil {
		// Cancelled run must not have bumped the sequence past beforeSeq+1.
		if seq := int(listResult(t, after)["runSeq"].(float64)); seq != beforeSeq+1 {
			t.Fatalf("cancelled run must not record a run: runSeq=%d, want %d", seq, beforeSeq+1)
		}
	}

	// Cancellation with delayMs=0: the timer is already ready and a select would
	// pick a branch at random, so the handler must check ctx up front to honour
	// "cancelled task never records a run" deterministically.
	for i := 0; i < 20; i++ {
		before, err := store.reportGenerate(contextTODO(), mustJSON(t, map[string]any{"range": "daily", "delayMs": 0}))
		if err != nil {
			t.Fatalf("seed run failed: %v", err)
		}
		seqBefore := int(listResult(t, before)["runSeq"].(float64))
		cancelCtx0, cancel0 := context.WithCancel(context.Background())
		cancel0()
		if _, err := store.reportGenerate(cancelCtx0, mustJSON(t, map[string]any{"range": "daily", "delayMs": 0})); err == nil || !strings.Contains(err.Error(), "report_generate_cancelled") {
			t.Fatalf("pre-cancelled delayMs=0 run err = %v, want report_generate_cancelled", err)
		}
		after, err := store.reportGenerate(contextTODO(), mustJSON(t, map[string]any{"range": "daily", "delayMs": 0}))
		if err != nil {
			t.Fatalf("post seed run failed: %v", err)
		}
		if seq := int(listResult(t, after)["runSeq"].(float64)); seq != seqBefore+1 {
			t.Fatalf("pre-cancelled delayMs=0 run recorded a run: runSeq=%d, want %d", seq, seqBefore+1)
		}
	}
}

// TestGameOpsReportGenerateIsTaskMode asserts the descriptor is registered as a
// task-mode contract with the task capability (the console/generator emits a
// task binding for it so the async scenario is reachable from the UI).
func TestGameOpsReportGenerateIsTaskMode(t *testing.T) {
	store := newGameOpsStore()
	for _, def := range gameOpsFunctionDefinitions(store) {
		if def.desc.ID != "report.generate" {
			continue
		}
		if def.desc.Execution != "task" {
			t.Fatalf("report.generate execution=%q, want task", def.desc.Execution)
		}
		if def.desc.Capability != "task" {
			t.Fatalf("report.generate capability=%q, want task", def.desc.Capability)
		}
		if def.desc.Risk != "warning" {
			t.Fatalf("report.generate risk=%q, want warning", def.desc.Risk)
		}
		return
	}
	t.Fatal("report.generate not registered")
}

// TestGameOpsReportRunSeqPersistsAcrossRestart verifies UI-002's process-
// restart evidence: a fresh store loading the same state file continues the
// report run sequence, so a re-delivered report.generate does not re-emit a
// reportId already handed out before the restart.
func TestGameOpsReportRunSeqPersistsAcrossRestart(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "report-state.json")

	first := newGameOpsStore(statePath)
	run := func(s *gameOpsStore) string {
		raw, err := s.reportGenerate(context.Background(), mustJSON(t, map[string]any{"range": "daily", "delayMs": 0}))
		if err != nil {
			t.Fatalf("report.generate: %v", err)
		}
		return listResult(t, raw)["reportId"].(string)
	}
	firstID := run(first)
	if firstID != "report-demo-1" {
		t.Fatalf("first reportId=%q, want report-demo-1", firstID)
	}

	// Simulate a process restart: a brand-new store loads the same state file.
	second := newGameOpsStore(statePath)
	secondID := run(second)
	if secondID != "report-demo-2" {
		t.Fatalf("post-restart reportId=%q, want report-demo-2 (sequence persisted)", secondID)
	}

	// A third restart again continues the sequence (no reset to 1).
	third := newGameOpsStore(statePath)
	if id := run(third); id != "report-demo-3" {
		t.Fatalf("third reportId=%q, want report-demo-3", id)
	}
}
