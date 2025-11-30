package config

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
)

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

func mapConfigVersion(v *model.ConfigVersion, includeValue bool) map[string]interface{} {
	if v == nil {
		return nil
	}
	data := map[string]interface{}{
		"key":       v.Key,
		"version":   v.Version,
		"createdBy": v.CreatedBy,
		"createdAt": utils.FormatTimestamp(v.CreatedAt),
	}
	if includeValue {
		data["value"] = v.Value
	}
	return data
}
