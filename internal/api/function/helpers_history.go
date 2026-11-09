package function

import (
	"context"

	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	"github.com/cuihairu/croupier/internal/svc"
)

// fetchFunctionHistoryPage delegates to the history logic; it is a package
// seam letting tests exercise nil-item handling without a full logic stack.
var fetchFunctionHistoryPage = func(ctx context.Context, svcCtx *svc.ServiceContext, req *logicfunction.FunctionHistoryRequest) ([]logicfunction.FunctionHistoryItem, int, error) {
	return logicfunction.NewFunctionHistoryLogic(ctx, svcCtx).FunctionHistoryPaged(req)
}

func functionHistory(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionHistoryRequest) (*FunctionHistoryResponse, error) {
	// 历史默认取最近一页；limit/offset 由 query 传入
	limit := req.Limit
	if limit <= 0 {
		limit = functionHistoryDefaultLimit
	}
	if limit > functionHistoryMaxLimit {
		limit = functionHistoryMaxLimit
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	items, total, err := fetchFunctionHistoryPage(ctx, svcCtx, &logicfunction.FunctionHistoryRequest{ID: req.ID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []logicfunction.FunctionHistoryItem{}
	}
	result := make([]FunctionHistoryItem, 0, len(items))
	for _, item := range items {
		result = append(result, FunctionHistoryItem{
			ID: item.ID, Action: item.Action, Operator: item.Operator,
			Timestamp: item.Timestamp, Details: rawJSONFromBytes(item.Details),
		})
	}
	return &FunctionHistoryResponse{Items: result, Total: total}, nil
}

// --- 统一执行链薄壳（internal/platform/functioninvoke）---
//
// functionInvoke 的完整执行链（scope、禁用、分配、策略审批、审计、执行
// 留痕）已上收为唯一实现 functioninvoke.Invoke，HTTP 入口与 logic 侧薄壳
// 共用同一实现。本包只保留 API DTO 适配与既有用例依赖的私有 helper 转发。
