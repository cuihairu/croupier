// Coverage targets (group E):
//   - the SaveAnalyticsFiltersJSON failure branch of filtersUpdate (overview.go:324-326)
//   - the empty CLICKHOUSE_DSN branch of defaultWarehouseConnect (warehouse.go:66-69)
package analytics

import (
	"context"
	"math"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Filters is interface{}: injecting NaN makes json.Marshal always fail (unsupported
// value), covering the serialization error in the filtersUpdate file branch. The load
// stage uses the default empty document ({"items":[]} when the file does not exist),
// ensuring execution reaches the save stage.
func TestFiltersUpdate_SaveJSONMarshalFailureGroupE(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")

	_, err := filtersUpdate(context.Background(), &svc.ServiceContext{
		Config:               config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: path}},
		AnalyticsFiltersLock: &sync.RWMutex{},
	}, &FiltersUpdateRequest{GameId: "g", Filters: math.NaN()})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported value")
}

// DSN not configured -> errWarehouseDisabled (not a dial error), covering the empty-DSN
// branch inside once.
func TestDefaultWarehouseConnect_EmptyDSNGroupE(t *testing.T) {
	resetWarehouseConnectState()
	t.Cleanup(resetWarehouseConnectState)
	t.Setenv("CLICKHOUSE_DSN", "")

	conn, err := defaultWarehouseConnect()
	require.Error(t, err)
	require.ErrorIs(t, err, errWarehouseDisabled)
	assert.Nil(t, conn)
}
