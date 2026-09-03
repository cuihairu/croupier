package function

import (
	"context"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionWarningsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionWarningsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionWarningsLogic {
	return &FunctionWarningsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionWarningsLogic) FunctionWarnings(req *FunctionWarningsRequest) (*FunctionWarningsResponse, error) {
	if l.svcCtx.RegistryStore == nil {
		return &FunctionWarningsResponse{Items: []FunctionWarningItem{}}, nil
	}
	items := l.svcCtx.RegistryStore.ListRegistrationWarnings(reg.RegistrationWarningFilter{
		FunctionID: req.FunctionID,
		AgentID:    req.AgentID,
		Code:       req.Code,
		Limit:      req.Limit,
	})
	out := make([]FunctionWarningItem, 0, len(items))
	for _, item := range items {
		out = append(out, FunctionWarningItem{
			Key:        item.Key,
			AgentID:    item.AgentID,
			FunctionID: item.FunctionID,
			Version:    item.Version,
			Code:       item.Code,
			Message:    item.Message,
			Count:      item.Count,
			FirstSeen:  item.FirstSeen.Format(time.RFC3339),
			LastSeen:   item.LastSeen.Format(time.RFC3339),
			Read:       item.Read,
		})
	}
	return &FunctionWarningsResponse{Items: out}, nil
}
