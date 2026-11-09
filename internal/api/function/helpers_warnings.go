package function

import (
	"context"
	"sort"
	"strings"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

func functionWarnings(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionWarningsRequest) (*FunctionWarningsResponse, error) {
	scope := currentFunctionScope(ctx)

	if svcCtx.RegistryStore == nil {
		return &FunctionWarningsResponse{Items: []FunctionWarningItem{}}, nil
	}

	filter := reg.RegistrationWarningFilter{
		GameID:     scope.GameID,
		Env:        scope.Env,
		FunctionID: req.FunctionID,
		AgentID:    req.AgentID,
		Code:       req.Code,
		Limit:      req.Limit,
	}

	warnings := svcCtx.RegistryStore.ListRegistrationWarnings(filter)

	items := make([]FunctionWarningItem, 0, len(warnings))
	for _, w := range warnings {
		items = append(items, FunctionWarningItem{
			Key:        w.Key,
			GameID:     w.GameID,
			Env:        w.Env,
			AgentID:    w.AgentID,
			FunctionID: w.FunctionID,
			Version:    w.Version,
			Code:       w.Code,
			Message:    w.Message,
			Count:      w.Count,
			FirstSeen:  w.FirstSeen.Format(time.RFC3339),
			LastSeen:   w.LastSeen.Format(time.RFC3339),
		})
	}

	return &FunctionWarningsResponse{
		Items: items,
	}, nil
}

// Descriptors implementations

// functionWarningFilterOptions 按 #34 提供注册警告过滤下拉的服务端聚合选项：
// 当前 scope 警告全集（与 functionWarnings 同源、不过滤）下 distinct
// 函数 ID / Agent 及各自警告条数，供 Warnings 页两个下拉一次取全。
func functionWarningFilterOptions(ctx context.Context, svcCtx *svc.ServiceContext) (*FunctionWarningFilterOptionsResponse, error) {
	scope := currentFunctionScope(ctx)

	resp := &FunctionWarningFilterOptionsResponse{
		Functions: []WarningFilterOptionItem{},
		Agents:    []WarningFilterOptionItem{},
	}
	if svcCtx.RegistryStore == nil {
		return resp, nil
	}

	warnings := svcCtx.RegistryStore.ListRegistrationWarnings(reg.RegistrationWarningFilter{
		GameID: scope.GameID,
		Env:    scope.Env,
		Limit:  0, // 聚合用全集，不限条数
	})

	funcCounts := map[string]int64{}
	agentCounts := map[string]int64{}
	for _, w := range warnings {
		if v := strings.TrimSpace(w.FunctionID); v != "" {
			funcCounts[v]++
		}
		if v := strings.TrimSpace(w.AgentID); v != "" {
			agentCounts[v]++
		}
	}
	resp.Functions = optionItemsFromCounts(funcCounts)
	resp.Agents = optionItemsFromCounts(agentCounts)
	return resp, nil
}

func optionItemsFromCounts(counts map[string]int64) []WarningFilterOptionItem {
	items := make([]WarningFilterOptionItem, 0, len(counts))
	for value, count := range counts {
		items = append(items, WarningFilterOptionItem{Value: value, Label: value, Count: count})
	}
	// 稳定序：条数降序、同数按 value 字典序（测试与展示均可预期）
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Value < items[j].Value
	})
	return items
}
