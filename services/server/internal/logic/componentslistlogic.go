// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewComponentsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsListLogic {
	return &ComponentsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsListLogic) ComponentsList(req *types.ComponentsQuery) (resp *types.ComponentsListResponse, err error) {
	cm := l.svcCtx.ComponentManager()
	if cm == nil {
		return &types.ComponentsListResponse{Components: map[string]any{}}, nil
	}
	category := strings.TrimSpace(req.Category)
	if category != "" {
		return &types.ComponentsListResponse{Components: cm.ListByCategory(category)}, nil
	}
	data := map[string]any{
		"installed": cm.ListInstalled(),
		"disabled":  cm.ListDisabled(),
	}
	return &types.ComponentsListResponse{Components: data}, nil
}
