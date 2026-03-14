package function

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FunctionUIHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionUIHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionUIHistoryLogic {
	return &FunctionUIHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionUIHistoryLogic) FunctionUIHistory(req *types.FunctionUIHistoryRequest) (*types.FunctionUIHistoryResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}
	versions, err := l.svcCtx.ConfigVersionModel.List(l.ctx, functionUIHistoryKey(functionID))
	if err != nil {
		return nil, err
	}
	items := make([]types.FunctionUIHistoryItem, 0, len(versions))
	for _, v := range versions {
		entry := types.FunctionUIHistoryItem{
			Version:   v.Version,
			Message:   v.Message,
			CreatedBy: v.CreatedBy,
			CreatedAt: formatVersionTime(v.CreatedAt),
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(v.Value), &cfg); err == nil {
			entry.Schema = cfg["schema"]
			entry.Layout = cfg["layout"]
			entry.Components = cfg["components"]
		}
		items = append(items, entry)
	}
	return &types.FunctionUIHistoryResponse{Items: items}, nil
}

func formatVersionTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}
