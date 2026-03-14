package role

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"

	"gorm.io/gorm"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// RoleCreate creates a new role with the provided permissions.
func (s *Service) RoleCreate(ctx context.Context, req *RoleCreateRequest) (*RoleCreateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权创建角色", "admin:all", "roles:manage", "role:write"); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("角色名称不能为空")
	}

	permissionIDs, err := s.ensurePermissionIDs(ctx, req.Permissions)
	if err != nil {
		return nil, err
	}

	var createdRole *model.Role
	err = s.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		role := &model.Role{
			Name:        name,
			Description: strings.TrimSpace(req.Description),
			Category:    strings.TrimSpace(req.Category),
		}
		roleModel := model.NewRoleModel(tx)
		if err := roleModel.Create(ctx, role); err != nil {
			return err
		}
		if err := roleModel.ReplacePermissions(ctx, role.ID, permissionIDs); err != nil {
			return err
		}
		createdRole = role
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &RoleCreateResponse{
		Role: s.buildRole(createdRole, permissionIDs),
	}, nil
}

// RoleDelete deletes a role by ID.
func (s *Service) RoleDelete(ctx context.Context, req *RoleDeleteRequest) error {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权删除角色", "admin:all", "roles:manage", "role:write"); err != nil {
		return err
	}

	roleID, err := s.parseRoleID(req.ID)
	if err != nil {
		return err
	}

	if err := s.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		roleModel := model.NewRoleModel(tx)
		if _, err := roleModel.FindOne(ctx, roleID); err != nil {
			return err
		}

		if err := tx.WithContext(ctx).
			Where("role_id = ?", roleID).
			Delete(&model.RolePermission{}).Error; err != nil {
			return errorx.NewInternalError("删除角色权限失败")
		}

		return roleModel.Delete(ctx, roleID)
	}); err != nil {
		return err
	}

	s.svcCtx.InvalidateRoleCache(ctx, roleID)
	return nil
}

// RoleDetail retrieves the details of a role by ID.
func (s *Service) RoleDetail(ctx context.Context, req *RoleDetailRequest) (*RoleDetailResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看角色", "admin:all", "roles:read", "role:read", "roles:manage", "role:write"); err != nil {
		return nil, err
	}

	roleID, err := s.parseRoleID(req.ID)
	if err != nil {
		return nil, err
	}

	role, err := s.svcCtx.GetRoleCached(ctx, roleID)
	if err != nil {
		return nil, err
	}

	permissions, err := s.svcCtx.GetRolePermissionIDsCached(ctx, role.ID)
	if err != nil {
		return nil, err
	}

	return &RoleDetailResponse{
		Role: s.buildRole(role, permissions),
	}, nil
}

// RoleUpdate updates a role's fields and optionally its permissions.
func (s *Service) RoleUpdate(ctx context.Context, req *RoleUpdateRequest) (*RoleUpdateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权更新角色", "admin:all", "roles:manage", "role:write"); err != nil {
		return nil, err
	}

	roleID, err := s.parseRoleID(req.ID)
	if err != nil {
		return nil, err
	}

	permissionIDs := req.Permissions
	updatePermissions := permissionIDs != nil
	var normalizedPermissions []string
	if updatePermissions {
		normalizedPermissions, err = s.ensurePermissionIDs(ctx, permissionIDs)
		if err != nil {
			return nil, err
		}
	}

	err = s.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		roleModel := model.NewRoleModel(tx)
		if _, err := roleModel.FindOne(ctx, roleID); err != nil {
			return err
		}

		updates := make(map[string]interface{})
		if name := strings.TrimSpace(req.Name); name != "" {
			updates["name"] = name
		}
		if desc := strings.TrimSpace(req.Description); desc != "" {
			updates["description"] = desc
		}
		if category := strings.TrimSpace(req.Category); category != "" {
			updates["category"] = category
		}

		if err := roleModel.Update(ctx, roleID, updates); err != nil {
			return err
		}

		if updatePermissions {
			if err := roleModel.ReplacePermissions(ctx, roleID, normalizedPermissions); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.svcCtx.InvalidateRoleCache(ctx, roleID)

	role, err := s.svcCtx.GetRoleCached(ctx, roleID)
	if err != nil {
		return nil, err
	}

	perms := normalizedPermissions
	if !updatePermissions {
		perms, err = s.svcCtx.GetRolePermissionIDsCached(ctx, roleID)
		if err != nil {
			return nil, err
		}
	}

	return &RoleUpdateResponse{
		Role: s.buildRole(role, perms),
	}, nil
}

// RolesList retrieves a paginated list of roles with optional filtering.
func (s *Service) RolesList(ctx context.Context, req *RolesListRequest) (*RolesListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看角色列表", "admin:all", "roles:read", "role:read", "roles:manage", "role:write"); err != nil {
		return nil, err
	}

	// Apply defaults
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	opts := model.ListRolesOptions{
		Page:     page,
		PageSize: pageSize,
		Category: strings.TrimSpace(req.Category),
		Search:   strings.TrimSpace(req.Search),
	}

	roles, total, err := s.svcCtx.RoleModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	resp := &RolesListResponse{
		Items: make([]Role, 0, len(roles)),
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}

	if len(roles) == 0 {
		return resp, nil
	}

	roleIDs := make([]uint, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
	}

	permMap, err := s.svcCtx.RoleModel.GetRolesPermissionIDs(ctx, roleIDs)
	if err != nil {
		return nil, err
	}

	for i := range roles {
		role := roles[i]
		resp.Items = append(resp.Items, s.buildRole(&role, permMap[role.ID]))
	}

	return resp, nil
}

// Helper methods

func (s *Service) parseRoleID(id string) (uint, error) {
	if strings.TrimSpace(id) == "" {
		return 0, errorx.NewBadRequest("角色ID不能为空")
	}
	value, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, errorx.NewBadRequest("无效的角色ID")
	}
	if value == 0 {
		return 0, errorx.NewBadRequest("角色ID必须大于0")
	}
	return uint(value), nil
}

func (s *Service) ensurePermissionIDs(ctx context.Context, permissionIDs []string) ([]string, error) {
	if s.svcCtx.RoleModel == nil {
		return nil, errorx.NewInternalError("role model is not initialized")
	}
	return s.svcCtx.RoleModel.ValidatePermissionIDs(ctx, permissionIDs)
}

func (s *Service) buildRole(role *model.Role, permissionIDs []string) Role {
	return Role{
		Id:          int64(role.ID),
		Name:        role.Name,
		Description: role.Description,
		Category:    role.Category,
		Permissions: permissionIDs,
		CreatedAt:   formatTimestamp(role.CreatedAt),
		UpdatedAt:   formatTimestamp(role.UpdatedAt),
	}
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
