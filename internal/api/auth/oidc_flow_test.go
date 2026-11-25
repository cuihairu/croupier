package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// gormSessionAlias makes constructing gorm.Session easier.
type gormSession = gorm.Session

// fakeOAuthProvider is a test double for OAuthProvider, covering the OIDC
// flow.
type fakeOAuthProvider struct {
	authURL string
	ident   *identity.Identity
	err     error
	lastCtx context.Context
}

func (f *fakeOAuthProvider) Kind() string { return identity.KindOIDC }

func (f *fakeOAuthProvider) AuthCodeURL(state string) string {
	return f.authURL + "?state=" + state
}

func (f *fakeOAuthProvider) Exchange(ctx context.Context, code string) (*identity.Identity, error) {
	f.lastCtx = ctx
	if f.err != nil {
		return nil, f.err
	}
	cp := *f.ident
	return &cp, nil
}

// newOIDCService builds a Service with the fake OIDC provider attached.
func newOIDCService(t *testing.T, oauth identity.OAuthProvider, successURL string) *Service {
	db := setupTestDB(t)
	svc := NewService(
		model.NewAdminModel(db),
		permissionservice.NewPermissionService(db),
		"test-secret",
	).WithRecoveryDB(db).WithRoleModel(model.NewRoleModel(db))
	if oauth != nil {
		svc.WithOIDCProvider(oauth, []string{"viewer"}, successURL)
	}
	return svc
}

func TestOIDCEnabled_Flags(t *testing.T) {
	svc := newOIDCService(t, nil, "")
	assert.False(t, svc.OIDCEnabled())

	fake := &fakeOAuthProvider{authURL: "https://idp/auth", ident: &identity.Identity{}}
	svc = newOIDCService(t, fake, "http://frontend/login")
	assert.True(t, svc.OIDCEnabled())
	assert.Equal(t, "http://frontend/login", svc.OIDCSuccessURL())
}

func TestOIDCAuthCodeURL(t *testing.T) {
	fake := &fakeOAuthProvider{authURL: "https://idp/auth", ident: &identity.Identity{}}
	svc := newOIDCService(t, fake, "")

	url, err := svc.OIDCAuthCodeURL()
	require.NoError(t, err)
	assert.Contains(t, url, "https://idp/auth?state=")
}

func TestOIDCAuthCodeURL_Disabled(t *testing.T) {
	svc := newOIDCService(t, nil, "")
	_, err := svc.OIDCAuthCodeURL()
	require.Error(t, err)
}

func TestOIDCLoginCallback_HappyPath(t *testing.T) {
	fake := &fakeOAuthProvider{
		authURL: "https://idp/auth",
		ident:   &identity.Identity{Provider: identity.KindOIDC, Username: "carol", Nickname: "Carol", Email: "carol@example.com"},
	}
	svc := newOIDCService(t, fake, "")

	state := svc.newOIDCState()

	resp, err := svc.OIDCLoginCallback(context.Background(), "auth-code", state, &LoginRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "carol", resp.User.Username)
	// JIT account creation with no default role configured against the viewer
	// role inside the RoleModel → empty roles but the login still succeeds.
	assert.Empty(t, resp.User.Roles)
}

func TestOIDCLoginCallback_Disabled(t *testing.T) {
	svc := newOIDCService(t, nil, "")
	_, err := svc.OIDCLoginCallback(context.Background(), "code", "state", nil)
	require.Error(t, err)
}

func TestOIDCLoginCallback_BadState(t *testing.T) {
	fake := &fakeOAuthProvider{ident: &identity.Identity{Username: "carol"}}
	svc := newOIDCService(t, fake, "")

	_, err := svc.OIDCLoginCallback(context.Background(), "code", "tampered", &LoginRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "重新发起登录")
}

func TestOIDCLoginCallback_EmptyCode(t *testing.T) {
	fake := &fakeOAuthProvider{ident: &identity.Identity{Username: "carol"}}
	svc := newOIDCService(t, fake, "")

	state := svc.newOIDCState()
	_, err := svc.OIDCLoginCallback(context.Background(), "", state, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "授权码")
}

func TestOIDCLoginCallback_ExchangeFailed(t *testing.T) {
	fake := &fakeOAuthProvider{
		err: errors.New("token endpoint 500"),
	}
	svc := newOIDCService(t, fake, "")

	state := svc.newOIDCState()
	_, err := svc.OIDCLoginCallback(context.Background(), "code", state, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OIDC 登录失败")
}

func TestHandler_Providers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := newOIDCService(t, nil, "")
	svc.WithPasswordProvider(&fakePasswordProvider{kind: identity.KindLDAP})
	h := NewHandler(svc)

	c, rec := newAuthTestContext(http.MethodGet, "/api/v1/auth/providers", "")
	h.Providers(c)
	assertHTTPStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assert.Contains(t, body, `"local":true`)
	assert.Contains(t, body, `"ldap":true`)
	assert.Contains(t, body, `"oidc":false`)
}

func TestHandler_OIDCLogin_Redirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &fakeOAuthProvider{authURL: "https://idp/auth", ident: &identity.Identity{}}
	h := NewHandler(newOIDCService(t, fake, ""))

	c, rec := newAuthTestContext(http.MethodGet, "/api/v1/auth/oidc/login", "")
	h.OIDCLogin(c)
	assertHTTPStatus(t, rec, http.StatusFound)
	assert.Contains(t, rec.Header().Get("Location"), "https://idp/auth?state=")
}

func TestHandler_OIDCLogin_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHandler(newOIDCService(t, nil, ""))
	c, rec := newAuthTestContext(http.MethodGet, "/api/v1/auth/oidc/login", "")
	h.OIDCLogin(c)
	assertHTTPStatus(t, rec, http.StatusBadRequest)
}

func TestHandler_OIDCCallback_JSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &fakeOAuthProvider{
		authURL: "https://idp/auth",
		ident:   &identity.Identity{Provider: identity.KindOIDC, Username: "carol", Nickname: "Carol"},
	}
	svc := newOIDCService(t, fake, "")
	h := NewHandler(svc)

	state := svc.newOIDCState()

	c, rec := newAuthTestContext(http.MethodGet, "/api/v1/auth/oidc/callback?code=abc&state="+state, "")
	h.OIDCCallback(c)
	assertHTTPStatus(t, rec, http.StatusOK)
	assert.Contains(t, rec.Body.String(), `"token"`)
}

func TestHandler_OIDCCallback_Redirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &fakeOAuthProvider{
		authURL: "https://idp/auth",
		ident:   &identity.Identity{Provider: identity.KindOIDC, Username: "carol"},
	}
	svc := newOIDCService(t, fake, "http://frontend:8000/login")
	h := NewHandler(svc)

	state := svc.newOIDCState()

	c, rec := newAuthTestContext(http.MethodGet, "/api/v1/auth/oidc/callback?code=abc&state="+state, "")
	h.OIDCCallback(c)
	assertHTTPStatus(t, rec, http.StatusFound)
	loc := rec.Header().Get("Location")
	assert.Contains(t, loc, "http://frontend:8000/login?")
	assert.Contains(t, loc, "token=")
}

func TestHandler_OIDCCallback_InvalidState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &fakeOAuthProvider{ident: &identity.Identity{Username: "carol"}}
	h := NewHandler(newOIDCService(t, fake, ""))

	c, rec := newAuthTestContext(http.MethodGet, "/api/v1/auth/oidc/callback?code=abc&state=bad", "")
	h.OIDCCallback(c)
	assertHTTPStatus(t, rec, http.StatusUnauthorized)
}

func TestHandler_OIDCCallback_BadSuccessURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &fakeOAuthProvider{
		authURL: "https://idp/auth",
		ident:   &identity.Identity{Provider: identity.KindOIDC, Username: "carol"},
	}
	// a control character makes url.Parse fail.
	svc := newOIDCService(t, fake, "http://bad\x7f")
	h := NewHandler(svc)

	state := svc.newOIDCState()

	c, rec := newAuthTestContext(http.MethodGet, "/api/v1/auth/oidc/callback?code=abc&state="+state, "")
	h.OIDCCallback(c)
	assertHTTPStatus(t, rec, http.StatusInternalServerError)
	assert.Contains(t, rec.Body.String(), "loginSuccessUrl")
}

func TestLogin_BackfillProfileFromLDAP(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(
		model.NewAdminModel(db),
		permissionservice.NewPermissionService(db),
		"test-secret",
	).WithRecoveryDB(db).WithRoleModel(model.NewRoleModel(db))

	// seed a local account with an empty nickname/email, whose password differs
	// from the LDAP one.
	admin := &model.Admin{Username: "dave", Status: 1}
	require.NoError(t, model.NewAdminModel(db).Create(context.Background(), admin, "localpw"))

	ldap := &fakePasswordProvider{
		kind:       identity.KindLDAP,
		validUsers: map[string]string{"dave": "ldappw"},
		identities: map[string]*identity.Identity{
			"dave": {Provider: identity.KindLDAP, Username: "dave", Nickname: "Dave L", Email: "dave@ldap.example.com"},
		},
	}
	svc.WithPasswordProvider(ldap)

	resp, err := svc.Login(context.Background(), &LoginRequest{Username: "dave", Password: "ldappw"})
	require.NoError(t, err)
	assert.Equal(t, "dave", resp.User.Username)

	// the display information is backfilled from the external identity.
	updated, err := model.NewAdminModel(db).FindByUsername(context.Background(), "dave")
	require.NoError(t, err)
	assert.Equal(t, "Dave L", updated.Nickname)
	assert.Equal(t, "dave@ldap.example.com", updated.Email)

	// logging in with the local password still works (the local cascade).
	resp, err = svc.Login(context.Background(), &LoginRequest{Username: "dave", Password: "localpw"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
}

func TestNewService_WithOpsStore(t *testing.T) {
	db := setupTestDB(t)
	assert.NotPanics(t, func() {
		svc := NewService(
			model.NewAdminModel(db),
			permissionservice.NewPermissionService(db),
			"test-secret",
			nil,
		)
		require.NotNil(t, svc)
	})
}

// fakeDiscoveryServer serves a minimal OIDC discovery endpoint (for the
// assembly-layer tests).
func fakeDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issuer": "` + srv.URL + `",
			"authorization_endpoint": "` + srv.URL + `/auth",
			"token_endpoint": "` + srv.URL + `/token",
			"jwks_uri": "` + srv.URL + `/jwks",
			"response_types_supported": ["code"],
			"subject_types_supported": ["public"],
			"id_token_signing_alg_values_supported": ["RS256"]
		}`))
	})
	return srv
}

func TestBuildIdentityProviders(t *testing.T) {
	// all off: empty assembly.
	ips, err := BuildIdentityProviders(config.AuthProvidersConfig{})
	require.NoError(t, err)
	require.NotNil(t, ips)
	assert.Nil(t, ips.ldap)
	assert.Nil(t, ips.oidc)

	// valid LDAP: assembled and Attach-able.
	cfg := config.Config{}
	cfg.Auth.Providers.LDAP = config.LDAPProviderConfig{
		Enabled:    true,
		Addr:       "ldap://x:389",
		BaseDN:     "dc=e,dc=c",
		BindDN:     "uid=svc",
		UserFilter: "(uid=%s)",
	}
	ips, err = BuildIdentityProviders(cfg.Auth.Providers)
	require.NoError(t, err)
	assert.NotNil(t, ips.ldap)

	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "s").WithRecoveryDB(db).WithRecoveryDB(db)
	assert.False(t, svc.LDAPEnabled())
	ips.Attach(svc)
	assert.True(t, svc.LDAPEnabled())

	// valid OIDC (against the fake discovery server).
	srv := fakeDiscoveryServer(t)
	cfg.Auth.Providers.OIDC = config.OIDCProviderConfig{
		Enabled:         true,
		Issuer:          srv.URL,
		ClientID:        "croupier",
		ClientSecret:    "cs",
		RedirectURL:     "http://localhost:18780/api/auth/oidc/callback",
		LoginSuccessURL: "http://frontend:8000/login",
	}
	ips, err = BuildIdentityProviders(cfg.Auth.Providers)
	require.NoError(t, err)
	assert.NotNil(t, ips.ldap)
	assert.NotNil(t, ips.oidc)
	assert.Equal(t, "http://frontend:8000/login", ips.oidcURL)

	svc2 := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "s").WithRecoveryDB(db).WithRecoveryDB(db)
	ips.Attach(svc2)
	assert.True(t, svc2.OIDCEnabled())

	// OIDC missing a key field: error.
	cfg.Auth.Providers.OIDC.ClientID = ""
	_, err = BuildIdentityProviders(cfg.Auth.Providers)
	require.Error(t, err)

	// OIDC identity source unreachable: degrade (no error, OIDC not assembled,
	// LDAP unaffected).
	cfg.Auth.Providers.OIDC = config.OIDCProviderConfig{
		Enabled:      true,
		Issuer:       "http://127.0.0.1:1/nowhere",
		ClientID:     "croupier",
		ClientSecret: "cs",
		RedirectURL:  "http://localhost/cb",
	}
	ips, err = BuildIdentityProviders(cfg.Auth.Providers)
	require.NoError(t, err)
	assert.NotNil(t, ips.ldap)
	assert.Nil(t, ips.oidc)
}

func TestBuildIdentityProviders_NilAttach(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "s").WithRecoveryDB(db).WithRecoveryDB(db)
	assert.NotPanics(t, func() { (*IdentityProviders)(nil).Attach(svc) })
}

func TestLogin_LocalRecordVanished(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)

	// seed a local account so LocalProvider's validation passes, then wipe it
	// from the underlying store to simulate the extreme window of "the record
	// vanished after validation passed".
	createTestAdminWithRole(t, db, "tempuser", "pw", "ops")
	stmt := db.Session(&gormSession{}).Exec("DELETE FROM admins WHERE username = ?", "tempuser")
	require.NoError(t, stmt.Error)

	_, err := svc.Login(context.Background(), &LoginRequest{Username: "tempuser", Password: "pw"})
	require.Error(t, err)
	// The adminModel inside LocalProvider is the same object as the outer one;
	// validation and resolution share the same view, so what actually triggers
	// here is the invalid-credentials branch.
	assert.True(t, strings.Contains(err.Error(), "用户名或密码错误") || strings.Contains(err.Error(), "登录失败"))
}
