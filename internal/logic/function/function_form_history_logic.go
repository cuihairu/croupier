package function

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionFormHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionFormHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionFormHistoryLogic {
	return &FunctionFormHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionFormHistoryLogic) FunctionFormHistory(req *FunctionFormHistoryRequest) (*FunctionFormHistoryResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}
	if l == nil || l.svcCtx == nil || l.svcCtx.ConfigVersionModel == nil {
		return &FunctionFormHistoryResponse{Items: []FunctionFormHistoryItem{}}, nil
	}
	versions, err := l.svcCtx.ConfigVersionModel.List(l.ctx, functionFormHistoryKey(functionID))
	if err != nil {
		return nil, err
	}

	// Filter by game scope for multi-game isolation.
	gameID, env := svc.GameScopeFromContext(l.ctx)
	scopeFiltered := make([]model.ConfigVersion, 0, len(versions))
	for _, v := range versions {
		if gameID != "" && v.GameID != gameID {
			continue
		}
		if env != "" && v.Env != env {
			continue
		}
		scopeFiltered = append(scopeFiltered, v)
	}

	items := make([]FunctionFormHistoryItem, 0, len(scopeFiltered))
	for _, v := range scopeFiltered {
		entry := FunctionFormHistoryItem{
			Version:   v.Version,
			Message:   v.Message,
			CreatedBy: v.CreatedBy,
			CreatedAt: formatVersionTime(v.CreatedAt),
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(v.Value), &cfg); err == nil {
			entry.Schema = rawJSONFromValue(cfg["schema"])
		}
		items = append(items, entry)
	}
	return &FunctionFormHistoryResponse{Items: items}, nil
}

func formatVersionTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}
