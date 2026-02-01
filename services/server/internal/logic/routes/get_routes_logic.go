// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package routes

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRoutesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetRoutesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoutesLogic {
	return &GetRoutesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRoutesLogic) GetRoutes() (*types.GetRoutesResponse, error) {
	// 获取所有已启用的函数
	status := 1
	functions, _, err := l.svcCtx.FunctionModel.List(l.ctx, model.ListFunctionsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: 1000,
		},
		Status: &status,
	})
	if err != nil {
		return nil, err
	}

	// 按对象分组函数
	grouped := make(map[string][]model.Function)
	for _, fn := range functions {
		objectName := l.extractObjectName(fn.FunctionID)
		grouped[objectName] = append(grouped[objectName], fn)
	}

	// 生成路由配置
	routes := make([]types.RouteItem, 0, len(grouped))
	for objectName, funcs := range grouped {
		routes = append(routes, l.buildRoute(objectName, funcs))
	}

	return &types.GetRoutesResponse{
		Code:    0,
		Message: "OK",
		Data:    routes,
	}, nil
}

func (l *GetRoutesLogic) extractObjectName(functionID string) string {
	parts := strings.Split(functionID, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return "other"
}

func (l *GetRoutesLogic) buildRoute(objectName string, functions []model.Function) types.RouteItem {
	subRoutes := make([]types.RouteItem, 0, len(functions))
	for _, fn := range functions {
		actionName := l.getActionName(fn.FunctionID)
		subRoutes = append(subRoutes, types.RouteItem{
			Path:      "/functions/" + objectName + "/" + actionName,
			Name:      l.getDisplayName(fn),
			Component: "../pages/Functions/DynamicInvoker",
			Meta: map[string]interface{}{
				"functionId":   fn.FunctionID,
				"functionName": actionName,
				"category":     fn.Category,
			},
		})
	}

	return types.RouteItem{
		Path:   "/functions/" + objectName,
		Name:   strings.ToUpper(objectName[:1]) + objectName[1:] + "Functions",
		Icon:   l.getIcon(objectName),
		Routes: subRoutes,
	}
}

func (l *GetRoutesLogic) getActionName(functionID string) string {
	parts := strings.Split(functionID, ".")
	if len(parts) > 1 {
		return strings.Join(parts[1:], ".")
	}
	return functionID
}

func (l *GetRoutesLogic) getDisplayName(fn model.Function) string {
	if fn.Name != "" {
		return fn.Name
	}
	return fn.FunctionID
}

func (l *GetRoutesLogic) getIcon(objectName string) string {
	icons := map[string]string{
		"player": "user", "item": "inbox", "quest": "file-text",
		"guild": "team", "mail": "mail", "shop": "shopping-cart",
		"battle": "thunderbolt", "chat": "message", "ranking": "trophy",
		"activity": "calendar", "system": "setting",
	}
	if icon, ok := icons[objectName]; ok {
		return icon
	}
	return "api"
}
