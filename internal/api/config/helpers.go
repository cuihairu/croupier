package config

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"gopkg.in/yaml.v3"
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
		"gameId":    v.GameID,
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
		"id":             v.Key,
		"format":         v.Format,
		"gameId":         v.GameID,
		"env":            v.Env,
		"latestVersion":  v.Version,
		"updatedAt":      utils.FormatTimestamp(v.UpdatedAt),
		"lastMessage":    v.Message,
		"lastModifiedBy": v.CreatedBy,
	}
}

func validateConfigContent(format, content string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "json":
		var out interface{}
		return json.Unmarshal([]byte(content), &out)
	case "yaml", "yml":
		var out interface{}
		return yaml.Unmarshal([]byte(content), &out)
	case "xml":
		var out interface{}
		return xml.Unmarshal([]byte(content), &out)
	case "ini":
		if strings.TrimSpace(content) == "" {
			return fmt.Errorf("ini content cannot be empty")
		}
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
				continue
			}
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				continue
			}
			if strings.Contains(trimmed, "=") || strings.Contains(trimmed, ":") {
				continue
			}
			return fmt.Errorf("invalid ini line: %s", trimmed)
		}
		return nil
	case "csv":
		if strings.TrimSpace(content) == "" {
			return fmt.Errorf("csv content cannot be empty")
		}
		return nil
	default:
		return fmt.Errorf("unsupported config format: %s", format)
	}
}
