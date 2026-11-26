package analytics

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errIterBroken = errors.New("connection reset while streaming rows")

// TestWarehouseIterationFailureIsNotASuccess pins DATA-044: a result stream
// that dies during iteration must surface as a warehouse query error in
// every reader that walks rows — never as a truncated response presented as
// a successful (possibly empty) payload.
func TestWarehouseIterationFailureIsNotASuccess(t *testing.T) {
	day := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)

	t.Run("dau: fails before any row", func(t *testing.T) {
		withFakeConn(t, &fakeConn{rows: [][]any{}, iterErr: errIterBroken})
		_, err := NewService(nil).WarehouseDAU(warehouseScopeCtx("game-a", "prod"), &WarehouseDAURequest{})
		require.ErrorIs(t, err, errWarehouseQuery)
		// the stream cause is carried in the message (inner errors are
		// attached with %v, matching the file's scan-error convention)
		assert.Contains(t, err.Error(), "connection reset")
	})

	t.Run("dau: fails after partial rows", func(t *testing.T) {
		withFakeConn(t, &fakeConn{rows: [][]any{
			{day, "game-a", "prod", uint64(5), uint64(1)},
		}, iterErr: errIterBroken})
		resp, err := NewService(nil).WarehouseDAU(warehouseScopeCtx("game-a", "prod"), &WarehouseDAURequest{})
		require.ErrorIs(t, err, errWarehouseQuery)
		assert.Nil(t, resp, "a truncated read must not be returned as success")
	})

	t.Run("online: fails after partial rows", func(t *testing.T) {
		withFakeConn(t, &fakeConn{rows: [][]any{
			{day, uint64(7)},
		}, iterErr: errIterBroken})
		_, err := NewService(nil).WarehouseOnline(warehouseScopeCtx("game-a", "prod"), &WarehouseOnlineRequest{})
		require.ErrorIs(t, err, errWarehouseQuery)
	})

	t.Run("revenue: fails before any row", func(t *testing.T) {
		withFakeConn(t, &fakeConn{rows: [][]any{}, iterErr: errIterBroken})
		_, err := NewService(nil).WarehouseRevenue(warehouseScopeCtx("game-a", "prod"), &WarehouseRevenueRequest{})
		require.ErrorIs(t, err, errWarehouseQuery)
	})

	t.Run("sessions: fails on the end scan", func(t *testing.T) {
		conn := &routingConn{
			ends:   &fakeRows{rows: [][]any{}, iterErr: errIterBroken},
			starts: &fakeRows{rows: [][]any{}},
		}
		withRoutingConn(t, conn)
		_, err := NewService(nil).WarehouseSessions(warehouseScopeCtx("game-a", "prod"), &WarehouseSessionsRequest{})
		require.ErrorIs(t, err, errWarehouseQuery)
	})

	t.Run("sessions: fails on the start scan", func(t *testing.T) {
		conn := &routingConn{
			ends:   &fakeRows{rows: [][]any{}},
			starts: &fakeRows{rows: [][]any{}, iterErr: errIterBroken},
		}
		withRoutingConn(t, conn)
		_, err := NewService(nil).WarehouseSessions(warehouseScopeCtx("game-a", "prod"), &WarehouseSessionsRequest{})
		require.ErrorIs(t, err, errWarehouseQuery)
	})

	t.Run("iteration error maps to 500 at the handler", func(t *testing.T) {
		withFakeConn(t, &fakeConn{rows: [][]any{}, iterErr: errIterBroken})
		handler := NewHandler(NewService(nil), config.SSEConfig{})
		ctx, rec := newCoverageGinContext(http.MethodGet, "/api/v1/analytics/warehouse/dau", "")
		withWarehouseScope(ctx, "game-a", "prod")
		handler.WarehouseDAU(ctx)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("normal full read and empty read stay successes", func(t *testing.T) {
		withFakeConn(t, &fakeConn{rows: [][]any{
			{day, "game-a", "prod", uint64(5), uint64(1)},
		}})
		resp, err := NewService(nil).WarehouseDAU(warehouseScopeCtx("game-a", "prod"), &WarehouseDAURequest{})
		require.NoError(t, err)
		assert.Len(t, resp.Points, 1)

		withFakeConn(t, &fakeConn{rows: [][]any{}})
		resp, err = NewService(nil).WarehouseDAU(warehouseScopeCtx("game-a", "prod"), &WarehouseDAURequest{})
		require.NoError(t, err)
		assert.Empty(t, resp.Points)
	})
}
