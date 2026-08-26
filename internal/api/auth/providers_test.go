package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakePasswordProvider 是密码提供方测试替身。
type fakePasswordProvider struct {
	kind        string
	validUsers  map[string]string // username -> password
	identities  map[string]*identity.Identity
	infraErr    error
	lastAttempt string
}

func (f *fakePasswordProvider) Kind() string { return f.kind }

func (f *fakePasswordProvider) Authenticate(ctx context.Context, username, password string) (*identity.Identity, error) {
	f.lastAttempt = username
	if f.infraErr != nil {
		return nil, f.infraErr
	}
	want, ok := f.validUsers[username]
	if !ok || want != password {
		return nil, identity.ErrInvalidCredentials
	}
	base := f.identities[username]
	if base == nil {
		base = &identity.Identity{Provider: f.kind, Username: username, Nickname: username + " Zhang"}
	}
	cp := *base
	return &cp, nil
}

func newCascadeService(t *testing.T, extra ...identity.PasswordProvider) (*Service, *gorm.DB) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), jwtutil.DevSecret()).
		WithRoleModel(roleModel)
	for _, p := range extra {
		svc.WithPasswordProvider(p)
	}
	return svc, db
}

func TestLogin_LocalStillWorks(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), jwtutil.DevSecret()).
		WithRoleModel(roleModel)

	// 外部源配置为空时，仅本地提供方可用。
	assert.False(t, svc.LDAPEnabled())
	assert.False(t, svc.OIDCEnabled())

	createTestAdminWithRole(t, db, "alice", "pw", "ops")
	resp, err := svc.Login(context.Background(), &LoginRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "alice", resp.User.Username)
}

func TestLogin_LDAPFallback_JITProvision(t *testing.T) {
	ldap := &fakePasswordProvider{
		kind:       identity.KindLDAP,
		validUsers: map[string]string{"bob": "ldappw"},
	}
	svc, db := newCascadeService(t, ldap)
	assert.True(t, svc.LDAPEnabled())

	// 准备默认角色。
	roleModel := model.NewRoleModel(db)
	role := &model.Role{Name: "viewer"}
	require.NoError(t, roleModel.Create(context.Background(), role))
	svc.WithProviderDefaultRoles(identity.KindLDAP, []string{"viewer", "missing-role"})

	// 本地不存在 bob：级联到 LDAP 成功，并 JIT 建号 + 赋默认角色。
	resp, err := svc.Login(context.Background(), &LoginRequest{Username: "bob", Password: "ldappw"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Contains(t, resp.User.Roles, "viewer")
	assert.NotContains(t, resp.User.Roles, "missing-role")

	// 影子账号密码为随机值，不能用本地密码登录。
	_, err = svc.Login(context.Background(), &LoginRequest{Username: "bob", Password: "anything"})
	require.Error(t, err)

	// LDAP 密码错误 → 整体凭证失败。
	_, err = svc.Login(context.Background(), &LoginRequest{Username: "bob", Password: "nope"})
	require.Error(t, err)
}

func TestLogin_AllProvidersFail(t *testing.T) {
	ldap := &fakePasswordProvider{kind: identity.KindLDAP, validUsers: map[string]string{}}
	svc, _ := newCascadeService(t, ldap)

	_, err := svc.Login(context.Background(), &LoginRequest{Username: "ghost", Password: "pw"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户名或密码错误")
}

func TestLogin_ProviderInfraError(t *testing.T) {
	ldap := &fakePasswordProvider{
		kind:     identity.KindLDAP,
		infraErr: errors.New("connection refused"),
	}
	svc, _ := newCascadeService(t, ldap)

	_, err := svc.Login(context.Background(), &LoginRequest{Username: "bob", Password: "pw"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "认证服务暂时不可用")
}

func TestOIDCState_RoundTrip(t *testing.T) {
	svc, _ := newCascadeService(t)

	state, err := svc.newOIDCState()
	require.NoError(t, err)
	assert.True(t, svc.verifyOIDCState(state), "fresh state must verify")

	assert.False(t, svc.verifyOIDCState("garbage"))
	assert.False(t, svc.verifyOIDCState(""))
	assert.False(t, svc.verifyOIDCState(state+"x"))

	// 篡改 payload 后签名不匹配。
	parts := splitState(state)
	assert.False(t, svc.verifyOIDCState("YWJj.13.37."+parts[1]))
}

func splitState(state string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(state); i++ {
		if state[i] == '.' {
			out = append(out, state[start:i])
			start = i + 1
		}
	}
	return append(out, state[start:])
}

func TestProviders_ConfigValidation(t *testing.T) {
	// LDAP 缺关键配置：报错。
	_, err := buildIdentityProviders(authProvidersConfigLDAP(true, "", "", ""))
	require.Error(t, err)

	// 合法 LDAP 配置：装配成功。
	ips, err := buildIdentityProviders(authProvidersConfigLDAP(true, "ldap://x:389", "dc=e,dc=c", "(uid=%s)"))
	require.NoError(t, err)
	require.NotNil(t, ips.ldap)

	// 未启用：不装配。
	ips, err = buildIdentityProviders(authProvidersConfigLDAP(false, "ldap://x:389", "dc=e,dc=c", "(uid=%s)"))
	require.NoError(t, err)
	assert.Nil(t, ips.ldap)
	assert.Nil(t, ips.oidc)
}

func authProvidersConfigLDAP(enabled bool, addr, baseDN, userFilter string) config.AuthProvidersConfig {
	cfg := config.AuthProvidersConfig{
		LDAP: config.LDAPProviderConfig{
			Enabled:    enabled,
			Addr:       addr,
			BaseDN:     baseDN,
			BindDN:     "uid=svc,ou=system,dc=example,dc=com",
			UserFilter: userFilter,
		},
	}
	return cfg
}
