package function

import (
	"context"
	"strings"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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

func (l *FunctionRouteUpdateLogic) FunctionRouteUpdate(req *types.FunctionRouteUpdateRequest) (*types.FunctionRouteResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := getOrCreateFunctionRecord(l.ctx, l.svcCtx, functionID)
	if err != nil {
		return nil, err
	}
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
	base := defaultMenu()
	mergeShallow(base, menu)
	applyEntityMenuDefaults(base, "", "", functionID)

	meta := fn.Metadata
	if meta == nil {
		meta = datatypes.JSONMap{}
	}
	meta["menu"] = base
	if err := l.svcCtx.FunctionModel.Update(l.ctx, fn.ID, map[string]interface{}{"metadata": meta}); err != nil {
		return nil, err
	}
	if err := persistFunctionRouteVersion(l.ctx, l.svcCtx, fn.FunctionID, base, "update route config"); err != nil {
		return nil, err
	}

	return &types.FunctionRouteResponse{
		Menu:   normalizeMenuConfig(base),
		Source: "metadata",
	}, nil
}
