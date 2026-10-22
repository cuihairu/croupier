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

// TestPerformanceSettings_IntSafeOverflow 整型收窄边界：配置值超出 int32
// 可表示范围时回退默认 0（不静默截断——32 位平台上 int(v) 会绕回，CodeQL
// go/incorrect-integer-conversion 修复的行为锚定）；int32 边界内原值保留。
func TestPerformanceSettings_IntSafeOverflow(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	set := func(key string, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	set(KeyPerfMaxCpuPct, `8589934592`)      // 2^33：int64 内、int32 外 → 回退 0
	set(KeyPerfMaxMemoryPct, `-2147483649`)  // 负向越界（int32 下边界外 1）→ 回退 0
	set(KeyPerfMaxThreadCount, `2147483647`) // int32 上边界 → 原值保留
	l.Reload(context.Background(), store)

	snap := l.PerformanceSettings()
	assert.Equal(t, 0, snap.MaxCpuPct, "超 int32 的配置回退默认，不截断")
	assert.Equal(t, 0, snap.MaxMemoryPct, "负向越界同样回退默认")
	assert.Equal(t, 2147483647, snap.MaxThreadCount)
	assert.Equal(t, "database", snap.Sources["maxCpuPct"], "来源仍标 database（值来自库、被收窄）")
}
