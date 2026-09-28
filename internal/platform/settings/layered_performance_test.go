package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPerformanceSettings_DefaultsAndL3 性能参数快照：默认全 0（不启用），
// L3 覆盖后值与来源同步更新（OPEN-ISSUES #53）。
func TestPerformanceSettings_DefaultsAndL3(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	snap := l.PerformanceSettings()
	assert.Equal(t, 0, snap.MaxCpuPct)
	assert.Equal(t, 0, snap.MaxConcurrent)
	assert.Equal(t, int64(0), snap.CacheSize)
	assert.Equal(t, "default", snap.Sources["maxCpuPct"])

	set := func(key string, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	set(KeyPerfMaxCpuPct, `85`)
	set(KeyPerfCacheSize, `268435456`)
	set(KeyPerfMaxThreadCount, `"16"`) // 字符串数值容错
	l.Reload(context.Background(), store)

	snap = l.PerformanceSettings()
	assert.Equal(t, 85, snap.MaxCpuPct)
	assert.Equal(t, "database", snap.Sources["maxCpuPct"])
	assert.Equal(t, int64(268435456), snap.CacheSize)
	assert.Equal(t, 16, snap.MaxThreadCount)
	assert.Equal(t, "default", snap.Sources["maxDiskPct"], "未覆盖键仍标 default")
}
