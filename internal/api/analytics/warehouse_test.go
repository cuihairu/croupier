package analytics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRows replays a fixed set of scan targets per row.
type fakeRows struct {
	rows  [][]any
	index int
}

func (f *fakeRows) Next() bool { f.index++; return f.index <= len(f.rows) }
func (f *fakeRows) Scan(dest ...any) error {
	row := f.rows[f.index-1]
	for i, d := range dest {
		switch target := d.(type) {
		case *time.Time:
			*target = row[i].(time.Time)
		case *string:
			*target = row[i].(string)
		case *uint64:
			*target = row[i].(uint64)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}
func (f *fakeRows) Close() error { return nil }

// fakeConn captures the executed query and returns scripted rows.
type fakeConn struct {
	query    string
	args     []any
	rows     [][]any
	queryErr error
}

func (f *fakeConn) Query(_ context.Context, query string, args ...any) (warehouseRows, error) {
	f.query = query
	f.args = args
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return &fakeRows{rows: f.rows}, nil
}

func withFakeConn(t *testing.T, conn *fakeConn) {
	t.Helper()
	orig := warehouseConnect
	warehouseConnect = func() (warehouseConn, error) { return conn, nil }
	t.Cleanup(func() { warehouseConnect = orig })
}

func withDisabledWarehouse(t *testing.T) {
	t.Helper()
	orig := warehouseConnect
	warehouseConnect = func() (warehouseConn, error) { return nil, errWarehouseDisabled }
	t.Cleanup(func() { warehouseConnect = orig })
}

// warehouseScopeCtx 给仓库查询注入 current scope（GameDBMiddleware 的测试替身）。
func warehouseScopeCtx(gameID, env string) context.Context {
	return svc.WithGameScope(context.Background(), svc.GameScope{GameID: gameID, Env: env})
}

// withWarehouseScope 给 handler 级 gin.Context 注入 current scope。
func withWarehouseScope(c *gin.Context, gameID, env string) {
	c.Request = c.Request.WithContext(svc.WithGameScope(c.Request.Context(), svc.GameScope{GameID: gameID, Env: env}))
}

func TestWarehouseDAU_ScopeAndWindow(t *testing.T) {
	conn := &fakeConn{rows: [][]any{
		{time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC), "game-a", "prod", uint64(120), uint64(12)},
		{time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), "game-a", "prod", uint64(150), uint64(30)},
	}}
	withFakeConn(t, conn)

	svcAnalytics := NewService(nil)
	resp, err := svcAnalytics.WarehouseDAU(warehouseScopeCtx("game-a", "prod"), &WarehouseDAURequest{GameId: "game-a", Env: "prod", Days: 2})
	require.NoError(t, err)
	require.Len(t, resp.Points, 2)
	assert.Equal(t, "2026-08-23", resp.Points[0].Date)
	assert.Equal(t, uint64(120), resp.Points[0].DAU)
	assert.Equal(t, uint64(30), resp.Points[1].NewUsers)

	assert.Contains(t, conn.query, "FROM analytics.daily_users FINAL")
	assert.Contains(t, conn.query, "AND game_id = ?")
	assert.Contains(t, conn.query, "AND env = ?")
	require.Len(t, conn.args, 3)
	assert.Equal(t, "game-a", conn.args[1])
	assert.Equal(t, "prod", conn.args[2])
}

func TestWarehouseDAU_DefaultWindowUsesCurrentScope(t *testing.T) {
	conn := &fakeConn{rows: [][]any{}}
	withFakeConn(t, conn)

	// AUD-048：请求不传 game/env 时作用域来自 current scope，不得展开为无过滤。
	_, err := NewService(nil).WarehouseDAU(warehouseScopeCtx("game-a", "prod"), &WarehouseDAURequest{})
	require.NoError(t, err)
	assert.Contains(t, conn.query, "AND game_id = ?")
	assert.Contains(t, conn.query, "AND env = ?")
	require.Len(t, conn.args, 3) // since + game + env
	assert.Equal(t, "game-a", conn.args[1])
	assert.Equal(t, "prod", conn.args[2])
}

func TestWarehouseOnline_SumGroupByMinute(t *testing.T) {
	conn := &fakeConn{rows: [][]any{
		{time.Date(2026, 8, 24, 6, 0, 0, 0, time.UTC), uint64(7)},
	}}
	withFakeConn(t, conn)

	resp, err := NewService(nil).WarehouseOnline(warehouseScopeCtx("game-a", "prod"), &WarehouseOnlineRequest{GameId: "game-a", Minutes: 30})
	require.NoError(t, err)
	require.Len(t, resp.Points, 1)
	assert.Equal(t, uint64(7), resp.Points[0].Online)
	assert.True(t, strings.HasSuffix(resp.Points[0].Minute, "Z"))

	assert.Contains(t, conn.query, "FROM analytics.minute_online")
	assert.Contains(t, conn.query, "SUM(online)")
	assert.Contains(t, conn.query, "GROUP BY m")
}

func TestWarehouseRevenue_ScanFields(t *testing.T) {
	conn := &fakeConn{rows: [][]any{
		{time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), "game-a", "prod", uint64(9900), uint64(500), uint64(2)},
	}}
	withFakeConn(t, conn)

	resp, err := NewService(nil).WarehouseRevenue(warehouseScopeCtx("game-a", "prod"), &WarehouseRevenueRequest{Days: 7})
	require.NoError(t, err)
	require.Len(t, resp.Points, 1)
	assert.Equal(t, uint64(9900), resp.Points[0].RevenueCents)
	assert.Equal(t, uint64(500), resp.Points[0].RefundsCents)
	assert.Equal(t, uint64(2), resp.Points[0].Failed)
	assert.Contains(t, conn.query, "FROM analytics.daily_revenue FINAL")
}

func TestWarehouse_DisabledReturnsSentinel(t *testing.T) {
	withDisabledWarehouse(t)

	_, err := NewService(nil).WarehouseDAU(warehouseScopeCtx("game-a", "prod"), &WarehouseDAURequest{})
	assert.ErrorIs(t, err, errWarehouseDisabled)

	_, err = NewService(nil).WarehouseOnline(warehouseScopeCtx("game-a", "prod"), &WarehouseOnlineRequest{})
	assert.ErrorIs(t, err, errWarehouseDisabled)

	_, err = NewService(nil).WarehouseRevenue(warehouseScopeCtx("game-a", "prod"), &WarehouseRevenueRequest{})
	assert.ErrorIs(t, err, errWarehouseDisabled)
}

func TestWarehouse_QueryFailureWrapped(t *testing.T) {
	conn := &fakeConn{queryErr: errors.New("dial refused")}
	withFakeConn(t, conn)

	_, err := NewService(nil).WarehouseDAU(warehouseScopeCtx("game-a", "prod"), &WarehouseDAURequest{})
	require.Error(t, err)
	assert.ErrorIs(t, err, errWarehouseQuery)
}

func TestWarehouse_ClampHelpers(t *testing.T) {
	assert.Equal(t, 14, clampDays(0, 14, 90))
	assert.Equal(t, 1, clampDays(-3, 14, 90))
	assert.Equal(t, 90, clampDays(365, 14, 90))
	assert.Equal(t, 42, clampDays(42, 14, 90))
	assert.Equal(t, 60, clampInt(0, 60, 1, 1440))
	assert.Equal(t, 1440, clampInt(9999, 60, 1, 1440))
}
