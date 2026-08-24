package analytics

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ---------------------------------------------------------------------------
// Warehouse read-only API over the ClickHouse aggregate tables written by
// the analytics worker (docs/analytics/repair-plan.md T16):
//
//	analytics.minute_online (m, game_id, env, online)            SummingMergeTree
//	analytics.daily_users   (d, game_id, env, dau, new_users, …) ReplacingMergeTree
//	analytics.daily_revenue (d, game_id, env, revenue_cents, …)  ReplacingMergeTree
//
// The server opens the connection lazily from CLICKHOUSE_DSN (the same
// variable the worker uses). No DSN or unreachable ClickHouse maps to a
// 503 "分析仓库未启用" so deployments without the analytics profile keep
// working; the frontend renders a guidance state on 503.
// ---------------------------------------------------------------------------

var (
	errWarehouseDisabled = errors.New("warehouse disabled")
	errWarehouseQuery    = errors.New("warehouse query failed")
)

// warehouseRows is the subset of clickhouse driver rows used here; kept as
// a local interface so tests can stub query results.
type warehouseRows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
}

type warehouseConn interface {
	Query(ctx context.Context, query string, args ...any) (warehouseRows, error)
}

// warehouseConnect is swappable for tests.
var warehouseConnect = defaultWarehouseConnect

var warehouseOnce sync.Once
var warehouseOpened warehouseConn
var warehouseOpenErr error

func defaultWarehouseConnect() (warehouseConn, error) {
	warehouseOnce.Do(func() {
		dsn := os.Getenv("CLICKHOUSE_DSN")
		if dsn == "" {
			warehouseOpenErr = errWarehouseDisabled
			return
		}
		opts, err := clickhouse.ParseDSN(dsn)
		if err != nil {
			warehouseOpenErr = fmt.Errorf("parse clickhouse dsn: %w", err)
			return
		}
		conn, err := clickhouse.Open(opts)
		if err != nil {
			warehouseOpenErr = fmt.Errorf("clickhouse: %w", err)
			return
		}
		warehouseOpened = driverConnAdapter{conn}
	})
	return warehouseOpened, warehouseOpenErr
}

// driverConnAdapter bridges the clickhouse driver Conn to warehouseConn so
// the rest of the file depends on the minimal local interface (testable).
type driverConnAdapter struct{ conn chdriver.Conn }

func (a driverConnAdapter) Query(ctx context.Context, query string, args ...any) (warehouseRows, error) {
	return a.conn.Query(ctx, query, args...)
}

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

type WarehouseDAURequest struct {
	GameId string `form:"gameId"`
	Env    string `form:"env"`
	Days   int    `form:"days"` // lookback window, default 14, max 90
}

type WarehouseDAUPoint struct {
	Date     string `json:"date"`
	GameId   string `json:"gameId,omitempty"`
	Env      string `json:"env,omitempty"`
	DAU      uint64 `json:"dau"`
	NewUsers uint64 `json:"newUsers"`
}

type WarehouseDAUResponse struct {
	Points []WarehouseDAUPoint `json:"points"`
}

type WarehouseOnlineRequest struct {
	GameId  string `form:"gameId"`
	Env     string `form:"env"`
	Minutes int    `form:"minutes"` // lookback window, default 60, max 1440
}

type WarehouseOnlinePoint struct {
	Minute string `json:"minute"`
	Online uint64 `json:"online"`
}

type WarehouseOnlineResponse struct {
	Points []WarehouseOnlinePoint `json:"points"`
}

type WarehouseRevenueRequest struct {
	GameId string `form:"gameId"`
	Env    string `form:"env"`
	Days   int    `form:"days"` // lookback window, default 14, max 90
}

type WarehouseRevenuePoint struct {
	Date         string `json:"date"`
	GameId       string `json:"gameId,omitempty"`
	Env          string `json:"env,omitempty"`
	RevenueCents uint64 `json:"revenueCents"`
	RefundsCents uint64 `json:"refundsCents"`
	Failed       uint64 `json:"failed"`
}

type WarehouseRevenueResponse struct {
	Points []WarehouseRevenuePoint `json:"points"`
}

// ---------------------------------------------------------------------------
// Service methods
// ---------------------------------------------------------------------------

func (s *Service) WarehouseDAU(ctx context.Context, req *WarehouseDAURequest) (*WarehouseDAUResponse, error) {
	days := clampDays(req.Days, 14, 90)
	since := time.Now().UTC().AddDate(0, 0, -(days - 1)).Truncate(24 * time.Hour)

	rows, err := warehouseQuery(ctx, fmt.Sprintf(
		"SELECT d, game_id, env, dau, new_users FROM analytics.daily_users FINAL WHERE d >= ?%s ORDER BY d",
		warehouseScopeSuffix(req.GameId, req.Env),
	), append([]any{since}, warehouseScopeArgs(req.GameId, req.Env)...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []WarehouseDAUPoint{}
	for rows.Next() {
		var d time.Time
		var gameID, env string
		var dau, newUsers uint64
		if err := rows.Scan(&d, &gameID, &env, &dau, &newUsers); err != nil {
			return nil, fmt.Errorf("%w: scan daily_users: %v", errWarehouseQuery, err)
		}
		points = append(points, WarehouseDAUPoint{
			Date: d.Format("2006-01-02"), GameId: gameID, Env: env, DAU: dau, NewUsers: newUsers,
		})
	}
	return &WarehouseDAUResponse{Points: points}, nil
}

func (s *Service) WarehouseOnline(ctx context.Context, req *WarehouseOnlineRequest) (*WarehouseOnlineResponse, error) {
	minutes := clampInt(req.Minutes, 60, 1, 1440)
	since := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute).Truncate(time.Minute)

	// SummingMergeTree merges duplicate flushes lazily; SUM(online) is
	// correct regardless of merge state.
	rows, err := warehouseQuery(ctx, fmt.Sprintf(
		"SELECT m, SUM(online) FROM analytics.minute_online WHERE m >= ?%s GROUP BY m ORDER BY m",
		warehouseScopeSuffix(req.GameId, req.Env),
	), append([]any{since}, warehouseScopeArgs(req.GameId, req.Env)...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []WarehouseOnlinePoint{}
	for rows.Next() {
		var m time.Time
		var online uint64
		if err := rows.Scan(&m, &online); err != nil {
			return nil, fmt.Errorf("%w: scan minute_online: %v", errWarehouseQuery, err)
		}
		points = append(points, WarehouseOnlinePoint{Minute: m.UTC().Format(time.RFC3339), Online: online})
	}
	return &WarehouseOnlineResponse{Points: points}, nil
}

func (s *Service) WarehouseRevenue(ctx context.Context, req *WarehouseRevenueRequest) (*WarehouseRevenueResponse, error) {
	days := clampDays(req.Days, 14, 90)
	since := time.Now().UTC().AddDate(0, 0, -(days - 1)).Truncate(24 * time.Hour)

	rows, err := warehouseQuery(ctx, fmt.Sprintf(
		"SELECT d, game_id, env, revenue_cents, refunds_cents, failed FROM analytics.daily_revenue FINAL WHERE d >= ?%s ORDER BY d",
		warehouseScopeSuffix(req.GameId, req.Env),
	), append([]any{since}, warehouseScopeArgs(req.GameId, req.Env)...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []WarehouseRevenuePoint{}
	for rows.Next() {
		var d time.Time
		var gameID, env string
		var revenue, refunds, failed uint64
		if err := rows.Scan(&d, &gameID, &env, &revenue, &refunds, &failed); err != nil {
			return nil, fmt.Errorf("%w: scan daily_revenue: %v", errWarehouseQuery, err)
		}
		points = append(points, WarehouseRevenuePoint{
			Date: d.Format("2006-01-02"), GameId: gameID, Env: env,
			RevenueCents: revenue, RefundsCents: refunds, Failed: failed,
		})
	}
	return &WarehouseRevenueResponse{Points: points}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// warehouseQuery resolves the shared ClickHouse connection and runs one
// query. Disabled (no DSN) surfaces as errWarehouseDisabled → HTTP 503.
func warehouseQuery(ctx context.Context, query string, args ...any) (warehouseRows, error) {
	conn, err := warehouseConnect()
	if err != nil {
		if errors.Is(err, errWarehouseDisabled) {
			return nil, errWarehouseDisabled
		}
		return nil, fmt.Errorf("%w: %v", errWarehouseQuery, err)
	}
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errWarehouseQuery, err)
	}
	return rows, nil
}

func warehouseScopeSuffix(gameID, env string) string {
	suffix := ""
	if gameID != "" {
		suffix += " AND game_id = ?"
	}
	if env != "" {
		suffix += " AND env = ?"
	}
	return suffix
}

func warehouseScopeArgs(gameID, env string) []any {
	args := []any{}
	if gameID != "" {
		args = append(args, gameID)
	}
	if env != "" {
		args = append(args, env)
	}
	return args
}

func clampDays(v, def, max int) int {
	return clampInt(v, def, 1, max)
}

func clampInt(v, def, min, max int) int {
	if v == 0 {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
