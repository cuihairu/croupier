package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/api/analytics"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

const (
	officialAnalyticsExtensionID = "official.analytics"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// GetAnalyticsFilters retrieves analytics filters for all games
func (s *Service) GetAnalyticsFilters(ctx context.Context, req *GetAnalyticsFiltersRequest) (*GetAnalyticsFiltersResponse, error) {
	if items, ok, err := s.loadFiltersFromAnalyticsInstallation(ctx); err != nil {
		return nil, err
	} else if ok {
		filters := make([]AnalyticsFilter, 0, len(items))
		for _, item := range items {
			filters = append(filters, AnalyticsFilter{
				GameId:  item.GameId,
				Filters: item.Filters,
			})
		}
		return &GetAnalyticsFiltersResponse{
			Items: filters,
			Count: len(filters),
		}, nil
	}

	path := helper.ResolveAnalyticsFiltersPath(s.svcCtx.Config)

	if lock := s.svcCtx.AnalyticsFiltersLock; lock != nil {
		lock.RLock()
		defer lock.RUnlock()
	}

	data, err := helper.ReadAnalyticsFiltersFile(path)
	if err != nil {
		return nil, err
	}

	items, err := analytics.LoadAnalyticsFilters(data)
	if err != nil {
		return nil, err
	}

	filters := make([]AnalyticsFilter, 0, len(items))
	for _, item := range items {
		filters = append(filters, AnalyticsFilter{
			GameId:  item.GameId,
			Filters: item.Filters,
		})
	}

	return &GetAnalyticsFiltersResponse{
		Items: filters,
		Count: len(filters),
	}, nil
}

func (s *Service) loadFiltersFromAnalyticsInstallation(ctx context.Context) ([]analytics.AnalyticsFilters, bool, error) {
	if s == nil || s.svcCtx == nil || s.svcCtx.Extensions == nil || s.svcCtx.Extensions.Installation == nil {
		return nil, false, nil
	}
	items, _, err := s.svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
		ExtensionID: officialAnalyticsExtensionID,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		return nil, false, err
	}
	var activeConfig []byte
	for i := range items {
		item := items[i]
		if strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") ||
			strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled") {
			continue
		}
		activeConfig = bytes.TrimSpace(item.ConfigJSON)
		break
	}
	if len(activeConfig) == 0 {
		return nil, false, nil
	}
	cfg := map[string]any{}
	if err := json.Unmarshal(activeConfig, &cfg); err != nil {
		return nil, false, err
	}
	raw := cfg["filters"]
	if raw == nil {
		raw = cfg["analytics_filters"]
	}
	if raw == nil {
		return []analytics.AnalyticsFilters{}, true, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, false, err
	}
	filters := []analytics.AnalyticsFilters{}
	if err := json.Unmarshal(data, &filters); err != nil {
		return nil, false, err
	}
	return filters, true, nil
}

// UpdateMeta retrieves and returns agent metadata
func (s *Service) UpdateMeta(ctx context.Context, req *UpdateMetaRequest) (*UpdateMetaResponse, error) {
	store := s.svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	agents := make([]AgentSnapshot, 0, len(store.AgentsUnsafe()))
	for _, sess := range store.AgentsUnsafe() {
		if snapshot := utils.BuildOpsAgentSnapshot(sess); snapshot != nil {
			agents = append(agents, snapshot)
		}
	}

	return &UpdateMetaResponse{
		Agents:    agents,
		Count:     len(agents),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
