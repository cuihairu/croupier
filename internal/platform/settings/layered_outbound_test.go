package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOutboundSnapshot sec.* 四键 L3 读写（OPEN-ISSUES #56）：默认全关
// （清单空串 + 不拦截），覆盖后 OutboundSnapshot 与来源标注同步透出。
func TestOutboundSnapshot(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	snap := l.OutboundSnapshot()
	assert.Equal(t, "", snap.AllowPorts)
	assert.Equal(t, "", snap.AllowIPs)
	assert.Equal(t, "", snap.DomainFilter)
	assert.False(t, snap.SSRFProtection)
	assert.Equal(t, "default", snap.Sources[KeySecAllowPorts])

	set := func(key string, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	set(KeySecAllowPorts, `"443,8080"`)
	set(KeySecAllowIPs, `"10.0.0.0/8,127.0.0.1"`)
	set(KeySecDomainFilter, `"example.com, foo.io"`)
	set(KeySecSSRFProtection, `true`)
	// net.* 出站调用策略三键（OPEN-ISSUES #57）
	set(KeyNetRequestTimeoutMs, `8000`)
	set(KeyNetMaxRetries, `3`)
	set(KeyNetRetryBackoffMs, `200`)
	l.Reload(context.Background(), store)

	snap = l.OutboundSnapshot()
	assert.Equal(t, "443,8080", snap.AllowPorts)
	assert.Equal(t, "10.0.0.0/8,127.0.0.1", snap.AllowIPs)
	assert.Equal(t, "example.com, foo.io", snap.DomainFilter)
	assert.True(t, snap.SSRFProtection)
	assert.Equal(t, "database", snap.Sources[KeySecAllowPorts])
	assert.Equal(t, "database", snap.Sources[KeySecSSRFProtection])
	assert.Equal(t, 8000, snap.RequestTimeoutMs)
	assert.Equal(t, 3, snap.MaxRetries)
	assert.Equal(t, 200, snap.RetryBackoffMs)
	assert.Equal(t, "database", snap.Sources[KeyNetMaxRetries])
}
