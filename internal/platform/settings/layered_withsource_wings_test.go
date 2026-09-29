package settings

// 覆盖率巡检第十二轮（wt-api）：layered.go 残余 7 块收口——
// getBoolWithSource 2 块（L3 raw 非 bool 的解析失败翼、L2 命中翼）、
// getIntWithSource 5 块（非法键/nil 接收者翼、raw 既非数字也非数字串
// 的解析失败翼、L2 命中翼）。
// L2 命中翼口径说明：resolveL2 目前只产出字符串与五域布尔键，整型键
// 无生产方——getIntWithSource 的 L2 翼以白盒直构锁定层契约（未来 L2
// 产出整型键时行为已定）；getBoolWithSource 的 L2 翼经
// ConfigInput.FeatureFlags 真实产出触达。

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBoolWithSource_Wings(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{
		FeatureFlags: map[string]bool{"dev": false},
	}, store)

	// L2 命中：features.dev 经 ConfigInput 落 L2，bool 可解 → config 来源
	v, src, found := l.getBoolWithSource(featureKey("dev"), true)
	assert.False(t, v, "L2 显式 false 应压过默认值")
	assert.Equal(t, "config", src)
	assert.True(t, found)

	// L3 raw 非 bool（字符串形态）→ 解析失败翼；该键无 L2 值 → 回落 default
	require.NoError(t, store.Set(context.Background(), KeySecurityMFARequired, json.RawMessage(`"yes"`), "t"))
	l.Reload(context.Background(), store)
	v, src, found = l.getBoolWithSource(KeySecurityMFARequired, false)
	assert.False(t, v)
	assert.Equal(t, "default", src)
	assert.False(t, found)
}

func TestGetIntWithSource_Wings(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	// 非法键 → (def, "default")
	n, src := l.getIntWithSource("bogus.key", 7)
	assert.Equal(t, int64(7), n)
	assert.Equal(t, "default", src)

	// L3 raw 既非数字也非数字串（布尔形态，两段 Unmarshal 均失败）→ 回落 default
	require.NoError(t, store.Set(context.Background(), KeyPerfMaxCpuPct, json.RawMessage(`true`), "t"))
	l.Reload(context.Background(), store)
	n, src = l.getIntWithSource(KeyPerfMaxCpuPct, 33)
	assert.Equal(t, int64(33), n)
	assert.Equal(t, "default", src)

	// L2 命中（白盒直构：resolveL2 现无整型键生产方，锁层契约）
	direct := &Layered{l2Values: map[string]json.RawMessage{
		KeyPerfMaxCpuPct: json.RawMessage(`42`),
	}}
	n, src = direct.getIntWithSource(KeyPerfMaxCpuPct, 0)
	assert.Equal(t, int64(42), n)
	assert.Equal(t, "config", src)
}
