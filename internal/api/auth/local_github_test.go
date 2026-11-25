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

// refreshWith builds a minimal Service and applies the given identity provider
// config (driving the production RefreshIdentityProviders path: the anti-lockout
// guard and the cascading rebuild both live there).
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

// TestRefreshIdentityProviders_AntiLockoutGuard all login methods disabled →
// rejected, and the current state is unchanged.
func TestRefreshIdentityProviders_AntiLockoutGuard(t *testing.T) {
	svc, err := refreshWith(t, config.AuthProvidersConfig{
		Local:  config.LocalProviderConfig{Enabled: localOff()},
		LDAP:   config.LDAPProviderConfig{Enabled: false},
		OIDC:   config.OIDCProviderConfig{Enabled: false},
		GitHub: config.GitHubProviderConfig{Enabled: false},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不能停用所有登录方式")
	// current state preserved: local login is still usable (not disabled by
	// default)
	assert.True(t, svc.LocalEnabled())
}

// TestRefreshIdentityProviders_LocalOffWithGitHubAllowed local off + GitHub
// on → allowed, local is cascade-removed and GitHub takes effect.
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

	// local account/password login is rejected: no local provider after the
	// cascade, so no credential can obtain an identity
	adminModel := svc.adminModel
	seed := &model.Admin{Username: "was-local", Nickname: "w", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), seed, "Str0ngPass!x"))
	_, loginErr := svc.Login(context.Background(), &LoginRequest{Username: "was-local", Password: "Str0ngPass!x"})
	require.Error(t, loginErr)
}

// TestRefreshIdentityProviders_LocalToggleRestore re-enabling local → the
// cascade is restored.
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
	// nil = enabled by default; re-attach with GitHub off — GitHub off while
	// local is on does not trip the lockout guard
	require.NoError(t, svc.RefreshIdentityProviders(on))
	assert.True(t, svc.LocalEnabled())
	assert.False(t, svc.GitHubEnabled())
}

// TestGitHubFlow_DisabledByDefault with no config GitHub is off, and URL
// generation errors.
func TestGitHubFlow_DisabledByDefault(t *testing.T) {
	svc, err := refreshWith(t, config.AuthProvidersConfig{})
	require.NoError(t, err)
	assert.True(t, svc.LocalEnabled(), "默认启用本地登录")
	assert.False(t, svc.GitHubEnabled())
	_, urlErr := svc.GitHubAuthCodeURL()
	require.Error(t, urlErr)
	assert.Contains(t, urlErr.Error(), "未启用")
}

// TestRefreshIdentityProviders_GitHubMissingConfig GitHub on but credentials
// missing → rejected (rolled back on the save side).
func TestRefreshIdentityProviders_GitHubMissingConfig(t *testing.T) {
	_, err := refreshWith(t, config.AuthProvidersConfig{
		GitHub: config.GitHubProviderConfig{Enabled: true},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.providers.github")
}
