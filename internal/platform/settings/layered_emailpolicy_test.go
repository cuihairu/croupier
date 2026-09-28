package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmailPolicySnapshot 注册邮箱策略键 L3 读写（OPEN-ISSUES #51c）：
// 默认空串 + 关；AuthSnapshot.Email 同步透出。
func TestEmailPolicySnapshot(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	snap := l.EmailPolicy()
	assert.Equal(t, "", snap.DomainWhitelist)
	assert.False(t, snap.AliasRestriction)
	assert.Equal(t, "default", snap.Sources[KeyAuthEmailDomainWhitelist])

	require.NoError(t, store.Set(context.Background(), KeyAuthEmailDomainWhitelist, json.RawMessage(`"example.com, foo.io"`), "tester"))
	require.NoError(t, store.Set(context.Background(), KeyAuthEmailAliasRestriction, json.RawMessage(`true`), "tester"))
	l.Reload(context.Background(), store)

	snap = l.EmailPolicy()
	assert.Equal(t, "example.com, foo.io", snap.DomainWhitelist)
	assert.True(t, snap.AliasRestriction)
	assert.Equal(t, "database", snap.Sources[KeyAuthEmailDomainWhitelist])
	assert.Equal(t, "database", snap.Sources[KeyAuthEmailAliasRestriction])

	auth := l.AuthSnapshot()
	assert.Equal(t, "example.com, foo.io", auth.Email.DomainWhitelist)
	assert.True(t, auth.Email.AliasRestriction)
}
