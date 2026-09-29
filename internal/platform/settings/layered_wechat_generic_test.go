package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthProviderConfig_WeChatAndGenericOAuth 微信/自定义 OAuth 键经 L3 覆盖
// 组装进 AuthProvidersConfig（#51 第三批）。
func TestAuthProviderConfig_WeChatAndGenericOAuth(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	cfg := l.AuthProviderConfig()
	assert.False(t, cfg.WeChat.Enabled)
	assert.False(t, cfg.GenericOAuth.Enabled)

	set := func(key, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	set(KeyAuthWeChatEnabled, `true`)
	set(KeyAuthWeChatAppId, `"wx-app"`)
	set(KeyAuthWeChatAppSecret, `"wx-secret"`)
	set(KeyAuthWeChatRedirectUrl, `"https://gm.example.com/api/v1/auth/wechat/callback"`)
	set(KeyAuthWeChatDefaultRoles, `"viewer"`)
	set(KeyAuthWeChatSuccessURL, `"https://gm.example.com/wx-done"`)
	set(KeyAuthGenericOAuthEnabled, `true`)
	set(KeyAuthGenericOAuthClientId, `"cid"`)
	set(KeyAuthGenericOAuthClientSecret, `"cs"`)
	set(KeyAuthGenericOAuthRedirectUrl, `"https://gm.example.com/api/v1/auth/generic/callback"`)
	set(KeyAuthGenericOAuthAuthUrl, `"https://idp/authorize"`)
	set(KeyAuthGenericOAuthTokenUrl, `"https://idp/token"`)
	set(KeyAuthGenericOAuthUserInfoUrl, `"https://idp/userinfo"`)
	set(KeyAuthGenericOAuthScopes, `"openid,profile"`)
	set(KeyAuthGenericOAuthUsernameField, `"preferred_username"`)
	l.Reload(context.Background(), store)

	cfg = l.AuthProviderConfig()
	assert.True(t, cfg.WeChat.Enabled)
	assert.Equal(t, "wx-app", cfg.WeChat.AppID)
	assert.Equal(t, "wx-secret", cfg.WeChat.AppSecret)
	assert.Equal(t, "https://gm.example.com/api/v1/auth/wechat/callback", cfg.WeChat.RedirectURL)
	assert.Equal(t, []string{"viewer"}, cfg.WeChat.DefaultRoles)
	assert.Equal(t, "https://gm.example.com/wx-done", cfg.WeChat.LoginSuccessURL)

	assert.True(t, cfg.GenericOAuth.Enabled)
	assert.Equal(t, "cid", cfg.GenericOAuth.ClientID)
	assert.Equal(t, "https://idp/authorize", cfg.GenericOAuth.AuthURL)
	assert.Equal(t, "https://idp/token", cfg.GenericOAuth.TokenURL)
	assert.Equal(t, "https://idp/userinfo", cfg.GenericOAuth.UserInfoURL)
	assert.Equal(t, "openid,profile", cfg.GenericOAuth.Scopes)
	assert.Equal(t, "preferred_username", cfg.GenericOAuth.UsernameField)
}

// TestAuthSnapshot_WeChatAndGenericOAuth 快照读视图：wechat/genericoauth 块
// 字段齐全、secret 脱敏只回尾 4 位。
func TestAuthSnapshot_WeChatAndGenericOAuth(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	snap := l.AuthSnapshot()
	assert.False(t, snap.WeChat.Enabled)
	assert.False(t, snap.GenericOAuth.Enabled)

	set := func(key, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	set(KeyAuthWeChatEnabled, `true`)
	set(KeyAuthWeChatAppId, `"wx-app"`)
	set(KeyAuthWeChatAppSecret, `"wx-secret-4321"`)
	set(KeyAuthWeChatRedirectUrl, `"https://gm.example.com/api/v1/auth/wechat/callback"`)
	set(KeyAuthGenericOAuthEnabled, `true`)
	set(KeyAuthGenericOAuthClientId, `"cid-9"`)
	set(KeyAuthGenericOAuthClientSecret, `"cs-secret-8888"`)
	set(KeyAuthGenericOAuthAuthUrl, `"https://idp/authorize"`)
	set(KeyAuthGenericOAuthScopes, `"openid"`)
	l.Reload(context.Background(), store)

	snap = l.AuthSnapshot()
	assert.True(t, snap.WeChat.Enabled)
	assert.Equal(t, "wx-app", snap.WeChat.Fields["appId"])
	assert.Equal(t, "https://gm.example.com/api/v1/auth/wechat/callback", snap.WeChat.Fields["redirectUrl"])
	assert.True(t, snap.WeChat.SecretSet)
	assert.Equal(t, "****4321", snap.WeChat.SecretMasked)

	assert.True(t, snap.GenericOAuth.Enabled)
	assert.Equal(t, "cid-9", snap.GenericOAuth.Fields["clientId"])
	assert.Equal(t, "https://idp/authorize", snap.GenericOAuth.Fields["authUrl"])
	assert.Equal(t, "openid", snap.GenericOAuth.Fields["scopes"])
	assert.True(t, snap.GenericOAuth.SecretSet)
	assert.Equal(t, "****8888", snap.GenericOAuth.SecretMasked)
}
