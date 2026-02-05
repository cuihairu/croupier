// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数列表
func NewFunctionsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionsListLogic {
	return &FunctionsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionsListLogic) FunctionsList(req *types.FunctionsListRequest) (*types.FunctionsListResponse, error) {
	// 获取当前用户角色，判断是否为管理员
	_, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	isAdmin := false
	if err == nil && utils.HasAdminRole(ExtractRoleNames(roles)) {
		isAdmin = true
	}

	opts := model.ListFunctionsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Category: strings.TrimSpace(req.Category),
	}

	// 只有非管理员才需要按 GameID 过滤
	if !isAdmin && req.GameId != "" {
		opts.GameID = strings.TrimSpace(req.GameId)
	}

	if req.Status != 0 {
		status := req.Status
		opts.Status = &status
	}

	functions, total, err := l.svcCtx.FunctionModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]types.Function, 0, len(functions))
	for i := range functions {
		items = append(items, utils.BuildFunctionDTO(&functions[i]))
	}

	opts.PaginationOptions.Normalize()

	return &types.FunctionsListResponse{
		Items: items,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}

// ExtractRoleNames 从角色列表中提取角色名称
func ExtractRoleNames(roles []model.Role) []string {
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.Name)
	}
	return names
}
