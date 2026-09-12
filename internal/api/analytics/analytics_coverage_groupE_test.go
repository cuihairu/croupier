// 覆盖目标（组 E）：
//   - filtersUpdate 的 SaveAnalyticsFiltersJSON 失败分支（overview.go:324-326）
//   - defaultWarehouseConnect 的 CLICKHOUSE_DSN 为空分支（warehouse.go:66-69）
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

// Filters 为 interface{}：注入 NaN 使 json.Marshal 必败（unsupported
// value），覆盖 filtersUpdate 文件分支中的序列化错误。加载阶段使用默认
// 空文档（文件不存在时返回 {"items":[]}），保证能走到保存阶段。
func TestFiltersUpdate_SaveJSONMarshalFailureGroupE(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")

	_, err := filtersUpdate(context.Background(), &svc.ServiceContext{
		Config:               config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: path}},
		AnalyticsFiltersLock: &sync.RWMutex{},
	}, &FiltersUpdateRequest{GameId: "g", Filters: math.NaN()})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported value")
}

// DSN 未配置 → errWarehouseDisabled（而非拨号错误），覆盖 once 内的空 DSN 分支。
func TestDefaultWarehouseConnect_EmptyDSNGroupE(t *testing.T) {
	resetWarehouseConnectState()
	t.Cleanup(resetWarehouseConnectState)
	t.Setenv("CLICKHOUSE_DSN", "")

	conn, err := defaultWarehouseConnect()
	require.Error(t, err)
	require.ErrorIs(t, err, errWarehouseDisabled)
	assert.Nil(t, conn)
}
