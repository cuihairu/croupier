package utils

import (
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
)

// NewPermissionDenied builds a stable 403 permission_denied error with
// machine-readable details (required permissions / resource scope) so the
// renderer can surface the concrete reason instead of only the HTTP status.
func NewPermissionDenied(message string, details map[string]any) *errorx.CodeError {
	return errorx.NewForbiddenWithCode("permission_denied", message, details)
}

// requiredPermissions builds the shared required-permission detail payload.
// 超管通配（admin:all / *）是既有授权模型的隐式兜底，不是普通运营可申请的具体
// 权限，若列入 required 会误导渲染端说「需要 admin:all」，故过滤；其余按冒号
// 权限码约定保留，供前端透出「缺少权限：<specific>」。
func requiredPermissions(required ...string) map[string]any {
	values := make([]string, 0, len(required))
	seen := make(map[string]struct{}, len(required))
	for _, value := range required {
		trimmed := strings.TrimSpace(value)
		key := strings.ToLower(trimmed)
		if key == "" || key == "admin:all" || key == "*" {
			continue
		}
		if !strings.Contains(key, ":") {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, trimmed)
	}
	if len(values) == 0 {
		return nil
	}
	return map[string]any{"required": values}
}
