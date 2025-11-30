package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type analyticsFiltersDocument struct {
	Items     []types.AnalyticsFilters `json:"items"`
	UpdatedAt time.Time                `json:"updated_at,omitempty"`
}

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

// LoadAnalyticsFilters reads the stored filters and normalizes the result.
func LoadAnalyticsFilters(path string) ([]types.AnalyticsFilters, error) {
	if strings.TrimSpace(path) == "" {
		return []types.AnalyticsFilters{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []types.AnalyticsFilters{}, nil
		}
		return nil, err
	}
	var doc analyticsFiltersDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return normalizeAnalyticsFilters(doc.Items), nil
}

// SaveAnalyticsFilters writes the provided filters back to disk.
func SaveAnalyticsFilters(path string, items []types.AnalyticsFilters) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	doc := analyticsFiltersDocument{
		Items:     normalizeAnalyticsFilters(items),
		UpdatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func normalizeAnalyticsFilters(items []types.AnalyticsFilters) []types.AnalyticsFilters {
	normalized := make([]types.AnalyticsFilters, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		gameID := strings.TrimSpace(item.GameId)
		if gameID == "" {
			continue
		}
		if _, ok := seen[gameID]; ok {
			continue
		}
		normalized = append(normalized, types.AnalyticsFilters{
			GameId:  gameID,
			Filters: item.Filters,
		})
		seen[gameID] = struct{}{}
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].GameId < normalized[j].GameId
	})
	return normalized
}
