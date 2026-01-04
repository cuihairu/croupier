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

type FunctionCategoriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数分类列表
func NewFunctionCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionCategoriesLogic {
	return &FunctionCategoriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionCategoriesLogic) FunctionCategories(_ *types.FunctionCategoriesRequest) (*types.FunctionCategoriesResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看函数分类", "admin:all", "functions:read", "functions:list"); err != nil {
		return nil, err
	}

	// Query distinct categories from functions table
	var categories []string
	err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Function{}).
		Select("DISTINCT category").
		Where("category != ''").
		Pluck("category", &categories).
		Error

	if err != nil {
		return nil, err
	}

	// Also include categories from descriptors
	var descriptorCategories []string
	l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Descriptor{}).
		Select("DISTINCT category").
		Where("category != ''").
		Pluck("category", &descriptorCategories)

	// Merge and deduplicate
	categoryMap := make(map[string]int)
	for _, cat := range categories {
		if cat = strings.TrimSpace(cat); cat != "" {
			categoryMap[cat]++
		}
	}
	for _, cat := range descriptorCategories {
		if cat = strings.TrimSpace(cat); cat != "" {
			categoryMap[cat]++
		}
	}

	// Build response with counts
	items := make([]types.FunctionCategoryItem, 0, len(categoryMap))
	for cat, count := range categoryMap {
		items = append(items, types.FunctionCategoryItem{
			Category: cat,
			Count:    count,
		})
	}

	return &types.FunctionCategoriesResponse{
		Categories: items,
	}, nil
}

type FunctionSearchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 高级搜索函数
func NewFunctionSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionSearchLogic {
	return &FunctionSearchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionSearchLogic) FunctionSearch(req *types.FunctionSearchRequest) (*types.FunctionSearchResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权搜索函数", "admin:all", "functions:read", "functions:search"); err != nil {
		return nil, err
	}

	opts := model.ListFunctionsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		GameID:   strings.TrimSpace(req.GameID),
		Category: strings.TrimSpace(req.Category),
		Search:   strings.TrimSpace(req.Query),
	}

	// Handle multiple statuses
	if len(req.Statuses) > 0 {
		// Use first status for now (could be extended to OR multiple statuses)
		status := req.Statuses[0]
		opts.Status = &status
	} else if req.Status != 0 {
		status := req.Status
		opts.Status = &status
	}

	// Handle tags (would need to query metadata JSON field)
	// For now, we'll use basic search

	functions, total, err := l.svcCtx.FunctionModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]types.Function, 0, len(functions))
	for i := range functions {
		items = append(items, utils.BuildFunctionDTO(&functions[i]))
	}

	opts.PaginationOptions.Normalize()

	return &types.FunctionSearchResponse{
		Items: items,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}
