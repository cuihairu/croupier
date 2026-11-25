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
	// History defaults to the most recent page; limit/offset come from the query.
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

// --- Thin shell over the unified execution chain (internal/platform/functioninvoke) ---
//
// The full functionInvoke execution chain (scope, disabling, assignment, policy
// approval, audit and execution traceability) now lives in the single
// implementation functioninvoke.Invoke, shared by the HTTP entry point and the
// logic-side thin shell. This package only keeps API DTO adaptation plus the
// private helper forwarding that existing test cases rely on.
