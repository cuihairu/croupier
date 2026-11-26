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
	// Sample discloses how much of the query window was aggregated: the
	// authoritative matching event count, the scanned count, and whether the
	// result is a bounded partial sample (sampled=true means coverage is
	// "partial" and the figures describe only scannedEvents rows).
	Sample map[string]interface{} `json:"sample"`
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
	Sample    map[string]interface{} `json:"sample"`
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
	Steps  []FunnelStep           `json:"steps"`
	Sample map[string]interface{} `json:"sample"`
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
	WAU        int     `json:"wau"`
	MAU        int     `json:"mau"`
	NewUsers   int     `json:"newUsers"`
	Revenue    float64 `json:"revenue"`
	ARPU       float64 `json:"arpu"`
	ARPPU      float64 `json:"arppu"`
	PayingRate float64 `json:"payingRate"`
	// RetentionD1/D7/D30 are computed from behavior events, not the empty
	// cohort table. A nil pointer means the metric was never requested.
	RetentionD1  *RetentionMetric `json:"retentionD1"`
	RetentionD7  *RetentionMetric `json:"retentionD7"`
	RetentionD30 *RetentionMetric `json:"retentionD30"`
}

// RetentionMetric is one N-day retention figure with an explicit computation
// status, so a real zero, a cohort that has not matured yet and an absent
// data source are distinguishable on every client (DATA-027).
type RetentionMetric struct {
	// Value is the pooled retention percentage (0-100). Null unless status
	// is ok; 0 is a real zero.
	Value *float64 `json:"value"`
	// Status: ok | pending | no_source.
	Status string `json:"status"`
	// Cohorts is the number of pooled mature cohort days behind value.
	Cohorts int `json:"cohorts"`
	// Sampled marks a distinct-pair scan truncated at the row cap.
	Sampled bool `json:"sampled"`
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
	OnlineUsers     int         `json:"onlineUsers"`
	ActiveSessions  int         `json:"activeSessions"`
	EventsPerMinute float64     `json:"eventsPerMinute"`
	QPS             float64     `json:"qps"`
	AvgLatency      float64     `json:"avgLatency"`
	ErrorRate       float64     `json:"errorRate"`
	TopEvents       interface{} `json:"topEvents"`
}

// RealtimeSource discloses where each realtime figure comes from, so consumers
// know onlineUsers is a heartbeat-based liveness proxy rather than a
// concurrent socket count (no player-session table exists yet).
type RealtimeSource struct {
	OnlineUsers     string `json:"onlineUsers"`
	ActiveSessions  string `json:"activeSessions"`
	EventsPerMinute string `json:"eventsPerMinute"`
	WindowSeconds   int    `json:"windowSeconds"`
}

type RealtimeResponse struct {
	RealtimeMetrics RealtimeMetrics `json:"realtimeMetrics"`
	Source          RealtimeSource  `json:"source"`
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
	Sample map[string]interface{} `json:"sample,omitempty"`
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
	Levels []LevelMetrics         `json:"levels"`
	Sample map[string]interface{} `json:"sample"`
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
	Episodes []EpisodeMetrics       `json:"episodes"`
	Sample   map[string]interface{} `json:"sample"`
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
	Maps   []MapMetrics           `json:"maps"`
	Sample map[string]interface{} `json:"sample"`
}

// Additional analytics types from types.go

type AnalyticsFiltersQuery struct {
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
