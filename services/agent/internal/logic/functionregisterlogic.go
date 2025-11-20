package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionRegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionRegisterLogic {
	return &FunctionRegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionRegisterLogic) FunctionRegister(req *types.FunctionRegisterRequest) (resp *types.FunctionRegisterResponse, err error) {
	logx.Infof("Function registration: gameId=%s, env=%s, functionId=%s",
		req.GameId, req.Env, req.FunctionId)

	// Validate required fields
	if req.FunctionId == "" || req.GameId == "" || req.Env == "" {
		return &types.FunctionRegisterResponse{
			Success: false,
			Message: "missing required fields: function_id, game_id, env",
		}, nil
	}

	// Create function key
	functionKey := fmt.Sprintf("%s:%s:%s", req.GameId, req.Env, req.FunctionId)

	// Validate descriptor
	if req.Descriptor == nil {
		return &types.FunctionRegisterResponse{
			Success: false,
			Message: "function descriptor is required",
		}, nil
	}

	// Prepare function data
	functionData := map[string]interface{}{
		"function_id": req.FunctionId,
		"game_id":     req.GameId,
		"env":         req.Env,
		"descriptor":  req.Descriptor,
		"schema":      req.Schema,
		"metadata":    req.Metadata,
		"registered_at": time.Now().Unix(),
		"status":      "active",
	}

	// Register function
	l.svcCtx.AgentStore.RegisterFunction(functionKey, functionData)

	logx.Infof("Function registered successfully: %s", functionKey)

	return &types.FunctionRegisterResponse{
		Success: true,
		Message: "Function registered successfully",
	}, nil
}