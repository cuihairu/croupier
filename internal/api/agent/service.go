package agent

import (
	"context"
	"errors"
	"time"

	"github.com/cuihairu/croupier/internal/api/analytics"
	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// GetAnalyticsFilters retrieves analytics filters for all games
func (s *Service) GetAnalyticsFilters(ctx context.Context, req *GetAnalyticsFiltersRequest) (*GetAnalyticsFiltersResponse, error) {
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
