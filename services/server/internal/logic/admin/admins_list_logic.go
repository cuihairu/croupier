// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取管理员列表
func NewAdminsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminsListLogic {
	return &AdminsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminsListLogic) AdminsList(req *types.AdminsListRequest) (*types.AdminsListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看管理员列表", "admin:all", "user:read", "user:write"); err != nil {
		return nil, err
	}

	opts := model.ListAdminsOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Search:   strings.TrimSpace(req.Search),
		Role:     strings.TrimSpace(req.Role),
	}

	if req.Status != -1 {
		status := req.Status
		opts.Status = &status
	}

	admins, total, err := l.svcCtx.AdminModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	resp := &types.AdminsListResponse{
		Items: make([]types.Admin, 0, len(admins)),
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}

	if len(admins) == 0 {
		return resp, nil
	}

	adminIDs := make([]uint, 0, len(admins))
	for i := range admins {
		adminIDs = append(adminIDs, admins[i].ID)
	}

	roleMap, err := l.loadAdminRoleNames(l.ctx, adminIDs)
	if err != nil {
		return nil, err
	}

	for i := range admins {
		admin := admins[i]
		resp.Items = append(resp.Items, buildAdminResponse(&admin, roleMap[admin.ID]))
	}

	return resp, nil
}

func (l *AdminsListLogic) loadAdminRoleNames(ctx context.Context, adminIDs []uint) (map[uint][]string, error) {
	if len(adminIDs) == 0 {
		return map[uint][]string{}, nil
	}

	type row struct {
		AdminID  uint
		RoleName string
	}

	var rows []row
	if err := l.svcCtx.DB.WithContext(ctx).
		Table("admin_roles").
		Select("admin_roles.admin_id AS admin_id, roles.name AS role_name").
		Joins("INNER JOIN roles ON roles.id = admin_roles.role_id").
		Where("admin_roles.admin_id IN ?", adminIDs).
		Order("admin_roles.admin_id").
		Scan(&rows).Error; err != nil {
		return nil, errorx.NewInternalError("查询管理员角色失败")
	}

	roleMap := make(map[uint][]string, len(adminIDs))
	for _, row := range rows {
		roleMap[row.AdminID] = append(roleMap[row.AdminID], row.RoleName)
	}

	return roleMap, nil
}
