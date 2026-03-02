package function

import (
	"context"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionWarningsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionWarningsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionWarningsLogic {
	return &FunctionWarningsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionWarningsLogic) FunctionWarnings(req *types.FunctionWarningsRequest) (*types.FunctionWarningsResponse, error) {
	if l.svcCtx.RegistryStore == nil {
		return &types.FunctionWarningsResponse{Items: []types.FunctionWarningItem{}}, nil
	}
	items := l.svcCtx.RegistryStore.ListRegistrationWarnings(reg.RegistrationWarningFilter{
		FunctionID: req.FunctionID,
		AgentID:    req.AgentID,
		Code:       req.Code,
		Limit:      req.Limit,
	})
	out := make([]types.FunctionWarningItem, 0, len(items))
	for _, item := range items {
		out = append(out, types.FunctionWarningItem{
			Key:        item.Key,
			AgentID:    item.AgentID,
			FunctionID: item.FunctionID,
			Version:    item.Version,
			Code:       item.Code,
			Message:    item.Message,
			Count:      item.Count,
			FirstSeen:  item.FirstSeen.Format(time.RFC3339),
			LastSeen:   item.LastSeen.Format(time.RFC3339),
		})
	}
	return &types.FunctionWarningsResponse{Items: out}, nil
}
