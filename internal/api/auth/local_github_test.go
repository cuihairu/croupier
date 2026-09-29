package auth

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refreshWith 构造最小 Service 并应用给定身份源配置（走 RefreshIdentityProviders
// 生产路径：防锁死守卫与级联重建都在这里）。
func refreshWith(t *testing.T, cfg config.AuthProvidersConfig) (*Service, error) {
	t.Helper()
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), jwtutil.DevSecret()).
		WithRecoveryDB(db)
	err := svc.RefreshIdentityProviders(cfg)
	return svc, err
}

func localOff() *bool {
	f := false
	return &f
}

// TestRefreshIdentityProviders_AntiLockoutGuard 全部登录方式关闭 → 拒绝且现状不变。
func TestRefreshIdentityProviders_AntiLockoutGuard(t *testing.T) {
	svc, err := refreshWith(t, config.AuthProvidersConfig{
		Local:  config.LocalProviderConfig{Enabled: localOff()},
		LDAP:   config.LDAPProviderConfig{Enabled: false},
		OIDC:   config.OIDCProviderConfig{Enabled: false},
		GitHub: config.GitHubProviderConfig{Enabled: false},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不能停用所有登录方式")
	// 现状保持：本地登录仍可用（默认未停用）
	assert.True(t, svc.LocalEnabled())
}

// TestRefreshIdentityProviders_LocalOffWithGitHubAllowed local 关 + GitHub 在 → 放行，
// 本地级联移除、GitHub 生效。
func TestRefreshIdentityProviders_LocalOffWithGitHubAllowed(t *testing.T) {
	svc, err := refreshWith(t, config.AuthProvidersConfig{
		Local: config.LocalProviderConfig{Enabled: localOff()},
		GitHub: config.GitHubProviderConfig{
			Enabled:      true,
			ClientID:     "cid",
			ClientSecret: "secret",
			RedirectURL:  "https://gm.example.com/api/v1/auth/github/callback",
		},
	})
	require.NoError(t, err)
	assert.False(t, svc.LocalEnabled(), "local 级联被移除")
	assert.True(t, svc.GitHubEnabled())
	url, err := svc.GitHubAuthCodeURL()
	require.NoError(t, err)
	assert.Contains(t, url, "github.com/login/oauth/authorize")

	// 本地账号密码登录被拒：级联无 local provider，任何凭据都拿不到身份
	adminModel := svc.adminModel
	seed := &model.Admin{Username: "was-local", Nickname: "w", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), seed, "Str0ngPass!x"))
	_, loginErr := svc.Login(context.Background(), &LoginRequest{Username: "was-local", Password: "Str0ngPass!x"})
	require.Error(t, loginErr)
}

// TestRefreshIdentityProviders_LocalToggleRestore 重新开启 local → 级联恢复。
func TestRefreshIdentityProviders_LocalToggleRestore(t *testing.T) {
	off := config.AuthProvidersConfig{
		Local: config.LocalProviderConfig{Enabled: localOff()},
		GitHub: config.GitHubProviderConfig{
			Enabled: true, ClientID: "cid", ClientSecret: "s",
			RedirectURL: "https://gm.example.com/cb",
		},
	}
	svc, err := refreshWith(t, off)
	require.NoError(t, err)
	assert.False(t, svc.LocalEnabled())

	on := config.AuthProvidersConfig{Local: config.LocalProviderConfig{Enabled: nil}}
	// nil = 默认启用；重新挂上 GitHub 关闭——GitHub 关了但 local 在，不触锁死守卫
	require.NoError(t, svc.RefreshIdentityProviders(on))
	assert.True(t, svc.LocalEnabled())
	assert.False(t, svc.GitHubEnabled())
}

// TestGitHubFlow_DisabledByDefault 未配置时 GitHub 关闭，URL 生成报错。
func TestGitHubFlow_DisabledByDefault(t *testing.T) {
	svc, err := refreshWith(t, config.AuthProvidersConfig{})
	require.NoError(t, err)
	assert.True(t, svc.LocalEnabled(), "默认启用本地登录")
	assert.False(t, svc.GitHubEnabled())
	_, urlErr := svc.GitHubAuthCodeURL()
	require.Error(t, urlErr)
	assert.Contains(t, urlErr.Error(), "未启用")
}

// TestRefreshIdentityProviders_GitHubMissingConfig GitHub 开启但凭证缺失 → 拒绝（保存端回滚）。
func TestRefreshIdentityProviders_GitHubMissingConfig(t *testing.T) {
	_, err := refreshWith(t, config.AuthProvidersConfig{
		GitHub: config.GitHubProviderConfig{Enabled: true},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.providers.github")
}
