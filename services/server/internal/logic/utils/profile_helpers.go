package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
)

// CurrentUsername extracts the authenticated username from context.
func CurrentUsername(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("请求上下文不存在")
	}
	username, _ := ctx.Value("username").(string)
	username = strings.TrimSpace(username)
	if username == "" {
		return "", errors.New("未找到登录用户")
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

	admin, err := svcCtx.AdminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, nil, fmt.Errorf("查询管理员失败: %w", err)
	}

	roles, err := svcCtx.AdminModel.GetAdminRoles(ctx, admin.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("查询管理员角色失败: %w", err)
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
