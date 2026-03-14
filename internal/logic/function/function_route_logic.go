package function

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"

	"gorm.io/gorm"
)

type FunctionRouteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionRouteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionRouteLogic {
	return &FunctionRouteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionRouteLogic) FunctionRoute(req *FunctionRouteRequest) (*FunctionRouteResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			base := defaultMenu()
			applyEntityMenuDefaults(base, "", "", functionID)
			cfg := normalizeMenuConfig(base)
			return &FunctionRouteResponse{
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
	applyEntityMenuDefaults(menu, "", "", functionID)
	return &FunctionRouteResponse{
		Menu:   normalizeMenuConfig(menu),
		Source: source,
	}, nil
}

func normalizeMenuConfig(menu map[string]interface{}) FunctionRouteConfig {
	cfg := FunctionRouteConfig{
		Nodes:  []string{},
		Path:   "",
		Order:  100,
		Hidden: false,
	}
	if menu == nil {
		return cfg
	}
	if nodes, ok := menu["nodes"].([]string); ok {
		cfg.Nodes = append(cfg.Nodes, nodes...)
	}
	if arr, ok := menu["nodes"].([]interface{}); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok {
				if normalized := sanitizeNodeKey(s); normalized != "" {
					cfg.Nodes = append(cfg.Nodes, normalized)
				}
			}
		}
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
	if len(cfg.Nodes) == 0 {
		cfg.Nodes = inferMenuNodes("", "", "")
	}
	return cfg
}
