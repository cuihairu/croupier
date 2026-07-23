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
			return &FunctionRouteResponse{
				Menu: FunctionRouteConfig{
					Nodes:  []string{},
					Path:   "",
					Order:  100,
					Hidden: false,
				},
				Source: "default",
			}, nil
		}
		return nil, err
	}

	// Extract menu from metadata if exists
	menu := FunctionRouteConfig{
		Nodes:  []string{},
		Path:   "",
		Order:  100,
		Hidden: false,
	}
	source := "default"

	if fn.Metadata != nil {
		if mm, ok := fn.Metadata["menu"].(map[string]interface{}); ok && mm != nil {
			if nodes, ok := mm["nodes"].([]interface{}); ok {
				for _, n := range nodes {
					if s, ok := n.(string); ok && s != "" {
						menu.Nodes = append(menu.Nodes, s)
					}
				}
			}
			if path, ok := mm["path"].(string); ok {
				menu.Path = path
			}
			if order, ok := mm["order"].(float64); ok {
				menu.Order = int(order)
			}
			if hidden, ok := mm["hidden"].(bool); ok {
				menu.Hidden = hidden
			}
			source = "metadata"
		}
	}

	return &FunctionRouteResponse{
		Menu:   menu,
		Source: source,
	}, nil
}
