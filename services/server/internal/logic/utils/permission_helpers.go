package utils

import (
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

// BuildPermission maps model.Permission to API type.
func BuildPermission(perm *model.Permission) types.Permission {
	return types.Permission{
		Id:          perm.ID,
		Name:        perm.Name,
		Description: perm.Description,
		Resource:    perm.Resource,
		Action:      perm.Action,
		Category:    perm.Category,
		CreatedAt:   FormatTimestamp(perm.CreatedAt),
		UpdatedAt:   FormatTimestamp(perm.UpdatedAt),
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
