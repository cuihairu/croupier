// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"gorm.io/gorm"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建管理员
func NewAdminCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateLogic {
	return &AdminCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCreateLogic) AdminCreate(req *types.AdminCreateRequest) (resp *types.AdminCreateResponse, err error) {
	// Check permission for creating admin
	permissionService := l.svcCtx.PermissionService
	adminIDValue := l.ctx.Value("adminID")
	adminID, ok := adminIDValue.(uint)
	if !ok || adminID == 0 {
		return nil, errors.New("missing admin context")
	}

	hasPermission, err := permissionService.CheckPermission(l.ctx, adminID, "admin", "create")
	if err != nil {
		l.Errorf("Permission check failed: %v", err)
		return nil, err
	}

	if !hasPermission {
		l.Errorf("Permission denied: admin create")
		return nil, errors.New("permission denied")
	}

	// Create admin
	adminRepo := l.svcCtx.AdminModel
	adminRecord := &model.Admin{
		Username:  req.Username,
		Nickname:  req.Nickname,
		Email:     req.Email,
		Phone:     req.Phone,
		Status:    1, // active by default
		CreatedBy: adminID,
		UpdatedBy: adminID,
	}

	err = adminRepo.Create(l.ctx, adminRecord, req.Password)
	if err != nil {
		l.Errorf("Failed to create admin: %v", err)
		return nil, err
	}

	assignedRoles, err := l.assignRoles(req.Roles, adminRecord.ID)
	if err != nil {
		return nil, err
	}

	// Convert to response type
	resp = &types.AdminCreateResponse{
		Admin: types.Admin{
			Id:        int64(adminRecord.ID),
			Username:  adminRecord.Username,
			Nickname:  adminRecord.Nickname,
			Email:     adminRecord.Email,
			Phone:     adminRecord.Phone,
			Status:    adminRecord.Status,
			CreatedAt: adminRecord.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: adminRecord.UpdatedAt.Format("2006-01-02 15:04:05"),
			Roles:     assignedRoles,
		},
	}

	return resp, nil
}

func (l *AdminCreateLogic) assignRoles(roleNames []string, adminID uint) ([]string, error) {
	if len(roleNames) == 0 {
		return nil, nil
	}

	db := l.svcCtx.DB.WithContext(l.ctx)
	adminRepo := l.svcCtx.AdminModel
	assigned := make([]string, 0, len(roleNames))

	for _, name := range roleNames {
		roleName := strings.TrimSpace(name)
		if roleName == "" {
			continue
		}

		var role model.Role
		if err := db.Where("name = ?", roleName).First(&role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("role %s not found", roleName)
			}
			return nil, fmt.Errorf("failed to load role %s: %w", roleName, err)
		}

		if err := adminRepo.AssignRole(l.ctx, adminID, role.ID); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				assigned = append(assigned, role.Name)
				continue
			}
			return nil, fmt.Errorf("failed to assign role %s: %w", roleName, err)
		}

		assigned = append(assigned, role.Name)
	}

	return assigned, nil
}
