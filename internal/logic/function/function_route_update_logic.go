package function

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionRouteUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionRouteUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionRouteUpdateLogic {
	return &FunctionRouteUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionRouteUpdateLogic) FunctionRouteUpdate(req *FunctionRouteUpdateRequest) (*FunctionRouteResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := getOrCreateFunctionRecord(l.ctx, l.svcCtx, functionID)
	if err != nil {
		return nil, err
	}

	// Build menu from request
	nodes := make([]string, 0, len(req.Nodes))
	for _, n := range req.Nodes {
		if normalized := sanitizeNodeKey(n); normalized != "" {
			nodes = append(nodes, normalized)
		}
	}

	menu := map[string]interface{}{
		"nodes":  nodes,
		"path":   strings.TrimSpace(req.Path),
		"order":  req.Order,
		"hidden": req.Hidden,
	}

	// Update metadata
	meta := fn.Metadata
	if meta == nil {
		meta = map[string]interface{}{}
	}
	meta["menu"] = menu
	if err := l.svcCtx.FunctionModel.Update(l.ctx, fn.ID, map[string]interface{}{"metadata": meta}); err != nil {
		return nil, err
	}
	if err := persistFunctionRouteVersion(l.ctx, l.svcCtx, fn.FunctionID, menu, "update route config"); err != nil {
		return nil, err
	}

	return &FunctionRouteResponse{
		Menu: FunctionRouteConfig{
			Nodes:  nodes,
			Path:   strings.TrimSpace(req.Path),
			Order:  req.Order,
			Hidden: req.Hidden,
		},
		Source: "metadata",
	}, nil
}
