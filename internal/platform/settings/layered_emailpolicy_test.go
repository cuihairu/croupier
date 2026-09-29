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

// TestEmailPolicySnapshot_VerificationRequired 注册邮箱验证开关 L3 读写
// （#51c 第二批）：默认关；bool 键 L3 播种须存裸 JSON；AuthSnapshot 透出。
func TestEmailPolicySnapshot_VerificationRequired(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	snap := l.EmailPolicy()
	assert.False(t, snap.VerificationRequired)
	assert.Equal(t, "default", snap.Sources[KeyAuthEmailVerificationRequired])

	require.NoError(t, store.Set(context.Background(), KeyAuthEmailVerificationRequired, json.RawMessage(`true`), "tester"))
	l.Reload(context.Background(), store)

	snap = l.EmailPolicy()
	assert.True(t, snap.VerificationRequired)
	assert.Equal(t, "database", snap.Sources[KeyAuthEmailVerificationRequired])
	assert.True(t, l.AuthSnapshot().Email.VerificationRequired)
}
