package utils

import (
	"encoding/json"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/model"
)

// BuildEntityDTO converts entity model to API payload.
func BuildEntityDTO(entity *model.Entity) map[string]interface{} {
	var payload interface{}
	if entity != nil && len(entity.Data) > 0 {
		if err := json.Unmarshal(entity.Data, &payload); err != nil {
			payload = string(entity.Data)
		}
	}
	return map[string]interface{}{
		"id":         entity.ID,
		"type":       entity.Type,
		"data":       payload,
		"providerId": entity.ProviderID,
		"status":     entity.Status,
		"createdAt":  FormatTimestamp(entity.CreatedAt),
		"updatedAt":  FormatTimestamp(entity.UpdatedAt),
	}
}

// ValidateEntityType checks entity type string.
func ValidateEntityType(entityType string) (string, error) {
	if entityType == "" {
		return "", fmt.Errorf("实体类型不能为空")
	}
	return entityType, nil
}
