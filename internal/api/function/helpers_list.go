package function

import (
	"context"
	"strings"

	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

func functionsList(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionsListRequest) (*FunctionsListResponse, error) {
	scope := currentFunctionScope(ctx)
	gameID := strings.TrimSpace(req.GameId)
	if scope.GameID != "" {
		gameID = scope.GameID
	}
	logicResp, err := logicfunction.NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&logicfunction.FunctionsListRequest{
		Page:     1,
		PageSize: 10000,
		GameId:   gameID,
		Resource: req.Resource,
		Status:   req.Status,
	})
	if err != nil {
		return nil, err
	}

	dbItems, _, err := svcCtx.FunctionModel.List(ctx, model.ListFunctionsOptions{
		PaginationOptions: model.NewPagination(1, 10000),
		GameID:            gameID,
	})
	if err != nil {
		return nil, err
	}
	dbIndex := make(map[string]model.Function, len(dbItems))
	for _, item := range dbItems {
		dbIndex[item.FunctionID] = item
	}

	items := make([]Function, 0, len(logicResp.Items))
	for _, fn := range logicResp.Items {
		dbFn, ok := dbIndex[fn.ID]
		resource := fn.Resource
		version := fn.Version
		specFormat := fn.SpecFormat
		openAPISpec := rawJSONFromAny(fn.OpenAPISpec)
		description := fn.Description
		createdAt := ""
		updatedAt := ""
		if ok {
			if resource == "" {
				resource = getStringFromMetadata(dbFn.Metadata, "resource")
			}
			if version == "" {
				version = getStringFromMetadata(dbFn.Metadata, "version")
			}
			if specFormat == "" {
				specFormat = getStringFromMetadata(dbFn.Metadata, "spec_format")
			}
			if len(openAPISpec) == 0 {
				openAPISpec = rawJSONFromAny(jsonValueFromMetadata(dbFn.Metadata, "openapi_spec"))
			}
			if description == "" {
				description = dbFn.Description
			}
			createdAt = utils.FormatTimestamp(dbFn.CreatedAt)
			updatedAt = utils.FormatTimestamp(dbFn.UpdatedAt)
		}
		items = append(items, Function{
			Id:          fn.ID,
			Name:        fn.Name,
			Description: description,
			Resource:    resource,
			GameId:      fn.GameId,
			Status:      fn.Status,
			Version:     version,
			Instances:   fn.Instances,
			SpecFormat:  specFormat,
			Tags:        firstNonEmptySlice(getStringSliceFromMetadata(dbFn.Metadata, "tags"), fn.Tags),
			Summary:     firstNonEmptyMap(getLocalizedTextFromMetadata(dbFn.Metadata, "summary"), fn.Summary),
			OpenAPISpec: openAPISpec,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
	}

	return &FunctionsListResponse{
		Items: items,
		Total: logicResp.Total,
		Page:  logicResp.Page,
		Size:  logicResp.Size,
	}, nil
}

func functionsPending(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionsPendingRequest) (*FunctionsPendingResponse, error) {
	// Implementation would query pending functions
	return &FunctionsPendingResponse{
		Items: []PendingFunction{},
	}, nil
}

func functionDetail(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionDetailRequest) (*FunctionDetailResponse, error) {
	logicResp, err := logicfunction.NewFunctionDetailLogic(ctx, svcCtx).FunctionDetail(&logicfunction.FunctionDetailRequest{
		ID: req.ID,
	})
	if err != nil {
		return nil, err
	}

	return &FunctionDetailResponse{
		Function: Function{
			Id:          logicResp.Function.ID,
			Name:        logicResp.Function.Name,
			Description: logicResp.Function.Description,
			Resource:    logicResp.Function.Resource,
			GameId:      logicResp.Function.GameId,
			Status:      logicResp.Function.Status,
			Version:     logicResp.Function.Version,
			Instances:   logicResp.Function.Instances,
			SpecFormat:  logicResp.Function.SpecFormat,
			OpenAPISpec: rawJSONFromBytes(logicResp.Function.OpenAPISpec),
			CreatedAt:   logicResp.Function.CreatedAt,
			UpdatedAt:   logicResp.Function.UpdatedAt,
		},
		Descriptor: FunctionDescriptor{
			Input:  rawJSONFromBytes(logicResp.Descriptor.Input),
			Output: rawJSONFromBytes(logicResp.Descriptor.Output),
			Schema: rawJSONFromBytes(logicResp.Descriptor.Schema),
		},
	}, nil
}

func functionAnalytics(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionAnalyticsRequest) (*FunctionAnalyticsResponse, error) {
	stats, err := logicfunction.NewFunctionAnalyticsLogic(ctx, svcCtx).FunctionAnalytics(
		&logicfunction.FunctionAnalyticsRequest{ID: req.ID},
	)
	if err != nil {
		return nil, err
	}
	return &FunctionAnalyticsResponse{
		TotalCalls:     stats.TotalCalls,
		FailedCalls:    stats.FailedCalls,
		SuccessRate:    stats.SuccessRate,
		AvgLatency:     stats.AvgLatency,
		CallsToday:     stats.CallsToday,
		CallsThisWeek:  stats.CallsThisWeek,
		CallsThisMonth: stats.CallsThisMonth,
	}, nil
}
