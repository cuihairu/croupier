// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type ProvidersDescriptorsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取提供者描述符
func NewProvidersDescriptorsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersDescriptorsLogic {
	return &ProvidersDescriptorsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersDescriptorsLogic) ProvidersDescriptors(req *types.ProvidersDescriptorsRequest) (resp *types.ProvidersDescriptorsResponse, err error) {
	store, err := ensureRegistryStore(l.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}

	// Get all OpenAPI providers
	providers := store.ListOpenAPIProviders()
	providerManifests := make(map[string]interface{})

	for _, provider := range providers {
		doc, err := decodeOpenAPIDoc(provider.OpenAPIDoc)
		if err != nil {
			continue
		}
		// Extract manifest info from OpenAPI doc
		providerManifests[provider.ID] = map[string]interface{}{
			"id":        provider.ID,
			"version":   provider.Version,
			"lang":      provider.Lang,
			"sdk":       provider.SDK,
			"updatedAt": provider.UpdatedAt,
			// Include functions and entities counts
			"functions": len(openAPIDocFunctions(doc)),
			"entities":  len(openAPIDocEntities(doc)),
			// Full OpenAPI doc
			"openapi": doc,
		}
	}

	return &types.ProvidersDescriptorsResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"provider_manifests": providerManifests,
		},
	}, nil
}
