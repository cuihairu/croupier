package analytics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// Behavior analytics methods

func (s *Service) Behavior(ctx context.Context, req *BehaviorRequest) (*BehaviorResponse, error) {
	return behaviorAnalytics(ctx, s.svcCtx, req)
}

func (s *Service) BehaviorEvents(ctx context.Context, req *BehaviorEventsRequest) (*BehaviorEventsResponse, error) {
	return behaviorEvents(ctx, s.svcCtx, req)
}

func (s *Service) BehaviorAdoption(ctx context.Context, req *BehaviorAdoptionRequest) (*BehaviorAdoptionResponse, error) {
	return behaviorAdoption(ctx, s.svcCtx, req)
}

func (s *Service) BehaviorAdoptionBreakdown(ctx context.Context, req *BehaviorAdoptionBreakdownRequest) (*BehaviorAdoptionBreakdownResponse, error) {
	return behaviorAdoptionBreakdown(ctx, s.svcCtx, req)
}

func (s *Service) BehaviorFunnel(ctx context.Context, req *BehaviorFunnelRequest) (*BehaviorFunnelResponse, error) {
	return behaviorFunnel(ctx, s.svcCtx, req)
}

func (s *Service) BehaviorPaths(ctx context.Context, req *BehaviorPathsRequest) (*BehaviorPathsResponse, error) {
	return behaviorPaths(ctx, s.svcCtx, req)
}

// Overview analytics methods

func (s *Service) Overview(ctx context.Context, req *OverviewRequest) (*OverviewResponse, error) {
	return overview(ctx, s.svcCtx, req)
}

func (s *Service) Realtime(ctx context.Context, req *RealtimeRequest) (*RealtimeResponse, error) {
	return realtime(ctx, s.svcCtx, req)
}

func (s *Service) RealtimeSeries(ctx context.Context, req *RealtimeSeriesRequest) (*RealtimeSeriesResponse, error) {
	return realtimeSeries(ctx, s.svcCtx, req)
}

func (s *Service) Ingest(ctx context.Context, req *IngestRequest) (*IngestResponse, error) {
	return ingest(ctx, s.svcCtx, req)
}

func (s *Service) FiltersGet(ctx context.Context, req *FiltersGetRequest) (*FiltersGetResponse, error) {
	return filtersGet(ctx, s.svcCtx, req)
}

func (s *Service) FiltersUpdate(ctx context.Context, req *FiltersUpdateRequest) (*FiltersGetResponse, error) {
	return filtersUpdate(ctx, s.svcCtx, req)
}

// Payments analytics methods

func (s *Service) Payments(ctx context.Context, req *PaymentsRequest) (*PaymentsResponse, error) {
	return payments(ctx, s.svcCtx, req)
}

func (s *Service) PaymentsIngest(ctx context.Context, req *PaymentsIngestRequest) (*PaymentsIngestResponse, error) {
	return paymentsIngest(ctx, s.svcCtx, req)
}

func (s *Service) PaymentsProductTrend(ctx context.Context, req *PaymentsProductTrendRequest) (*PaymentsProductTrendResponse, error) {
	return paymentsProductTrend(ctx, s.svcCtx, req)
}

func (s *Service) PaymentsSummary(ctx context.Context, req *PaymentsSummaryRequest) (*PaymentsSummaryResponse, error) {
	return paymentsSummary(ctx, s.svcCtx, req)
}

func (s *Service) PaymentsTransactions(ctx context.Context, req *PaymentsTransactionsRequest) (*PaymentsTransactionsResponse, error) {
	return paymentsTransactions(ctx, s.svcCtx, req)
}

// Retention analytics methods

func (s *Service) Retention(ctx context.Context, req *RetentionRequest) (*RetentionResponse, error) {
	return retention(ctx, s.svcCtx, req)
}

func (s *Service) Levels(ctx context.Context, req *LevelsRequest) (*LevelsResponse, error) {
	return levels(ctx, s.svcCtx, req)
}

func (s *Service) LevelsEpisodes(ctx context.Context, req *LevelsEpisodesRequest) (*LevelsEpisodesResponse, error) {
	return levelsEpisodes(ctx, s.svcCtx, req)
}

func (s *Service) LevelsMaps(ctx context.Context, req *LevelsMapsRequest) (*LevelsMapsResponse, error) {
	return levelsMaps(ctx, s.svcCtx, req)
}

// Helper functions shared across sub-services

func safeDivide(num float64, denom float64) float64 {
	if denom == 0 {
		return 0
	}
	return num / denom
}

func resolveRange(startRaw, endRaw string, fallbackDays int) (time.Time, time.Time, error) {
	start, end, err := helper.NormalizeDateRange(startRaw, endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	now := time.Now().UTC()
	if end.IsZero() {
		end = now
	}
	if start.IsZero() {
		if fallbackDays <= 0 {
			fallbackDays = 7
		}
		start = end.Add(-time.Duration(fallbackDays) * 24 * time.Hour)
	}
	return start, end, nil
}

func aggregateAgentMetrics(store *reg.Store, gameID, env string) (avgLatency float64, errorRate float64) {
	if store == nil {
		return 0, 0
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	var (
		latencySum float64
		latencyCnt int
		errorSum   float64
		errorCnt   int
	)

	for _, agent := range store.AgentsUnsafe() {
		if agent == nil {
			continue
		}
		if trimString(gameID) != "" && agent.GameID != trimString(gameID) {
			continue
		}
		if trimString(env) != "" && agent.Env != trimString(env) {
			continue
		}

		if v, ok := parseFloat(agent.Labels["stats.avg_latency_ms"]); ok {
			latencySum += v
			latencyCnt++
		}
		if v, ok := parseFloat(agent.Labels["stats.error_rate"]); ok {
			errorSum += v
			errorCnt++
		}
	}

	if latencyCnt > 0 {
		avgLatency = latencySum / float64(latencyCnt)
	}
	if errorCnt > 0 {
		errorRate = errorSum / float64(errorCnt)
	}
	return avgLatency, errorRate
}

func trimString(s string) string {
	return strings.TrimSpace(s)
}

func parseFloat(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	var f float64
	_, err := fmt.Sscanf(raw, "%f", &f)
	if err != nil {
		return 0, false
	}
	return f, true
}

func loadBehaviorEvents(ctx context.Context, svcCtx *svc.ServiceContext, gameID, env string, start, end time.Time, eventTypes []string, pageSize int) ([]model.BehaviorEvent, error) {
	if svcCtx == nil || svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if pageSize <= 0 {
		pageSize = 3000
	}
	appendEvents := func(eventType string) ([]model.BehaviorEvent, error) {
		opts := model.BehaviorEventOptions{
			PaginationOptions: model.NewPagination(1, pageSize),
			GameID:    trimString(gameID),
			Env:       trimString(env),
			EventType: trimString(eventType),
			StartTime: start,
			EndTime:   end,
		}
		events, _, err := svcCtx.BehaviorModel.ListEvents(ctx, opts)
		if err != nil {
			return nil, err
		}
		return events, nil
	}

	var all []model.BehaviorEvent
	if len(eventTypes) == 0 {
		events, err := appendEvents("")
		if err != nil {
			return nil, err
		}
		all = append(all, events...)
		return all, nil
	}

	for _, evt := range eventTypes {
		events, err := appendEvents(evt)
		if err != nil {
			return nil, err
		}
		all = append(all, events...)
	}
	return all, nil
}
