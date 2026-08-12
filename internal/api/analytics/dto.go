package analytics

import (
	"encoding/json"
	"strings"
	"time"
)

// Behavior analytics DTOs

type BehaviorRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type BehaviorResponse struct {
	TopActions []map[string]interface{} `json:"topActions"`
	UserFlows  map[string]interface{}   `json:"userFlows"`
	HeatMap    map[string]interface{}   `json:"heatMap"`
}

type BehaviorEventsRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	EventType string `json:"eventType"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Limit     int    `json:"limit"`
}

type BehaviorEvent struct {
	EventType string      `json:"eventType"`
	UserId    string      `json:"userId"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

type BehaviorEventsResponse struct {
	Items []BehaviorEvent `json:"items"`
	Total int             `json:"total"`
}

type BehaviorAdoptionRequest struct {
	GameId  string `json:"gameId" binding:"required"`
	Env     string `json:"env"`
	Feature string `json:"feature"`
}

type FeatureAdoption struct {
	Feature      string  `json:"feature"`
	Users        int     `json:"users"`
	AdoptionRate float64 `json:"adoptionRate"`
	Frequency    float64 `json:"frequency"`
}

type BehaviorAdoptionResponse struct {
	Features []FeatureAdoption `json:"features"`
}

type BehaviorAdoptionBreakdownRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	Feature   string `json:"feature" binding:"required"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type BehaviorAdoptionBreakdownResponse struct {
	BySegment map[string]interface{} `json:"bySegment"`
	ByTime    map[string]interface{} `json:"byTime"`
}

type BehaviorFunnelRequest struct {
	GameId    string   `json:"gameId" binding:"required"`
	Env       string   `json:"env"`
	StartDate string   `json:"startDate"`
	EndDate   string   `json:"endDate"`
	Steps     []string `json:"steps" binding:"required,min=1"`
}

type FunnelStep struct {
	Step           string  `json:"step"`
	Users          int     `json:"users"`
	ConversionRate float64 `json:"conversionRate"`
	DropOffRate    float64 `json:"dropOffRate"`
}

type BehaviorFunnelResponse struct {
	Steps []FunnelStep `json:"steps"`
}

type BehaviorPathsRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Depth     int    `json:"depth"`
}

type BehaviorPathsResponse struct {
	Paths map[string]interface{} `json:"paths"`
}

// Overview analytics DTOs

type OverviewRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type OverviewMetrics struct {
	DAU        int     `json:"dau"`
	MAU        int     `json:"mau"`
	NewUsers   int     `json:"newUsers"`
	Revenue    float64 `json:"revenue"`
	ARPU       float64 `json:"arpu"`
	ARPPU      float64 `json:"arppu"`
	PayingRate float64 `json:"payingRate"`
}

type OverviewResponse struct {
	Metrics OverviewMetrics        `json:"metrics"`
	Trends  map[string]interface{} `json:"trends"`
}

type RealtimeRequest struct {
	GameId string `json:"gameId" binding:"required"`
	Env    string `json:"env"`
}

type RealtimeMetrics struct {
	OnlineUsers    int         `json:"onlineUsers"`
	ActiveSessions int         `json:"activeSessions"`
	QPS            float64     `json:"qps"`
	AvgLatency     float64     `json:"avgLatency"`
	ErrorRate      float64     `json:"errorRate"`
	TopEvents      interface{} `json:"topEvents"`
}

type RealtimeResponse struct {
	RealtimeMetrics RealtimeMetrics `json:"realtimeMetrics"`
	Timestamp       string          `json:"timestamp"`
}

type RealtimeSeriesRequest struct {
	GameId   string `json:"gameId" binding:"required"`
	Env      string `json:"env"`
	Interval string `json:"interval"`
	Duration int    `json:"duration"`
}

type RealtimeSeriesResponse struct {
	Series map[string]interface{} `json:"series"`
}

type IngestRequest struct {
	GameId    string      `json:"gameId" binding:"required"`
	Env       string      `json:"env"`
	Events    interface{} `json:"events" binding:"required"`
	Timestamp string      `json:"timestamp"`
}

type IngestResponse struct {
	Accepted int    `json:"accepted"`
	Rejected int    `json:"rejected"`
	BatchId  string `json:"batchId"`
}

type FiltersGetRequest struct {
	GameId string `json:"gameId"`
}

type FiltersGetResponse struct {
	Items []AnalyticsFilters `json:"items"`
}

type FiltersUpdateRequest struct {
	GameId  string      `json:"gameId" binding:"required"`
	Filters interface{} `json:"filters" binding:"required"`
}

type AnalyticsFilters struct {
	GameId  string      `json:"gameId"`
	Filters interface{} `json:"filters"`
}

// Payments analytics DTOs

type PaymentsRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type PaymentsMetrics struct {
	Revenue        float64 `json:"revenue"`
	Transactions   int     `json:"transactions"`
	PayingUsers    int     `json:"payingUsers"`
	ARPU           float64 `json:"arpu"`
	ARPPU          float64 `json:"arppu"`
	ConversionRate float64 `json:"conversionRate"`
}

type PaymentsResponse struct {
	Metrics PaymentsMetrics        `json:"metrics"`
	Trends  map[string]interface{} `json:"trends"`
}

type PaymentsIngestRequest struct {
	GameId       string      `json:"gameId" binding:"required"`
	Env          string      `json:"env"`
	Transactions interface{} `json:"transactions" binding:"required"`
}

type PaymentsIngestResponse struct {
	Accepted int    `json:"accepted"`
	Rejected int    `json:"rejected"`
	BatchId  string `json:"batchId"`
}

type PaymentsProductTrendRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Limit     int    `json:"limit"`
}

type ProductTrend struct {
	ProductId   string  `json:"productId"`
	ProductName string  `json:"productName"`
	Revenue     float64 `json:"revenue"`
	Sales       int     `json:"sales"`
	Growth      float64 `json:"growth"`
}

type PaymentsProductTrendResponse struct {
	Items []ProductTrend `json:"items"`
}

type PaymentsSummaryRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	GroupBy   string `json:"groupBy"`
}

type PaymentsSummary struct {
	Date         string  `json:"date"`
	Revenue      float64 `json:"revenue"`
	Transactions int     `json:"transactions"`
	Users        int     `json:"users"`
}

type PaymentsSummaryResponse struct {
	Items []PaymentsSummary `json:"items"`
}

type PaymentsTransactionsRequest struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	Status    string `json:"status"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type PaymentTransaction struct {
	Id            string  `json:"id"`
	UserId        string  `json:"userId"`
	ProductId     string  `json:"productId"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Status        string  `json:"status"`
	PaymentMethod string  `json:"paymentMethod"`
	CreatedAt     string  `json:"createdAt"`
}

type PaymentsTransactionsResponse struct {
	Items []PaymentTransaction `json:"items"`
	Total int                  `json:"total"`
	Page  int                  `json:"page"`
	Size  int                  `json:"size"`
}

// Retention analytics DTOs

type RetentionRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	Cohort    string `json:"cohort"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type RetentionCohort struct {
	Cohort    string    `json:"cohort"`
	Users     int       `json:"users"`
	Retention []float64 `json:"retention"`
}

type RetentionResponse struct {
	Cohorts []RetentionCohort `json:"cohorts"`
}

type LevelsRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type LevelMetrics struct {
	LevelId        string  `json:"levelId"`
	Attempts       int     `json:"attempts"`
	Completions    int     `json:"completions"`
	CompletionRate float64 `json:"completionRate"`
	AvgDuration    float64 `json:"avgDuration"`
	AvgRetries     float64 `json:"avgRetries"`
}

type LevelsResponse struct {
	Levels []LevelMetrics `json:"levels"`
}

type LevelsEpisodesRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type EpisodeMetrics struct {
	EpisodeId      string  `json:"episodeId"`
	Players        int     `json:"players"`
	CompletionRate float64 `json:"completionRate"`
	AvgProgress    float64 `json:"avgProgress"`
}

type LevelsEpisodesResponse struct {
	Episodes []EpisodeMetrics `json:"episodes"`
}

type LevelsMapsRequest struct {
	GameId    string `json:"gameId" binding:"required"`
	Env       string `json:"env"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type MapMetrics struct {
	MapId      string      `json:"mapId"`
	HeatMap    interface{} `json:"heatMap"`
	DeathSpots interface{} `json:"deathSpots"`
}

type LevelsMapsResponse struct {
	Maps []MapMetrics `json:"maps"`
}

// Additional analytics types from types.go

type AnalyticsFiltersQuery struct {
}

type AnalyticsQuery struct {
	GameId    string `form:"gameId"`
	Env       string `form:"env"`
	StartDate string `form:"startDate"`
	EndDate   string `form:"endDate"`
}

// Analytics filters helper functions (moved from logic/utils to avoid import cycle)

type analyticsFiltersDocument struct {
	Items     []AnalyticsFilters `json:"items"`
	UpdatedAt string             `json:"updatedAt,omitempty"`
}

// LoadAnalyticsFilters reads the stored filters from a JSON file
func LoadAnalyticsFilters(data []byte) ([]AnalyticsFilters, error) {
	if len(data) == 0 {
		return []AnalyticsFilters{}, nil
	}
	var doc analyticsFiltersDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return normalizeAnalyticsFilters(doc.Items), nil
}

// SaveAnalyticsFilters converts filters to JSON for storage
func SaveAnalyticsFiltersJSON(items []AnalyticsFilters) ([]byte, error) {
	doc := analyticsFiltersDocument{
		Items:     normalizeAnalyticsFilters(items),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	return json.MarshalIndent(doc, "", "  ")
}

func normalizeAnalyticsFilters(items []AnalyticsFilters) []AnalyticsFilters {
	normalized := make([]AnalyticsFilters, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		gameID := strings.TrimSpace(item.GameId)
		if gameID == "" {
			continue
		}
		if _, ok := seen[gameID]; ok {
			continue
		}
		normalized = append(normalized, AnalyticsFilters{
			GameId:  gameID,
			Filters: item.Filters,
		})
		seen[gameID] = struct{}{}
	}
	return normalized
}
