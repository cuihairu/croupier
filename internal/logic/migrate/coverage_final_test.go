package migrate

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// saveMigrateHistory 的 json.MarshalIndent 错误分支（helpers.go:49-51）不可达：
// MigrationResult 只含 string 字段，任何 []MigrationResult 都能被序列化。
// 此测试固化该边界：大批量、超长字符串场景下序列化依然成功。
func TestSaveMigrateHistoryLargePayloadNeverFailsMarshal(t *testing.T) {
	dir := t.TempDir()
	items := make([]MigrationResult, 0, 600)
	for i := 0; i < 600; i++ {
		items = append(items, MigrationResult{
			Name:      "migration-with-a-very-long-name-" + string(rune('a'+i%26)) + "-suffix",
			Version:   "20260906120000",
			StartTime: "2026-09-06T12:00:00Z",
			EndTime:   "2026-09-06T12:00:01Z",
			Status:    "applied",
			Error:     "",
			AppliedBy: "coverage-runner",
		})
	}
	path := filepath.Join(dir, "migrate_history.json")
	if err := saveMigrateHistory(path, items); err != nil {
		t.Fatalf("saveMigrateHistory large payload: %v", err)
	}

	loaded, err := loadMigrateHistory(path)
	if err != nil {
		t.Fatalf("loadMigrateHistory: %v", err)
	}
	if len(loaded) != 600 {
		t.Fatalf("loaded %d items, want 600", len(loaded))
	}
	var raw []MigrationResult
	blob, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal reference: %v", err)
	}
	if err := json.Unmarshal(blob, &raw); err != nil {
		t.Fatalf("unmarshal reference: %v", err)
	}
}
