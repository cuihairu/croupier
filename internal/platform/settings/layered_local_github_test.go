package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthProviderConfig_LocalAndGitHub local 开关默认启用、L3 显式关闭生效；
// GitHub 五键经 L3 覆盖组装进 AuthProvidersConfig。
func TestAuthProviderConfig_LocalAndGitHub(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	cfg := l.AuthProviderConfig()
	assert.NotNil(t, cfg.Local.Enabled)
	assert.True(t, cfg.Local.LocalEnabled(), "L3 未覆盖 = 默认启用")
	assert.False(t, cfg.GitHub.Enabled)

	set := func(key, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	set(KeyAuthLocalEnabled, `false`)
	set(KeyAuthGitHubEnabled, `true`)
	set(KeyAuthGitHubClientId, `"cid-1"`)
	set(KeyAuthGitHubClientSecret, `"cs-1"`)
	set(KeyAuthGitHubRedirectUrl, `"https://gm.example.com/api/v1/auth/github/callback"`)
	set(KeyAuthGitHubDefaultRoles, `"viewer,ops"`)
	set(KeyAuthGitHubSuccessURL, `"https://gm.example.com/oauth/done"`)
	l.Reload(context.Background(), store)

	cfg = l.AuthProviderConfig()
	assert.False(t, cfg.Local.LocalEnabled())
	assert.True(t, cfg.GitHub.Enabled)
	assert.Equal(t, "cid-1", cfg.GitHub.ClientID)
	assert.Equal(t, "cs-1", cfg.GitHub.ClientSecret)
	assert.Equal(t, "https://gm.example.com/api/v1/auth/github/callback", cfg.GitHub.RedirectURL)
	assert.Equal(t, []string{"viewer", "ops"}, cfg.GitHub.DefaultRoles)
	assert.Equal(t, "https://gm.example.com/oauth/done", cfg.GitHub.LoginSuccessURL)
}

// TestAuthSnapshot_LocalAndGitHub 快照读视图：local 默认启用（未覆盖）/覆盖后
// 带来源；github 块字段齐全且 secret 脱敏。
func TestAuthSnapshot_LocalAndGitHub(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	snap := l.AuthSnapshot()
	assert.True(t, snap.Local.Enabled, "L3 未覆盖 = 默认启用")
	assert.False(t, snap.Local.Overridden)
	assert.False(t, snap.GitHub.Enabled)

	set := func(key, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	set(KeyAuthLocalEnabled, `false`)
	set(KeyAuthGitHubEnabled, `true`)
	set(KeyAuthGitHubClientId, `"cid-1"`)
	set(KeyAuthGitHubClientSecret, `"cs-secret-9999"`)
	set(KeyAuthGitHubRedirectUrl, `"https://gm.example.com/api/v1/auth/github/callback"`)
	set(KeyAuthGitHubSuccessURL, `"/"`)
	l.Reload(context.Background(), store)

	snap = l.AuthSnapshot()
	assert.False(t, snap.Local.Enabled)
	assert.True(t, snap.Local.Overridden)
	assert.Equal(t, "database", snap.Local.Source)
	assert.True(t, snap.GitHub.Enabled)
	assert.Equal(t, "cid-1", snap.GitHub.Fields["clientId"])
	assert.Equal(t, "https://gm.example.com/api/v1/auth/github/callback", snap.GitHub.Fields["redirectUrl"])
	assert.Equal(t, "/", snap.GitHub.Fields["successUrl"])
	assert.True(t, snap.GitHub.SecretSet, "secret 已设置")
	assert.Equal(t, "****9999", snap.GitHub.SecretMasked, "secret 只回尾 4 位")
}
