package provider

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns the list of providers
func (s *Service) List(ctx context.Context, req *ProvidersListRequest) (*ProvidersListResponse, error) {
	store, err := ensureRegistryStore(s.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}
	if req == nil {
		req = &ProvidersListRequest{}
	}

	providers := store.ListOpenAPIProviders()
	items := make([]ProviderItem, 0, len(providers))
	for _, provider := range providers {
		items = append(items, buildProviderMeta(*provider, false))
	}

	return &ProvidersListResponse{
		Items:    items,
		Total:    len(items),
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Capabilities returns the capabilities of all providers
func (s *Service) Capabilities(ctx context.Context, req *ProvidersCapabilitiesRequest) (*ProvidersCapabilitiesResponse, error) {
	store, err := ensureRegistryStore(s.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}
	providers := store.ListOpenAPIProviders()

	items := make([]ProviderItem, 0, len(providers))
	for _, provider := range providers {
		items = append(items, buildProviderMeta(*provider, true))
	}

	return &ProvidersCapabilitiesResponse{
		Items: items,
		Total: len(items),
	}, nil
}

// Descriptors returns the OpenAPI descriptors of all providers
func (s *Service) Descriptors(ctx context.Context, req *ProvidersDescriptorsRequest) (*ProvidersDescriptorsResponse, error) {
	store, err := ensureRegistryStore(s.svcCtx.RegistryStore)
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

	return &ProvidersDescriptorsResponse{
		ProviderManifests: providerManifests,
	}, nil
}

// Detail returns the details of a provider
func (s *Service) Detail(ctx context.Context, req *ProviderDetailRequest) (*ProviderDetailResponse, error) {
	caps, err := getProviderCaps(s.svcCtx.RegistryStore, req.ID)
	if err != nil {
		return nil, err
	}

	meta := ProviderItem(buildProviderMeta(caps, true))
	return &meta, nil
}

// Entities returns the entities of providers
func (s *Service) Entities(ctx context.Context, req *ProvidersEntitiesRequest) (*ProvidersEntitiesResponse, error) {
	store, err := ensureRegistryStore(s.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}

	var entities []map[string]interface{}
	if strings.TrimSpace(req.ID) == "" || req.ID == "*" {
		entities = aggregateEntities(store)
	} else {
		entities, err = aggregateEntitiesForProvider(store, req.ID)
		if err != nil {
			return nil, err
		}
	}

	return &ProvidersEntitiesResponse{
		Items: entities,
		Total: len(entities),
	}, nil
}

// Delete deletes a provider
func (s *Service) Delete(ctx context.Context, req *ProviderActionRequest) (*ProviderDeleteResponse, error) {
	if err := deleteProviderCaps(s.svcCtx.RegistryStore, req.ID); err != nil {
		return nil, err
	}

	return &ProviderDeleteResponse{
		ID: req.ID,
	}, nil
}

// Reload reloads a provider
func (s *Service) Reload(ctx context.Context, req *ProviderActionRequest) (*ProviderReloadResponse, error) {
	caps, err := getProviderCaps(s.svcCtx.RegistryStore, req.ID)
	if err != nil {
		return nil, err
	}

	if _, err := decodeOpenAPIDoc(caps.OpenAPIDoc); err != nil {
		return nil, err
	}

	refreshProviderTimestamp(s.svcCtx.RegistryStore, caps)

	return &ProviderReloadResponse{
		ID:        caps.ID,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
