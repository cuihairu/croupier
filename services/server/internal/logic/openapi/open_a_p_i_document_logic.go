package openapi

import (
	"context"
	"encoding/json"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type OpenAPIDocumentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenAPIDocumentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenAPIDocumentLogic {
	return &OpenAPIDocumentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenAPIDocumentLogic) OpenAPIDocument(_ *types.OpenAPIDocumentRequest) (*types.OpenAPIDocumentResponse, error) {
	if l.svcCtx == nil || l.svcCtx.RegistryStore == nil {
		return nil, errorx.NewInternalError("registry store is not ready")
	}

	doc, err := l.svcCtx.RegistryStore.BuildOpenAPISpec()
	if err != nil {
		l.Errorf("failed to build OpenAPI doc: %v", err)
		return nil, err
	}

	b, err := doc.MarshalJSON()
	if err != nil {
		l.Errorf("failed to marshal OpenAPI doc: %v", err)
		return nil, err
	}

	var spec interface{}
	if err := json.Unmarshal(b, &spec); err != nil {
		l.Errorf("failed to unmarshal OpenAPI doc JSON: %v", err)
		return nil, err
	}

	return &types.OpenAPIDocumentResponse{Spec: spec}, nil
}
