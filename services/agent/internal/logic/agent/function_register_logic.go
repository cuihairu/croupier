// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"strings"
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

func (l *FunctionRegisterLogic) FunctionRegister(req *types.FunctionRegisterRequest) (*types.FunctionRegisterResponse, error) {
	if req == nil {
		return nil, errors.New("missing request payload")
	}
	functionID := strings.TrimSpace(req.FunctionId)
	if functionID == "" {
		return nil, errors.New("function_id 不能为空")
	}

	state := l.svcCtx.AgentState
	state.Mu.Lock()
	state.Functions[functionID] = &svc.FunctionRecord{
		ID:         functionID,
		GameID:     strings.TrimSpace(req.GameId),
		Env:        strings.TrimSpace(req.Env),
		Descriptor: svc.CloneInterfaceMap(req.Descriptor),
		Schema:     svc.CloneInterfaceMap(req.Schema),
		Metadata:   svc.CloneInterfaceMap(req.Metadata),
		Registered: time.Now(),
	}
	state.Mu.Unlock()

	return &types.FunctionRegisterResponse{
		Success: true,
		Message: "function registered",
	}, nil
}
