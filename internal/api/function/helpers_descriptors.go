package function

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/svc"
)

func descriptors(ctx context.Context, svcCtx *svc.ServiceContext, req *DescriptorsRequest) (*DescriptorsResponse, error) {
	scope := currentFunctionScope(ctx)
	gameID := strings.TrimSpace(req.GameId)
	if scope.GameID != "" {
		gameID = scope.GameID
	}
	descs, err := svcCtx.FunctionModel.ListDescriptors(ctx, gameID)
	if err != nil {
		return nil, err
	}

	items := make([]Descriptor, 0, len(descs))
	for _, d := range descs {
		items = append(items, Descriptor{
			Id:          d.FunctionID,
			Name:        "", // FunctionDescriptor doesn't have Name
			Description: "", // FunctionDescriptor doesn't have Description
			Input:       rawJSONFromAny(d.Input),
			Output:      rawJSONFromAny(d.Output),
		})
	}

	return &DescriptorsResponse{
		Items: items,
	}, nil
}

func currentFunctionScope(ctx context.Context) svc.GameScope {
	return svc.GameScopeFromContext(ctx)
}

// Batch operations implementations
