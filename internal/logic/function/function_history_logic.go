package function

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
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

// FunctionHistory returns newest-first history together with the total count
// for paginated HTTP responses.
func (l *FunctionHistoryLogic) FunctionHistory(req *FunctionHistoryRequest) ([]FunctionHistoryItem, int, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, 0, err
	}
	fn, err := getOrCreateFunctionRecord(l.ctx, l.svcCtx, functionID)
	if err != nil {
		return nil, 0, err
	}

	items := []FunctionHistoryItem{
		{
			ID:        fmt.Sprintf("function-created:%d", fn.ID),
			Action:    "function_created",
			Operator:  "system",
			Timestamp: fn.CreatedAt.UTC().Format(time.RFC3339),
			Details: rawJSONFromValue(map[string]interface{}{
				"functionId": fn.FunctionID,
				"name":       fn.Name,
				"status":     fn.Status,
			}),
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
			items = append(items, FunctionHistoryItem{
				ID:        fmt.Sprintf("%s:v%d", key, v.Version),
				Action:    action,
				Operator:  v.CreatedBy,
				Timestamp: v.CreatedAt.UTC().Format(time.RFC3339),
				Details: rawJSONFromValue(map[string]interface{}{
					"version": v.Version,
					"message": v.Message,
					"config":  details,
				}),
			})
		}
		return nil
	}

	if err := appendConfigVersions("function_form:"+functionID, "form_config_updated"); err != nil {
		return nil, 0, err
	}

	sort.Slice(items, func(i, j int) bool {
		ti := parseHistoryTime(items[i].Timestamp)
		tj := parseHistoryTime(items[j].Timestamp)
		return ti.After(tj)
	})
	total := len(items)
	// 历史按时间倒序后分页截断，避免长寿命函数的配置历史全量下发
	if req.Offset > 0 {
		if req.Offset >= total {
			return []FunctionHistoryItem{}, total, nil
		}
		items = items[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(items) {
		items = items[:req.Limit]
	}
	return items, total, nil
}

// FunctionHistoryPaged is an explicit alias for HTTP callers that want to
// make pagination semantics visible at the call site.
func (l *FunctionHistoryLogic) FunctionHistoryPaged(req *FunctionHistoryRequest) ([]FunctionHistoryItem, int, error) {
	return l.FunctionHistory(req)
}

func parseHistoryTime(raw string) time.Time {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return model.NowUTC()
	}
	return t
}
