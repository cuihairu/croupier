package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
)

func migrateHistoryPath(ctx *svc.ServiceContext) string {
	base := "data"
	if ctx != nil {
		if b := strings.TrimSpace(ctx.Config.BootstrapData.BaseDir); b != "" {
			base = b
		}
	}
	return filepath.Join(base, "migrate_history.json")
}

func loadMigrateHistory(path string) ([]MigrationResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []MigrationResult{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return []MigrationResult{}, nil
	}
	var out []MigrationResult
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []MigrationResult{}, nil
	}
	return out, nil
}

func saveMigrateHistory(path string, items []MigrationResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func appendMigrateHistory(ctx *svc.ServiceContext, results []MigrationResult) error {
	path := migrateHistoryPath(ctx)
	items, err := loadMigrateHistory(path)
	if err != nil {
		return err
	}
	items = append(results, items...)
	if len(items) > 500 {
		items = items[:500]
	}
	return saveMigrateHistory(path, items)
}

func nowDurationMS(start time.Time) string {
	return time.Since(start).Round(time.Millisecond).String()
}
