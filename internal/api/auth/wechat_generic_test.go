package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/security/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRefreshIdentityProviders_WeChatMissingConfig 微信开启但凭证缺失 → 拒绝（保存端回滚）。
func TestRefreshIdentityProviders_WeChatMissingConfig(t *testing.T) {
	_, err := refreshWith(t, config.AuthProvidersConfig{
		WeChat: config.WeChatProviderConfig{Enabled: true},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.providers.wechat")
}

// TestRefreshIdentityProviders_GenericOAuthMissingConfig 自定义 OAuth 开启但
// 端点缺失 → 拒绝（保存端回滚）。
func TestRefreshIdentityProviders_GenericOAuthMissingConfig(t *testing.T) {
	_, err := refreshWith(t, config.AuthProvidersConfig{
		GenericOAuth: config.GenericOAuthProviderConfig{Enabled: true, ClientID: "cid", ClientSecret: "cs"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.providers.genericoauth")
}

// TestRefreshIdentityProviders_LocalOffWithWeChatAllowed local 关 + 微信在 → 放行，
// 扫码授权 URL 形态正确（qrconnect + #wechat_redirect）。
func TestRefreshIdentityProviders_LocalOffWithWeChatAllowed(t *testing.T) {
	svc, err := refreshWith(t, config.AuthProvidersConfig{
		Local: config.LocalProviderConfig{Enabled: localOff()},
		WeChat: config.WeChatProviderConfig{
			Enabled:         true,
			AppID:           "wx123",
			AppSecret:       "secret",
			RedirectURL:     "https://gm.example.com/api/v1/auth/wechat/callback",
			LoginSuccessURL: "https://gm.example.com/done",
		},
	})
	require.NoError(t, err)
	assert.False(t, svc.LocalEnabled())
	assert.True(t, svc.WeChatEnabled())
	assert.Equal(t, "https://gm.example.com/done", svc.WeChatSuccessURL())
	url, err := svc.WeChatAuthCodeURL()
	require.NoError(t, err)
	assert.Contains(t, url, "open.weixin.qq.com/connect/qrconnect")
	assert.Contains(t, url, "appid=wx123")
	assert.Contains(t, url, "#wechat_redirect")

	_, err = svc.WeChatAuthCodeURL()
	require.NoError(t, err)
}

// TestRefreshIdentityProviders_LocalOffWithGenericOAuthAllowed local 关 +
// 自定义 OAuth 在 → 放行；scopes 逗号串切分进授权 URL。
func TestRefreshIdentityProviders_LocalOffWithGenericOAuthAllowed(t *testing.T) {
	svc, err := refreshWith(t, config.AuthProvidersConfig{
		Local: config.LocalProviderConfig{Enabled: localOff()},
		GenericOAuth: config.GenericOAuthProviderConfig{
			Enabled:      true,
			ClientID:     "cid",
			ClientSecret: "cs",
			RedirectURL:  "https://gm.example.com/api/v1/auth/generic/callback",
			AuthURL:      "https://idp.example.com/authorize",
			TokenURL:     "https://idp.example.com/token",
			UserInfoURL:  "https://idp.example.com/userinfo",
			Scopes:       "openid, profile ,email",
		},
	})
	require.NoError(t, err)
	assert.False(t, svc.LocalEnabled())
	assert.True(t, svc.GenericOAuthEnabled())
	url, err := svc.GenericOAuthAuthCodeURL()
	require.NoError(t, err)
	assert.Contains(t, url, "https://idp.example.com/authorize")
	assert.Contains(t, url, "scope=openid+profile+email")
}

// TestWeChatGenericLoginCallback_Flows 微信/自定义 OAuth 回调走公共出口：
// JIT 建号签发 token、Exchange 失败文案带提供方 label、未启用报错。
func TestWeChatGenericLoginCallback_Flows(t *testing.T) {
	// 复用 newOIDCService 的构造（角色模型齐备），分别挂微信/自定义 OAuth 假提供方。
	fake := &fakeOAuthProvider{
		authURL: "https://idp/auth",
		ident:   &identity.Identity{Provider: identity.KindWeChat, Username: "oX-wechat-1", Nickname: "老王"},
	}
	svc := newOIDCService(t, nil, "")
	svc.WithWeChatProvider(fake, []string{"viewer"}, "")

	state := svc.newOIDCState()
	resp, err := svc.WeChatLoginCallback(context.Background(), "code", state, &LoginRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "oX-wechat-1", resp.User.Username)

	// Exchange 失败：label 进文案（微信）。
	failing := &fakeOAuthProvider{err: errors.New("errcode=40029")}
	svc2 := newOIDCService(t, nil, "")
	svc2.WithWeChatProvider(failing, nil, "")
	_, err = svc2.WeChatLoginCallback(context.Background(), "code", svc2.newOIDCState(), &LoginRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "微信 登录失败")

	// 自定义 OAuth happy path + 未启用报错。
	fakeG := &fakeOAuthProvider{
		authURL: "https://idp/auth",
		ident:   &identity.Identity{Provider: identity.KindGenericOAuth, Username: "alice", Email: "a@ex.com"},
	}
	svc3 := newOIDCService(t, nil, "")
	svc3.WithGenericOAuthProvider(fakeG, nil, "")
	assert.True(t, svc3.GenericOAuthEnabled())
	resp3, err := svc3.GenericOAuthLoginCallback(context.Background(), "code", svc3.newOIDCState(), &LoginRequest{})
	require.NoError(t, err)
	assert.Equal(t, "alice", resp3.User.Username)

	_, err = svc3.WeChatLoginCallback(context.Background(), "code", svc3.newOIDCState(), &LoginRequest{})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "微信 登录未启用"), err.Error())
}
