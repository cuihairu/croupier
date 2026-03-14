package utils

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/config"
)

// ResolveAnalyticsFiltersPath returns an absolute path to the filters document.
func ResolveAnalyticsFiltersPath(cfg config.Config) string {
	raw := strings.TrimSpace(cfg.Registry.AnalyticsFiltersPath)
	if raw == "" {
		if dir := strings.TrimSpace(cfg.Schemas.Dir); dir != "" {
			raw = filepath.Join(dir, "analytics_filters.json")
		} else if dir := strings.TrimSpace(cfg.Packs.Dir); dir != "" {
			raw = filepath.Join(dir, "ui", "analytics_filters.json")
		} else {
			raw = "analytics_filters.json"
		}
	}
	if !filepath.IsAbs(raw) {
		if abs, err := filepath.Abs(raw); err == nil {
			raw = abs
		}
	}
	return filepath.Clean(raw)
}

// ReadAnalyticsFiltersFile reads the filters file content
func ReadAnalyticsFiltersFile(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return []byte{}, nil
	}
	return os.ReadFile(path)
}

// WriteAnalyticsFiltersFile writes the filters file content
func WriteAnalyticsFiltersFile(path string, data []byte) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
