package utils

import (
	"encoding/json"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/model"
)

// BuildMessageDTO converts model.Message into a serializable map.
func BuildMessageDTO(msg *model.Message) map[string]interface{} {
	var payload interface{}
	if msg.Data != nil && len(msg.Data) > 0 {
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			payload = string(msg.Data)
		}
	}
	return map[string]interface{}{
		"id":        msg.ID,
		"to":        msg.To,
		"type":      msg.Type,
		"title":     msg.Title,
		"content":   msg.Content,
		"data":      payload,
		"status":    msg.Status,
		"readAt":    FormatTimestampPtr(msg.ReadAt),
		"createdAt": FormatTimestamp(msg.CreatedAt),
		"updatedAt": FormatTimestamp(msg.UpdatedAt),
	}
}

// ValidateMessageType optionally ensures type is present.
func ValidateMessageType(messageType string) (string, error) {
	if messageType == "" {
		return "", fmt.Errorf("消息类型不能为空")
	}
	return messageType, nil
}
