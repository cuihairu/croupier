package permission

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) List(ctx context.Context, req *PermissionsListRequest) (*PermissionsListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看权限列表", "admin:all", "roles:read", "role:read", "roles:manage", "role:write"); err != nil {
		return nil, err
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	items, total, err := s.svcCtx.PermissionModel.List(ctx, model.ListPermissionsOptions{
		Page:     page,
		PageSize: pageSize,
		Category: strings.TrimSpace(req.Category),
		Resource: strings.TrimSpace(req.Resource),
	})
	if err != nil {
		return nil, err
	}

	resp := &PermissionsListResponse{
		Items: make([]Permission, 0, len(items)),
		Total: total,
		Page:  page,
		Size:  pageSize,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, buildPermission(item))
	}
	return resp, nil
}

func (s *Service) Detail(ctx context.Context, req *PermissionDetailRequest) (*PermissionDetailResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看权限详情", "admin:all", "roles:read", "role:read", "roles:manage", "role:write"); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		return nil, errorx.NewBadRequest("权限ID不能为空")
	}

	item, err := s.svcCtx.PermissionModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := &PermissionDetailResponse{
		Permission: buildPermission(*item),
	}
	return resp, nil
}

func buildPermission(item model.Permission) Permission {
	return Permission{
		Id:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		Resource:    item.Resource,
		Action:      item.Action,
		Category:    item.Category,
		CreatedAt:   formatTime(item.CreatedAt),
		UpdatedAt:   formatTime(item.UpdatedAt),
	}
}

func formatTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}
