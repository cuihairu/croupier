package admin

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"gorm.io/gorm"
)

func parseAdminID(id string) (uint, error) {
	if strings.TrimSpace(id) == "" {
		return 0, fmt.Errorf("管理员ID不能为空")
	}

	value, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的管理员ID: %w", err)
	}

	if value == 0 {
		return 0, fmt.Errorf("管理员ID必须大于0")
	}

	return uint(value), nil
}

func buildAdminResponse(admin *model.Admin, roleNames []string) types.Admin {
	return types.Admin{
		Id:        int64(admin.ID),
		Username:  admin.Username,
		Nickname:  admin.Nickname,
		Email:     admin.Email,
		Phone:     admin.Phone,
		Roles:     roleNames,
		Status:    admin.Status,
		CreatedAt: formatTimestamp(admin.CreatedAt),
		UpdatedAt: formatTimestamp(admin.UpdatedAt),
	}
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func roleNamesFromModels(roles []model.Role) []string {
	if len(roles) == 0 {
		return nil
	}

	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}

func fetchRolesByNames(ctx context.Context, db *gorm.DB, names []string) ([]model.Role, error) {
	ordered, lowered := uniqueRoleInputs(names)
	if len(ordered) == 0 {
		return nil, nil
	}

	var roles []model.Role
	if err := db.WithContext(ctx).
		Where("LOWER(name) IN ?", lowered).
		Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("查询角色失败: %w", err)
	}

	if len(roles) == 0 {
		return nil, fmt.Errorf("角色不存在: %s", strings.Join(ordered, ", "))
	}

	found := make(map[string]model.Role, len(roles))
	for _, role := range roles {
		found[strings.ToLower(role.Name)] = role
	}

	var missing []string
	for _, name := range ordered {
		if _, ok := found[strings.ToLower(name)]; !ok {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("角色不存在: %s", strings.Join(missing, ", "))
	}

	orderedRoles := make([]model.Role, 0, len(ordered))
	for _, name := range ordered {
		orderedRoles = append(orderedRoles, found[strings.ToLower(name)])
	}
	return orderedRoles, nil
}

func uniqueRoleInputs(names []string) ([]string, []string) {
	if len(names) == 0 {
		return nil, nil
	}

	ordered := make([]string, 0, len(names))
	lowered := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, raw := range names {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		ordered = append(ordered, trimmed)
		lowered = append(lowered, key)
	}

	return ordered, lowered
}
