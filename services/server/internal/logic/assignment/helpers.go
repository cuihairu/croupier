package assignment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
)

func assignmentsPath(ctx *svc.ServiceContext) string {
	if ctx == nil {
		return filepath.Join("data", "assignments.json")
	}
	path := strings.TrimSpace(ctx.Config.Registry.AssignmentsPath)
	if path == "" {
		path = filepath.Join("data", "assignments.json")
	}
	if !filepath.IsAbs(path) {
		if base := strings.TrimSpace(ctx.Config.BootstrapData.BaseDir); base != "" && filepath.IsAbs(base) {
			path = filepath.Join(base, path)
		}
	}
	return filepath.Clean(path)
}

func loadAssignments(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]string{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string][]string{}, nil
	}
	var assignments map[string][]string
	if err := json.Unmarshal(data, &assignments); err != nil {
		return nil, err
	}
	if assignments == nil {
		assignments = map[string][]string{}
	}
	return assignments, nil
}

func saveAssignments(path string, data map[string][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0o644)
}

func filterAssignments(data map[string][]string, gameID, env string) map[string][]string {
	if gameID == "" && env == "" {
		return cloneAssignments(data)
	}
	filtered := make(map[string][]string)
	for key, functions := range data {
		g, e := splitAssignmentKey(key)
		if gameID != "" && !strings.EqualFold(g, gameID) {
			continue
		}
		if env != "" && !strings.EqualFold(e, env) {
			continue
		}
		filtered[key] = append([]string(nil), functions...)
	}
	return filtered
}

func splitAssignmentKey(key string) (gameID, env string) {
	parts := strings.SplitN(key, "|", 2)
	gameID = parts[0]
	if len(parts) > 1 {
		env = parts[1]
	}
	return strings.TrimSpace(gameID), strings.TrimSpace(env)
}

func buildAssignmentKey(gameID, env string) string {
	return fmt.Sprintf("%s|%s", strings.TrimSpace(gameID), strings.TrimSpace(env))
}

func cloneAssignments(data map[string][]string) map[string][]string {
	out := make(map[string][]string, len(data))
	for key, functions := range data {
		out[key] = append([]string(nil), functions...)
	}
	return out
}

// LoadAllAssignments returns all assignment mappings for the service context.
func LoadAllAssignments(ctx *svc.ServiceContext) (map[string][]string, error) {
	path := assignmentsPath(ctx)
	return loadAssignments(path)
}

func normalizeFunctions(functions []string) []string {
	seen := make(map[string]struct{}, len(functions))
	result := make([]string, 0, len(functions))
	for _, fn := range functions {
		trimmed := strings.TrimSpace(fn)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func splitKnownAndUnknown(functions []string, known map[string]struct{}) (accepted, unknown []string) {
	if len(known) == 0 {
		return append([]string(nil), functions...), nil
	}
	for _, fn := range functions {
		if _, ok := known[fn]; ok {
			accepted = append(accepted, fn)
		} else {
			unknown = append(unknown, fn)
		}
	}
	if len(unknown) > 1 {
		sort.Strings(unknown)
	}
	return
}
