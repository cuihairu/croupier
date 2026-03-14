package config

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
)

// configAuthor extracts the author from context
func configAuthor(ctx context.Context) string {
	if ctx == nil {
		return "system"
	}
	if username, ok := ctx.Value("username").(string); ok {
		if trimmed := strings.TrimSpace(username); trimmed != "" {
			return trimmed
		}
	}
	return "system"
}

// mapConfigVersion converts a ConfigVersion model to a map representation
func mapConfigVersion(v *model.ConfigVersion, includeValue bool) map[string]interface{} {
	if v == nil {
		return nil
	}
	data := map[string]interface{}{
		"key":       v.Key,
		"version":   v.Version,
		"createdBy": v.CreatedBy,
		"createdAt": utils.FormatTimestamp(v.CreatedAt),
		"game_id":   v.GameID,
		"env":       v.Env,
		"format":    v.Format,
		"message":   v.Message,
	}
	if includeValue {
		data["value"] = v.Value
	}
	return data
}

// mapConfigItem converts a ConfigVersion model to a simplified config item map
func mapConfigItem(v *model.ConfigVersion) map[string]interface{} {
	if v == nil {
		return nil
	}
	return map[string]interface{}{
		"id":              v.Key,
		"format":          v.Format,
		"game_id":         v.GameID,
		"env":             v.Env,
		"latest_version":  v.Version,
		"updated_at":      utils.FormatTimestamp(v.UpdatedAt),
		"last_message":    v.Message,
		"last_modifiedBy": v.CreatedBy,
	}
}
