package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthProviderConfig_Register 自助注册键：默认关；L3 覆盖开 + 默认角色。
func TestAuthProviderConfig_Register(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	cfg := l.AuthProviderConfig()
	assert.False(t, cfg.Register.Enabled, "注册默认关闭")
	assert.Nil(t, cfg.Register.DefaultRoles)

	set := func(key, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	set(KeyAuthRegisterEnabled, `true`)
	set(KeyAuthRegisterDefaultRoles, `"viewer,ops"`)
	l.Reload(context.Background(), store)

	cfg = l.AuthProviderConfig()
	assert.True(t, cfg.Register.Enabled)
	assert.Equal(t, []string{"viewer", "ops"}, cfg.Register.DefaultRoles)

	snap := l.AuthSnapshot()
	assert.True(t, snap.Register.Enabled)
	assert.Equal(t, "viewer,ops", snap.Register.Fields["defaultRoles"])
}
