package function

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FunctionHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionHistoryLogic {
	return &FunctionHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionHistoryLogic) FunctionHistory(req *types.FunctionHistoryRequest) ([]types.FunctionHistoryItem, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := getOrCreateFunctionRecord(l.ctx, l.svcCtx, functionID)
	if err != nil {
		return nil, err
	}

	items := []types.FunctionHistoryItem{
		{
			ID:        fmt.Sprintf("function-created:%d", fn.ID),
			Action:    "function_created",
			Operator:  "system",
			Timestamp: fn.CreatedAt.UTC().Format(time.RFC3339),
			Details: map[string]interface{}{
				"functionId": fn.FunctionID,
				"name":       fn.Name,
				"status":     fn.Status,
			},
		},
	}

	appendConfigVersions := func(key, action string) error {
		if l.svcCtx.ConfigVersionModel == nil {
			return nil
		}
		versions, listErr := l.svcCtx.ConfigVersionModel.List(l.ctx, key)
		if listErr != nil {
			return listErr
		}
		for _, v := range versions {
			details := map[string]interface{}{}
			if strings.TrimSpace(v.Value) != "" {
				_ = json.Unmarshal([]byte(v.Value), &details)
			}
			items = append(items, types.FunctionHistoryItem{
				ID:        fmt.Sprintf("%s:v%d", key, v.Version),
				Action:    action,
				Operator:  v.CreatedBy,
				Timestamp: v.CreatedAt.UTC().Format(time.RFC3339),
				Details: map[string]interface{}{
					"version": v.Version,
					"message": v.Message,
					"config":  details,
				},
			})
		}
		return nil
	}

	if err := appendConfigVersions(functionUIHistoryKey(functionID), "ui_config_updated"); err != nil {
		return nil, err
	}
	if err := appendConfigVersions(functionRouteHistoryKey(functionID), "route_config_updated"); err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool {
		ti := parseHistoryTime(items[i].Timestamp)
		tj := parseHistoryTime(items[j].Timestamp)
		return ti.After(tj)
	})
	return items, nil
}

func parseHistoryTime(raw string) time.Time {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return model.NowUTC()
	}
	return t
}
