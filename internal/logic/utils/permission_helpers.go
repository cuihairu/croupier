package utils

import (
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
)

// BuildPermission maps model.Permission to API type.
func BuildPermission(perm *model.Permission) Permission {
	return Permission{
		Id:          perm.ID,
		Name:        perm.Name,
		Description: perm.Description,
		Resource:    perm.Resource,
		Action:      perm.Action,
		Category:    perm.Category,
		CreatedAt:   helper.FormatTimestamp(perm.CreatedAt),
		UpdatedAt:   helper.FormatTimestamp(perm.UpdatedAt),
	}
}

// ValidatePermissionID ensures ID is not empty.
func ValidatePermissionID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", errorx.NewBadRequest("权限ID不能为空")
	}
	return trimmed, nil
}

// Local types for backward compatibility
type Permission struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Category    string `json:"category"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}
