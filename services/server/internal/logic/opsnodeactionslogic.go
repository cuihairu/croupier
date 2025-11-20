package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodeDrainLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsNodeDrainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeDrainLogic {
	return &OpsNodeDrainLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeDrainLogic) OpsNodeDrain(req *types.OpsNodeActionRequest) (*types.GenericOkResponse, error) {
	return nodeCommandWithDraining(l.svcCtx, req, boolPtr(true), "drain")
}

type OpsNodeUndrainLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsNodeUndrainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeUndrainLogic {
	return &OpsNodeUndrainLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeUndrainLogic) OpsNodeUndrain(req *types.OpsNodeActionRequest) (*types.GenericOkResponse, error) {
	return nodeCommandWithDraining(l.svcCtx, req, boolPtr(false), "undrain")
}

type OpsNodeRestartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsNodeRestartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeRestartLogic {
	return &OpsNodeRestartLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeRestartLogic) OpsNodeRestart(req *types.OpsNodeActionRequest) (*types.GenericOkResponse, error) {
	return nodeCommandWithDraining(l.svcCtx, req, nil, "restart")
}

func nodeCommandWithDraining(ctx *svc.ServiceContext, req *types.OpsNodeActionRequest, draining *bool, cmd string) (*types.GenericOkResponse, error) {
	if req == nil || strings.TrimSpace(req.Id) == "" {
		return nil, ErrInvalidRequest
	}
	if draining != nil {
		ctx.SetNodeDraining(req.Id, *draining)
	}
	ctx.EnqueueNodeCommand(req.Id, cmd)
	return &types.GenericOkResponse{Ok: true}, nil
}

func boolPtr(v bool) *bool {
	return &v
}
