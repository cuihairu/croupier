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

// SdkStats aggregates the SDK language/version distribution across online
// provider sessions (F: the sdk-stats page). A missing language/version falls
// into "unknown"; languages and versions are each sorted by instance count
// descending. metaKey/metaValue perform a substring filter on user instance
// metadata (case-insensitive; online sessions are in-memory, so a linear
// filter suffices). When ctx carries a game scope, instances are filtered by
// (gameID, env).
func (s *Service) SdkStats(ctx context.Context, req *SdkStatsRequest) (*SdkStatsResponse, error) {
	store, err := ensureRegistryStore(s.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}

	if req == nil {
		req = &SdkStatsRequest{}
	}
	// Case normalization is handled inside matchMetadata; here only leading and
	// trailing whitespace is trimmed
	metaKey := strings.TrimSpace(req.MetaKey)
	metaValue := strings.TrimSpace(req.MetaValue)

	// #38: the SDK distribution is shown isolated per game -- the route hangs
	// off the scoped group and middleware guarantees the scope is complete and
	// authorized; a missing scope (internal calls / tests calling the service
	// directly) keeps the full-data behavior.
	scope := svc.GameScopeFromContext(ctx)
	snapshots := store.ProviderSessionSnapshots()
	instances := make([]SdkInstanceItem, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if scope.GameID != "" && strings.TrimSpace(snapshot.GameID) != scope.GameID {
			continue
		}
		if scope.Env != "" && strings.TrimSpace(snapshot.Env) != scope.Env {
			continue
		}
		metadata := snapshot.Metadata
		if metadata != nil {
			metadata = make(map[string]string, len(snapshot.Metadata))
			for key, value := range snapshot.Metadata {
				metadata[key] = value
			}
		}
		if metaKey != "" || metaValue != "" {
			if !matchMetadata(metadata, metaKey, metaValue) {
				continue
			}
		}
		language := strings.TrimSpace(snapshot.SDKLanguage)
		if language == "" {
			language = "unknown"
		}
		version := strings.TrimSpace(snapshot.SDKVersion)
		if version == "" {
			version = "unknown"
		}
		// #44: the registration time is normalized server-side (the same
		// pattern as openapi #27②) -- the in-memory snapshot's historical field
		// is naturally missing after session expiry/restart, so a zero value
		// falls back to the last-active time and the frontend need not check for
		// zero anymore.
		firstSeen := snapshot.FirstSeenUnix
		if firstSeen <= 0 {
			firstSeen = snapshot.LastSeenUnix
		}
		instances = append(instances, SdkInstanceItem{
			ProviderID:    snapshot.ProviderID,
			AgentID:       snapshot.AgentID,
			GameID:        snapshot.GameID,
			Env:           snapshot.Env,
			ServiceAddr:   snapshot.Addr,
			SdkName:       snapshot.SDKName,
			SdkLanguage:   language,
			SdkVersion:    version,
			Metadata:      metadata,
			FirstSeenUnix: firstSeen,
			LastSeenUnix:  snapshot.LastSeenUnix,
		})
	}

	response := &SdkStatsResponse{
		TotalInstances: len(instances),
		Languages:      aggregateSdkLanguages(instances),
		Instances:      instances,
	}
	return response, nil
}

// MetaOptions returns the deduplicated key/value aggregation of instance
// metadata under the current scope (#2): the filter dropdown options are
// supplied by the server (from actual instance metadata, aggregated from the
// #11 EAV table / online sessions), and the frontend must not derive them from
// the (already filtered) list data. The route hangs off the scoped group with
// the game scope in ctx; a missing scope (internal calls / direct test calls)
// keeps the full-data behavior.
func (s *Service) MetaOptions(ctx context.Context) (*ProviderMetaOptionsResponse, error) {
	store, err := ensureRegistryStore(s.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}
	scope := svc.GameScopeFromContext(ctx)
	items := store.ProviderMetaOptions(scope.GameID, scope.Env)
	if items == nil {
		items = []ProviderMetaKeyOption{}
	}
	return &ProviderMetaOptionsResponse{Items: items}, nil
}

// matchMetadata reports whether instance metadata satisfies both the key and
// value substring conditions (case-insensitive, with the arguments normalized
// internally; empty conditions always pass). The value condition matches
// against the key name as well as the full k=v pair -- when "searching by
// serverId" users often paste just the value or the whole pair without saying
// which key it is; this stays isomorphic with the frontend instance search
// haystack (k=v + key + value).
func matchMetadata(metadata map[string]string, key, value string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	value = strings.ToLower(strings.TrimSpace(value))
	if key == "" && value == "" {
		return true
	}
	if len(metadata) == 0 {
		return false
	}
	for k, v := range metadata {
		lk := strings.ToLower(k)
		lv := strings.ToLower(v)
		keyHit := key == "" || strings.Contains(lk, key)
		valueHit := value == "" ||
			strings.Contains(lv, value) ||
			strings.Contains(lk, value) ||
			strings.Contains(lk+"="+lv, value)
		if keyHit && valueHit {
			return true
		}
	}
	return false
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
	// Languages with more instances come first; ties are stably sorted by
	// language name
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
