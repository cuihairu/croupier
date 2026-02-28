package function

import (
	"context"
	"strings"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionRouteUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionRouteUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionRouteUpdateLogic {
	return &FunctionRouteUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionRouteUpdateLogic) FunctionRouteUpdate(req *types.FunctionRouteUpdateRequest) (*types.FunctionRouteResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	menu := map[string]interface{}{
		"section": strings.TrimSpace(req.Section),
		"group":   strings.TrimSpace(req.Group),
		"path":    strings.TrimSpace(req.Path),
		"order":   req.Order,
		"hidden":  req.Hidden,
	}
	base := defaultMenu()
	mergeShallow(base, menu)

	meta := fn.Metadata
	if meta == nil {
		meta = datatypes.JSONMap{}
	}
	meta["menu"] = base
	if err := l.svcCtx.FunctionModel.Update(l.ctx, fn.ID, map[string]interface{}{"metadata": meta}); err != nil {
		return nil, err
	}

	return &types.FunctionRouteResponse{
		Menu:   normalizeMenuConfig(base),
		Source: "metadata",
	}, nil
}
