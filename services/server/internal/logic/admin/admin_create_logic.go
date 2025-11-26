// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"

	"github.com/cuihairu/croupier/internal/repo/gorm/users"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

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
	adminID := l.ctx.Value("adminID").(uint)

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
	adminRepo := l.svcCtx.AdminRepository
	adminRecord := &users.AdminRecord{
		Username:    req.Username,
		DisplayName: req.Nickname,
		Email:       req.Email,
		Phone:       req.Phone,
		Status:      1, // active by default
		CreatedBy:   adminID,
		UpdatedBy:   adminID,
	}

	err = adminRepo.CreateAdmin(l.ctx, adminRecord, req.Password)
	if err != nil {
		l.Errorf("Failed to create admin: %v", err)
		return nil, err
	}

	// Assign roles if provided
	for _, roleName := range req.Roles {
		// TODO: Get role by name and assign
		l.Infof("Role assignment not implemented yet: %s", roleName)
	}

	// Convert to response type
	resp = &types.AdminCreateResponse{
		Admin: types.Admin{
			Id:        int64(adminRecord.ID),
			Username:  adminRecord.Username,
			Nickname:  adminRecord.DisplayName,
			Email:     adminRecord.Email,
			Phone:     adminRecord.Phone,
			Status:    adminRecord.Status,
			CreatedAt: adminRecord.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: adminRecord.UpdatedAt.Format("2006-01-02 15:04:05"),
		},
	}

	return resp, nil
}