// 覆盖目标：resolveL2 的 obs URL 兜底分支、GetBool/GetInt 的 L2 命中
// 分支，以及登录方式（auth.*）相关 AuthSnapshot / authProviderSnapshot /
// settingsKey / AuthProviderConfig 全量读取路径（含凭据脱敏与角色解析）。
package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveL2_ObsURLFallbacks(t *testing.T) {
	resetForTest()
	l := InitLayered(context.Background(), &ConfigInput{
		ObsGrafanaExploreURL: "http://grafana:3000/explore",
		ObsJaegerURL:         "http://jaeger:16686",
	}, newStore(t))

	v, src, ok := l.GetString(context.Background(), KeyObsGrafanaExploreURL)
	require.True(t, ok)
	assert.Equal(t, "http://grafana:3000/explore", v)
	assert.Equal(t, "config", src)

	v, src, ok = l.GetString(context.Background(), KeyObsJaegerURL)
	require.True(t, ok)
	assert.Equal(t, "http://jaeger:16686", v)
	assert.Equal(t, "config", src)
}

func TestGetBool_L2ValueBranch(t *testing.T) {
	resetForTest()
	// L2 显式 true：FeatureEnabled 合成后 GetBool 命中 L2 分支。
	l := InitLayered(context.Background(), &ConfigInput{
		FeatureFlags: map[string]bool{"ops": true},
	}, newStore(t))
	assert.True(t, l.FeatureEnabled("ops"))

	// L2 存了非 bool JSON：解析失败回落默认值。
	l.l2Values[KeyFeatureDev] = json.RawMessage(`"yes"`)
	assert.True(t, l.GetBool(KeyFeatureDev, true))
	assert.False(t, l.GetBool(KeyFeatureDev, false))
}

func TestGetInt_L2ValueBranch(t *testing.T) {
	resetForTest()
	l := InitLayered(context.Background(), &ConfigInput{}, newStore(t))

	l.l2Values[KeyNotifySMTPPort] = json.RawMessage(`587`)
	assert.Equal(t, 587, l.GetInt(KeyNotifySMTPPort, 25))

	// L3 未命中时 L3 分支跳过，直接落 L2。
	l.l3Loaded = false
	assert.Equal(t, 587, l.GetInt(KeyNotifySMTPPort, 25))
}

func TestAuthSnapshot_LayeredFieldsAndMasking(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	set := func(key string, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "admin"))
	}
	// LDAP 全量配置（secret >4 位走尾 4 回显）。
	set(KeyAuthLdapEnabled, `true`)
	set(KeyAuthLdapAddr, `"ldap://dir:389"`)
	set(KeyAuthLdapBaseDn, `"dc=example,dc=com"`)
	set(KeyAuthLdapBindDn, `"uid=svc,ou=system,dc=example,dc=com"`)
	set(KeyAuthLdapUserFilter, `"(uid=%s)"`)
	set(KeyAuthLdapStartTLS, `true`)
	set(KeyAuthLdapDefaultRoles, `"viewer"`)
	set(KeyAuthLdapBindPassword, `"ldap-secret-99"`)
	// OIDC 部分（secret <=4 全掩码；addr 类字段留空验证不进 Fields）。
	set(KeyAuthOidcEnabled, `true`)
	set(KeyAuthOidcIssuer, `"https://idp.example.com"`)
	set(KeyAuthOidcClientId, `"croupier-web"`)
	set(KeyAuthOidcRedirectUrl, `"http://localhost:18780/api/auth/oidc/callback"`)
	set(KeyAuthOidcClientSecret, `"ab1"`)
	l.Reload(context.Background(), store)

	snap := l.AuthSnapshot()

	ldap := snap.LDAP
	assert.True(t, ldap.Enabled)
	assert.Equal(t, "ldap://dir:389", ldap.Fields["addr"])
	assert.Equal(t, "dc=example,dc=com", ldap.Fields["baseDn"])
	assert.Equal(t, "uid=svc,ou=system,dc=example,dc=com", ldap.Fields["bindDn"])
	assert.Equal(t, "(uid=%s)", ldap.Fields["userFilter"])
	assert.Equal(t, "true", ldap.Fields["startTls"])
	assert.Equal(t, "viewer", ldap.Fields["defaultRoles"])
	assert.True(t, ldap.SecretSet)
	assert.Equal(t, "****t-99", ldap.SecretMasked) // "****" + 尾4（"ldap-secret-99"）
	assert.Equal(t, "database", ldap.Sources["addr"])
	assert.Equal(t, "database", ldap.Sources["secret"])

	oidc := snap.OIDC
	assert.True(t, oidc.Enabled)
	assert.Equal(t, "https://idp.example.com", oidc.Fields["issuer"])
	assert.Equal(t, "croupier-web", oidc.Fields["clientId"])
	assert.Equal(t, "http://localhost:18780/api/auth/oidc/callback", oidc.Fields["redirectUrl"])
	// OIDC 无 startTls 字段语义，不进 Fields。
	assert.NotContains(t, oidc.Fields, "startTls")
	assert.NotContains(t, oidc.Fields, "baseDn")
	assert.True(t, oidc.SecretSet)
	assert.Equal(t, "****", oidc.SecretMasked) // <=4 全掩码

	// 清空 secret：SecretSet 回 false。
	require.NoError(t, store.Clear(context.Background(), KeyAuthOidcClientSecret))
	set(KeyAuthOidcClientSecret, `""`)
	l.Reload(context.Background(), store)
	assert.False(t, l.AuthSnapshot().OIDC.SecretSet)
	assert.Empty(t, l.AuthSnapshot().OIDC.SecretMasked)
}

func TestAuthSnapshot_NilReceiverIsSafe(t *testing.T) {
	var l *Layered
	snap := l.AuthSnapshot()
	assert.False(t, snap.LDAP.Enabled)
	assert.False(t, snap.OIDC.Enabled)
	assert.Empty(t, snap.LDAP.Fields)
	assert.Empty(t, snap.LDAP.Sources)
	assert.False(t, snap.LDAP.SecretSet)
}

func TestAuthProviderConfig_ResolvesLayers(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	set := func(key string, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "admin"))
	}
	set(KeyAuthLdapEnabled, `true`)
	set(KeyAuthLdapAddr, `"ldap://dir:389"`)
	set(KeyAuthLdapBaseDn, `"dc=example,dc=com"`)
	set(KeyAuthLdapBindDn, `"uid=svc"`)
	set(KeyAuthLdapBindPassword, `"bind-pw"`)
	set(KeyAuthLdapUserFilter, `"(uid=%s)"`)
	set(KeyAuthLdapStartTLS, `true`)
	set(KeyAuthLdapDefaultRoles, `" viewer , editor ,,ops "`)
	set(KeyAuthOidcEnabled, `true`)
	set(KeyAuthOidcIssuer, `"https://idp.example.com"`)
	set(KeyAuthOidcClientId, `"croupier-web"`)
	set(KeyAuthOidcClientSecret, `"cs"`)
	set(KeyAuthOidcRedirectUrl, `"http://localhost/cb"`)
	set(KeyAuthOidcDefaultRoles, `"ops"`)
	l.Reload(context.Background(), store)

	cfg := l.AuthProviderConfig()
	assert.True(t, cfg.LDAP.Enabled)
	assert.Equal(t, "ldap://dir:389", cfg.LDAP.Addr)
	assert.Equal(t, "dc=example,dc=com", cfg.LDAP.BaseDN)
	assert.Equal(t, "uid=svc", cfg.LDAP.BindDN)
	assert.Equal(t, "bind-pw", cfg.LDAP.BindPassword)
	assert.Equal(t, "(uid=%s)", cfg.LDAP.UserFilter)
	assert.True(t, cfg.LDAP.StartTLS)
	// 兼容语义：UserDNTemplate 回退为 userFilter 值。
	assert.Equal(t, "(uid=%s)", cfg.LDAP.UserDNTemplate)
	// 角色解析：trim + 剔除空段。
	assert.Equal(t, []string{"viewer", "editor", "ops"}, cfg.LDAP.DefaultRoles)

	assert.True(t, cfg.OIDC.Enabled)
	assert.Equal(t, "https://idp.example.com", cfg.OIDC.Issuer)
	assert.Equal(t, "croupier-web", cfg.OIDC.ClientID)
	assert.Equal(t, "cs", cfg.OIDC.ClientSecret)
	assert.Equal(t, "http://localhost/cb", cfg.OIDC.RedirectURL)
	assert.Equal(t, []string{"ops"}, cfg.OIDC.DefaultRoles)

	// 未配置任何覆盖时：全默认、roles 为 nil。
	resetForTest()
	empty := InitLayered(context.Background(), &ConfigInput{}, newStore(t)).AuthProviderConfig()
	assert.False(t, empty.LDAP.Enabled)
	assert.False(t, empty.OIDC.Enabled)
	assert.Nil(t, empty.LDAP.DefaultRoles)
	assert.Nil(t, empty.OIDC.DefaultRoles)
}
