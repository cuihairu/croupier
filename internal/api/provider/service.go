package provider

import (
	"context"
	"sort"
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
			// Include functions and resources counts
			"functions": len(openAPIDocFunctions(doc)),
			"resources": len(openAPIDocResources(doc)),
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

// Resources returns the resources of providers
func (s *Service) Resources(ctx context.Context, req *ProvidersResourcesRequest) (*ProvidersResourcesResponse, error) {
	store, err := ensureRegistryStore(s.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}

	var resources []map[string]interface{}
	if strings.TrimSpace(req.ID) == "" || req.ID == "*" {
		resources = aggregateResources(store)
	} else {
		resources, err = aggregateResourcesForProvider(store, req.ID)
		if err != nil {
			return nil, err
		}
	}

	return &ProvidersResourcesResponse{
		Items: resources,
		Total: len(resources),
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

// SdkStats 聚合在线 provider 会话的 SDK 语言/版本分布（F：sdk-stats 页面）。
// 语言/版本缺失归入 "unknown"；语言按实例数降序、版本按实例数降序排列。
func (s *Service) SdkStats(ctx context.Context, _ *SdkStatsRequest) (*SdkStatsResponse, error) {
	store, err := ensureRegistryStore(s.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}

	snapshots := store.ProviderSessionSnapshots()
	instances := make([]SdkInstanceItem, 0, len(snapshots))
	for _, snapshot := range snapshots {
		language := strings.TrimSpace(snapshot.SDKLanguage)
		if language == "" {
			language = "unknown"
		}
		version := strings.TrimSpace(snapshot.SDKVersion)
		if version == "" {
			version = "unknown"
		}
		instances = append(instances, SdkInstanceItem{
			ProviderID:   snapshot.ProviderID,
			AgentID:      snapshot.AgentID,
			GameID:       snapshot.GameID,
			Env:          snapshot.Env,
			ServiceAddr:  snapshot.Addr,
			SdkName:      snapshot.SDKName,
			SdkLanguage:  language,
			SdkVersion:   version,
			LastSeenUnix: snapshot.LastSeenUnix,
		})
	}

	response := &SdkStatsResponse{
		TotalInstances: len(instances),
		Languages:      aggregateSdkLanguages(instances),
		Instances:      instances,
	}
	return response, nil
}

func aggregateSdkLanguages(instances []SdkInstanceItem) []SdkLanguageStats {
	versionsByLanguage := map[string]map[string]int{}
	countByLanguage := map[string]int{}
	for _, item := range instances {
		if versionsByLanguage[item.SdkLanguage] == nil {
			versionsByLanguage[item.SdkLanguage] = map[string]int{}
		}
		versionsByLanguage[item.SdkLanguage][item.SdkVersion]++
		countByLanguage[item.SdkLanguage]++
	}

	languages := make([]string, 0, len(countByLanguage))
	for language := range countByLanguage {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	// 实例数多的语言排前；同数量按语言名稳定排序
	sort.SliceStable(languages, func(i, j int) bool {
		return countByLanguage[languages[i]] > countByLanguage[languages[j]]
	})

	out := make([]SdkLanguageStats, 0, len(languages))
	for _, language := range languages {
		versionCounts := versionsByLanguage[language]
		versions := make([]string, 0, len(versionCounts))
		for version := range versionCounts {
			versions = append(versions, version)
		}
		sort.Strings(versions)
		sort.SliceStable(versions, func(i, j int) bool {
			return versionCounts[versions[i]] > versionCounts[versions[j]]
		})
		versionStats := make([]SdkVersionCount, 0, len(versions))
		for _, version := range versions {
			versionStats = append(versionStats, SdkVersionCount{Version: version, Count: versionCounts[version]})
		}
		out = append(out, SdkLanguageStats{
			Language: language,
			Count:    countByLanguage[language],
			Versions: versionStats,
		})
	}
	return out
}
