package utils

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

var ErrCurrentUserNotFound = errors.New("未找到登录用户")

// CurrentUsername extracts the authenticated username from context.
func CurrentUsername(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("请求上下文不存在")
	}
	username, _ := ctx.Value("username").(string)
	username = strings.TrimSpace(username)
	if username == "" {
		return "", ErrCurrentUserNotFound
	}
	return username, nil
}

// LoadCurrentAdmin fetches the admin record + roles for the current request.
func LoadCurrentAdmin(ctx context.Context, svcCtx *svc.ServiceContext) (*model.Admin, []model.Role, error) {
	if svcCtx == nil || svcCtx.AdminModel == nil {
		return nil, nil, errors.New("管理员模型未初始化")
	}

	username, err := CurrentUsername(ctx)
	if err != nil {
		return nil, nil, err
	}

	// 使用缓存查询管理员信息
	admin, err := svcCtx.GetAdminByUsernameCached(ctx, username)
	if err != nil {
		return nil, nil, errorx.NewInternalError("查询管理员失败")
	}
	if admin == nil {
		return nil, nil, errorx.NewUnauthorized("登录用户不存在")
	}

	// 使用缓存查询角色信息
	roles, err := svcCtx.GetAdminRolesCached(ctx, admin.ID)
	if err != nil {
		return nil, nil, errorx.NewInternalError("查询管理员角色失败")
	}
	return admin, roles, nil
}

// DecodeStringSlice decodes a JSON array into a slice of strings.
func DecodeStringSlice(data datatypes.JSON) []string {
	if len(data) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return nil
	}
	return values
}

// EncodeStringSlice encodes a string slice into JSON for persistence.
func EncodeStringSlice(values []string) datatypes.JSON {
	bytes, _ := json.Marshal(values)
	return datatypes.JSON(bytes)
}

// HasAdminRole reports whether the provided role names contain an admin-level role.
func HasAdminRole(roles []string) bool {
	for _, role := range roles {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "admin", "super_admin":
			return true
		}
	}
	return false
}

// EnforcePermission checks if the user has the required permission.
// Admin and super_admin roles bypass all permission checks.
// Returns nil if allowed, error if denied.
func EnforcePermission(roleNames []string, check func() bool) error {
	// Admin role bypasses all permission checks
	if HasAdminRole(roleNames) {
		return nil
	}
	if check() {
		return nil
	}
	return errorx.NewForbidden("权限不足")
}

// EnforceFunctionInvokePermission checks if user can invoke a function.
// Admin role always allowed. Otherwise checks allowedRoles.
func EnforceFunctionInvokePermission(roleNames []string, allowedRoles []string) error {
	return EnforcePermission(roleNames, func() bool {
		if len(allowedRoles) == 0 {
			return true // No restriction
		}
		for _, allowed := range allowedRoles {
			for _, role := range roleNames {
				if strings.EqualFold(strings.TrimSpace(role), strings.TrimSpace(allowed)) {
					return true
				}
			}
		}
		return false
	})
}
