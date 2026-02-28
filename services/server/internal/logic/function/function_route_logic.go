package function

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type FunctionRouteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionRouteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionRouteLogic {
	return &FunctionRouteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionRouteLogic) FunctionRoute(req *types.FunctionRouteRequest) (*types.FunctionRouteResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cfg := normalizeMenuConfig(defaultMenu())
			return &types.FunctionRouteResponse{
				Menu:   cfg,
				Source: "default",
			}, nil
		}
		return nil, err
	}

	menu := defaultMenu()
	source := "default"
	if fn.Metadata != nil {
		if mm, ok := fn.Metadata["menu"].(map[string]interface{}); ok && mm != nil {
			mergeShallow(menu, mm)
			source = "metadata"
		}
	}
	return &types.FunctionRouteResponse{
		Menu:   normalizeMenuConfig(menu),
		Source: source,
	}, nil
}

func normalizeMenuConfig(menu map[string]interface{}) types.FunctionRouteConfig {
	cfg := types.FunctionRouteConfig{
		Section: "",
		Group:   "",
		Path:    "",
		Order:   100,
		Hidden:  false,
	}
	if menu == nil {
		return cfg
	}
	if v, ok := menu["section"].(string); ok {
		cfg.Section = v
	}
	if v, ok := menu["group"].(string); ok {
		cfg.Group = v
	}
	if v, ok := menu["path"].(string); ok {
		cfg.Path = v
	}
	if v, ok := menu["order"].(int); ok {
		cfg.Order = v
	}
	if v, ok := menu["order"].(float64); ok {
		cfg.Order = int(v)
	}
	if v, ok := menu["hidden"].(bool); ok {
		cfg.Hidden = v
	}
	return cfg
}
