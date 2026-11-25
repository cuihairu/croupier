// Coverage targets: the three handler-layer MFA endpoints and the Login
// mfa_required branch, all error branches and audit records of mfa.go,
// RefreshIdentityProviders, and the error and fallback paths of
// Login/Logout/Check/OIDC state verification.
// Error injection tools: PRAGMA query_only (reads succeed, writes fail),
// dropping tables, and test doubles.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// readOnlyDB collapses the connection pool to a single connection and sets
// query_only: reads work as usual, writes always fail — used to cover the
// "read succeeds, write fails" warning branch.
func readOnlyDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)
}

func TestHandler_Login_MFARequiredBranch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	ctx := context.Background()
	createTestAdminWithRole(t, db, "mfahttp", "CorrectPass123", "admin")

	setup, err := svc.MFASetup(ctx, "mfahttp")
	require.NoError(t, err)
	_, err = svc.MFAConfirm(ctx, "mfahttp", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	h := NewHandler(svc)
	c, rec := newAuthTestContext("POST", "/login", `{"username":"mfahttp","password":"CorrectPass123"}`)
	h.Login(c)

	assertHTTPStatus(t, rec, http.StatusUnauthorized)
	assertErrorCode(t, rec, "mfa_required")
}

func TestMFAUsername_ContextBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := newAuthTestContext("POST", "/x", "")
	assert.Empty(t, mfaUsername(c), "no username in context")

	c, _ = newAuthTestContext("POST", "/x", "")
	c.Set("username", 12345) // not a string
	assert.Empty(t, mfaUsername(c))

	c, _ = newAuthTestContext("POST", "/x", "")
	c.Set("username", "alice")
	assert.Equal(t, "alice", mfaUsername(c))
}

func TestHandler_MFAEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	createTestAdminWithRole(t, db, "mfaep", "CorrectPass123", "admin")
	h := NewHandler(svc)

	post := func(path, body string, username any) *httptest.ResponseRecorder {
		c, rec := newAuthTestContext("POST", path, body)
		if username != nil {
			c.Set("username", username)
		}
		switch path {
		case "/mfa/setup":
			h.MFASetup(c)
		case "/mfa/confirm":
			h.MFAConfirm(c)
		case "/mfa/disable":
			h.MFADisable(c)
		}
		return rec
	}

	// Not logged in: 401.
	for _, path := range []string{"/mfa/setup", "/mfa/confirm", "/mfa/disable"} {
		rec := post(path, `{}`, nil)
		assertHTTPStatus(t, rec, http.StatusUnauthorized)
	}

	// setup succeeds: returns the secret.
	rec := post("/mfa/setup", ``, "mfaep")
	assertHTTPStatus(t, rec, http.StatusOK)
	var setup MFASetupResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &setup))
	require.NotEmpty(t, setup.Secret)

	// confirm: missing code fails to bind → 400; user missing → 400; correct code → 200.
	assertHTTPStatus(t, post("/mfa/confirm", `{}`, "mfaep"), http.StatusBadRequest)
	assertHTTPStatus(t, post("/mfa/confirm", `{"code":"000000"}`, "ghost"), http.StatusBadRequest)
	assertHTTPStatus(t, post("/mfa/confirm", `{"code":"`+currentTOTP(t, setup.Secret)+`"}`, "mfaep"), http.StatusOK)

	// disable: missing field → 400; wrong code → 400; correct code+password → 200.
	assertHTTPStatus(t, post("/mfa/disable", `{"code":"000000"}`, "mfaep"), http.StatusBadRequest)
	assertHTTPStatus(t, post("/mfa/disable", `{"code":"000000","password":"x"}`, "mfaep"), http.StatusBadRequest)
	assertHTTPStatus(t, post("/mfa/disable",
		`{"code":"`+currentTOTP(t, setup.Secret)+`","password":"CorrectPass123"}`, "mfaep"), http.StatusOK)

	// setup with a service error (user missing): 400.
	assertHTTPStatus(t, post("/mfa/setup", ``, "ghost"), http.StatusBadRequest)
}

// The setup endpoint also works with an empty body (no fields to bind).

func TestMFAService_ErrorBranches(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	ctx := context.Background()
	createTestAdminWithRole(t, db, "mfabranch", "CorrectPass123", "admin")

	// User missing.
	_, err := svc.MFASetup(ctx, "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户不存在")
	_, err = svc.MFAConfirm(ctx, "ghost", "000000")
	require.Error(t, err)
	require.Error(t, svc.MFADisable(ctx, "ghost", "000000", "pw"))

	// Shadow account (external identity source, PasswordHash empty).
	shadow := &model.Admin{Username: "extshadow", Status: 1}
	require.NoError(t, db.Create(shadow).Error)
	_, err = svc.MFASetup(ctx, "extshadow")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "外部身份源账号")

	// confirm without a prior setup: prompts to fetch the secret first.
	_, err = svc.MFAConfirm(ctx, "mfabranch", "000000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "请先获取二次验证密钥")

	// disable when not enabled.
	err = svc.MFADisable(ctx, "mfabranch", "000000", "pw")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未启用二次验证")

	// After enabling, disable with a wrong code.
	setup, err := svc.MFASetup(ctx, "mfabranch")
	require.NoError(t, err)
	_, err = svc.MFAConfirm(ctx, "mfabranch", currentTOTP(t, setup.Secret))
	require.NoError(t, err)
	err = svc.MFADisable(ctx, "mfabranch", "000000", "CorrectPass123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "验证码错误")
}

func TestMFAService_StoreWriteFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("setup save secret fails", func(t *testing.T) {
		db := setupTestDB(t)
		adminModel := model.NewAdminModel(db)
		svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
		createTestAdminWithRole(t, db, "saveme", "CorrectPass123", "admin")

		readOnlyDB(t, db)
		_, err := svc.MFASetup(ctx, "saveme")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "保存密钥失败")
	})

	t.Run("confirm enable fails", func(t *testing.T) {
		db := setupTestDB(t)
		adminModel := model.NewAdminModel(db)
		svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
		createTestAdminWithRole(t, db, "enableme", "CorrectPass123", "admin")

		setup, err := svc.MFASetup(ctx, "enableme")
		require.NoError(t, err)
		readOnlyDB(t, db)
		_, err = svc.MFAConfirm(ctx, "enableme", currentTOTP(t, setup.Secret))
		require.Error(t, err)
		// Recovery codes land in the database before the toggle (if that write
		// fails, MFA must not be enabled, or the user never gets recovery
		// codes), so on a read-only database the recovery-code write is hit
		// first.
		assert.Contains(t, err.Error(), "保存备用恢复码失败")
	})

	// When recovery codes are already persisted but enabling the toggle fails,
	// it must report the enablement failure message asserted below and stay disabled.
	// Technique: the recovery-code store runs in DryRun (builds SQL without
	// executing → counts as a successful write), while the admins table is
	// genuinely read-only, so only the EnableOTP failure is hit.
	// (This cannot be done with "two *gorm.DB handles": SQLite's PRAGMA
	//   query_only is per-connection, and two handles sharing one *sql.DB
	//   turn read-only together.)
	t.Run("confirm recovery saved but enable fails", func(t *testing.T) {
		db := setupTestDB(t)
		writableAdmin := model.NewAdminModel(db)
		svc := NewService(writableAdmin, permissionservice.NewPermissionService(db), "test-secret").
			WithRecoveryDB(db.Session(&gorm.Session{DryRun: true}))
		createTestAdminWithRole(t, db, "enablefail", "CorrectPass123", "admin")

		setup, err := svc.MFASetup(ctx, "enablefail")
		require.NoError(t, err)
		readOnlyDB(t, db)
		_, err = svc.MFAConfirm(ctx, "enablefail", currentTOTP(t, setup.Secret))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "启用失败")
	})

	t.Run("disable fails", func(t *testing.T) {
		db := setupTestDB(t)
		adminModel := model.NewAdminModel(db)
		svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
		createTestAdminWithRole(t, db, "disableme", "CorrectPass123", "admin")

		setup, err := svc.MFASetup(ctx, "disableme")
		require.NoError(t, err)
		_, err = svc.MFAConfirm(ctx, "disableme", currentTOTP(t, setup.Secret))
		require.NoError(t, err)
		readOnlyDB(t, db)
		err = svc.MFADisable(ctx, "disableme", currentTOTP(t, setup.Secret), "CorrectPass123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "关闭失败")
	})
}

func TestRecordMfaAudit_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("persists mfa events", func(t *testing.T) {
		db := setupTestDB(t)
		db.Exec("DELETE FROM audit_records")
		svc := withTableAudit(t, db, NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db))
		createTestAdminWithRole(t, db, "auditmfa", "CorrectPass123", "admin")

		setup, err := svc.MFASetup(ctx, "auditmfa")
		require.NoError(t, err)
		_, err = svc.MFAConfirm(ctx, "auditmfa", currentTOTP(t, setup.Secret))
		require.NoError(t, err)

		row := lastAuditRow(t, db)
		assert.Equal(t, string(audit.EventMFAEnabled), row["event_type"])
		assert.Equal(t, "auditmfa", row["actor_id"])
	})

	t.Run("audit store failure logs and continues", func(t *testing.T) {
		db := setupTestDB(t)
		svc := withTableAudit(t, db, NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db))
		require.NoError(t, db.Migrator().DropTable("audit_records"))

		// Log fails: the warning branch, no panic.
		assert.NotPanics(t, func() {
			svc.recordMfaAudit(ctx, "someone", audit.EventMFADisabled)
		})
	})
}

func TestBuildIdentityProviders_LDAPRequiresBindContext(t *testing.T) {
	// LDAP enabled with an addr, but baseDn/bindDn/userDnTemplate all missing: error.
	_, err := buildIdentityProviders(config.AuthProvidersConfig{
		LDAP: config.LDAPProviderConfig{Enabled: true, Addr: "ldap://x:389"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "baseDn")
}

func TestRefreshIdentityProviders_RebuildAndInvalid(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	ctx := context.Background()

	// Invalid config: returns an error and leaves the current state unchanged.
	require.False(t, svc.LDAPEnabled())
	err := svc.RefreshIdentityProviders(config.AuthProvidersConfig{
		LDAP: config.LDAPProviderConfig{Enabled: true, Addr: "ldap://x:389"},
	})
	require.Error(t, err)
	assert.False(t, svc.LDAPEnabled(), "failed refresh must not change providers")

	// Valid LDAP config: cascading rebuild (local stays first), default roles
	// take effect.
	err = svc.RefreshIdentityProviders(config.AuthProvidersConfig{
		LDAP: config.LDAPProviderConfig{
			Enabled:      true,
			Addr:         "ldap://x:389",
			BaseDN:       "dc=e,dc=c",
			BindDN:       "uid=svc",
			UserFilter:   "(uid=%s)",
			StartTLS:     true,
			DefaultRoles: []string{"viewer"},
		},
	})
	require.NoError(t, err)
	assert.True(t, svc.LDAPEnabled())
	_, _, _, roles := svc.snapshotProviders()
	assert.Equal(t, []string{"viewer"}, roles[identity.KindLDAP])
	// The local provider stays first in the cascade; local login is unaffected
	// by the refresh.
	createTestAdminWithRole(t, db, "refreshlocal", "pw", "ops")
	_, err = svc.Login(ctx, &LoginRequest{Username: "refreshlocal", Password: "pw"})
	require.NoError(t, err)

	// OIDC assembly success path (a local fake discovery server, no external
	// network dependency).
	srv := fakeDiscoveryServer(t)
	err = svc.RefreshIdentityProviders(config.AuthProvidersConfig{
		OIDC: config.OIDCProviderConfig{
			Enabled:         true,
			Issuer:          srv.URL,
			ClientID:        "croupier",
			ClientSecret:    "cs",
			RedirectURL:     "http://localhost/cb",
			LoginSuccessURL: "http://frontend/login",
		},
	})
	require.NoError(t, err)
	assert.True(t, svc.OIDCEnabled())
	assert.Equal(t, "http://frontend/login", svc.OIDCSuccessURL())
}

func TestLogin_LocalWriteFailuresStillSucceedOrFailCleanly(t *testing.T) {
	ctx := context.Background()

	t.Run("reset failures warn only", func(t *testing.T) {
		db := setupTestDB(t)
		svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
		createTestAdminWithRole(t, db, "resetwarn", "pw", "ops")
		readOnlyDB(t, db)

		// Correct password: ResetLoginFailures write failure only warns; login
		// still succeeds.
		resp, err := svc.Login(ctx, &LoginRequest{Username: "resetwarn", Password: "pw"})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Token)
	})

	t.Run("record failure warn only", func(t *testing.T) {
		db := setupTestDB(t)
		svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
		createTestAdminWithRole(t, db, "recordwarn", "pw", "ops")
		readOnlyDB(t, db)

		// Wrong password: RecordLoginFailure write failure only warns; the
		// generic error is still returned.
		_, err := svc.Login(ctx, &LoginRequest{Username: "recordwarn", Password: "bad"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "用户名或密码错误")
	})
}

func TestLogin_LocalProviderRecordVanished(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)

	// A local-kind stub authenticates, but no record exists in the database:
	// treated as invalid credentials.
	svc.WithPasswordProvider(stubPasswordProvider{kind: identity.KindLocal, username: "vanish"})
	_, err := svc.Login(context.Background(), &LoginRequest{Username: "vanish", Password: "pw"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "登录失败")
}

func TestLogin_BackfillUpdateFailsWarnsOnly(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)

	admin := &model.Admin{Username: "backfill", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "localpw"))

	svc.WithPasswordProvider(&fakePasswordProvider{
		kind:       identity.KindLDAP,
		validUsers: map[string]string{"backfill": "ldappw"},
		identities: map[string]*identity.Identity{
			"backfill": {Provider: identity.KindLDAP, Username: "backfill", Nickname: "B F", Email: "bf@x.example.com"},
		},
	})

	readOnlyDB(t, db)
	// Update write failure only warns; login succeeds.
	resp, err := svc.Login(context.Background(), &LoginRequest{Username: "backfill", Password: "ldappw"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
}

func TestLogin_AssignDefaultRoles_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("nil role model skips assignment", func(t *testing.T) {
		db := setupTestDB(t)
		// No WithRoleModel: after JIT provisioning, role assignment is skipped.
		svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
		svc.WithPasswordProvider(stubPasswordProvider{kind: identity.KindLDAP, username: "norolemodel"})
		resp, err := svc.Login(ctx, &LoginRequest{Username: "norolemodel", Password: "x"})
		require.NoError(t, err)
		assert.Empty(t, resp.User.Roles)
	})

	t.Run("role list failure skips missing roles", func(t *testing.T) {
		db := setupTestDB(t)
		svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db).WithRecoveryDB(db).
			WithRoleModel(model.NewRoleModel(db))
		svc.WithPasswordProvider(stubPasswordProvider{kind: identity.KindLDAP, username: "listfail"})
		svc.WithProviderDefaultRoles(identity.KindLDAP, []string{"viewer"})
		require.NoError(t, db.Migrator().DropTable("roles"))

		// findRoleByName query failure only warns and skips; then GetAdminRoles
		// fails on the same missing table and login errors out entirely (the
		// target branch has executed).
		_, err := svc.Login(ctx, &LoginRequest{Username: "listfail", Password: "x"})
		require.Error(t, err)
	})

	t.Run("assign role failure warns only", func(t *testing.T) {
		db := setupTestDB(t)
		svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db).WithRecoveryDB(db).
			WithRoleModel(model.NewRoleModel(db))
		svc.WithPasswordProvider(stubPasswordProvider{kind: identity.KindLDAP, username: "assignfail"})
		svc.WithProviderDefaultRoles(identity.KindLDAP, []string{"viewer"})

		require.NoError(t, model.NewRoleModel(db).Create(ctx, &model.Role{Name: "viewer"}))
		require.NoError(t, db.Migrator().DropTable("admin_roles"))

		// AssignRole write failure only warns; GetAdminRoles reading the same
		// missing table makes login error out (the target branch has executed).
		_, err := svc.Login(ctx, &LoginRequest{Username: "assignfail", Password: "x"})
		require.Error(t, err)
	})
}

func TestLogin_EmptySecretTokenGenerationFails(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "").WithRecoveryDB(db)
	createTestAdminWithRole(t, db, "signfail", "pw", "ops")

	_, err := svc.Login(context.Background(), &LoginRequest{Username: "signfail", Password: "pw"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "生成 token 失败")
}

func TestOIDCLoginCallback_ProvisionFails(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	fake := &fakeOAuthProvider{
		authURL: "https://idp/auth",
		ident:   &identity.Identity{Provider: identity.KindOIDC, Username: "provisionfail"},
	}
	svc.WithOIDCProvider(fake, nil, "")

	// admins table unavailable: JIT provisioning fails → login fails.
	require.NoError(t, db.Migrator().DropTable("admins"))
	state := svc.newOIDCState()

	_, err := svc.OIDCLoginCallback(context.Background(), "code", state, &LoginRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "登录失败")
}

func TestVerifyOIDCState_MalformedPayloads(t *testing.T) {
	svc := newOIDCService(t, nil, "")

	// payload is not valid base64url.
	assert.False(t, svc.verifyOIDCState("!!!.deadbeef"))

	b64 := base64.RawURLEncoding.EncodeToString([]byte("abc"))
	sig := svc.signState("abc")
	// payload is valid base64 but lacks the nonce.timestamp structure.
	assert.False(t, svc.verifyOIDCState(b64+"."+sig))

	b64 = base64.RawURLEncoding.EncodeToString([]byte("abc.xyz"))
	sig = svc.signState("abc.xyz")
	// timestamp is not numeric.
	assert.False(t, svc.verifyOIDCState(b64+"."+sig))
}

func TestValidLastScope_EnvScopeQueryFails(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "secret").WithRecoveryDB(db).
		WithGameModel(gameModel)

	game := &model.Game{Name: "scopegame", Status: "running"}
	require.NoError(t, gameModel.Create(context.Background(), game))
	require.NoError(t, gameModel.AddEnvBinding(context.Background(), game.GameID, "prod", "test", "", ""))

	admin := &model.Admin{Username: "scopeviewer", Nickname: "V", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "pw"))
	role := &model.Role{Name: "viewer"}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	// scope table query failure: falls back to empty scope, no panic.
	require.NoError(t, db.Migrator().DropTable("admin_game_env_scopes"))
	gameID, env := svc.validLastScope(context.Background(), admin.ID, []string{"viewer"}, game.GameID, "prod")
	assert.Empty(t, gameID)
	assert.Empty(t, env)
}

func TestLogout_BumpsTokenVersion(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	createTestAdminWithRole(t, db, "bumpme", "pw", "ops")

	before, err := adminModel.FindByUsername(ctx, "bumpme")
	require.NoError(t, err)

	svc.Logout(ctx, &LogoutRequest{Username: "bumpme"})

	after, err := adminModel.FindByUsername(ctx, "bumpme")
	require.NoError(t, err)
	assert.Equal(t, before.TokenVersion+1, after.TokenVersion)

	// Logout for a missing user: skips the bump and returns normally.
	svc.Logout(ctx, &LogoutRequest{Username: "ghost"})

	// bump write failure: only warns, no error returned (a different role name
	// avoids the name unique conflict).
	// Logout's signature was tightened (no failure path); this verifies it does
	// not panic on a read-only database either.
	createTestAdminWithRole(t, db, "bumpfail", "pw", "ops2")
	readOnlyDB(t, db)
	svc.Logout(ctx, &LogoutRequest{Username: "bumpfail"})
}

func TestCheck_PermissionServiceError(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	createTestAdminWithRole(t, db, "permerr", "pw", "ops")

	// Invalid resource: CheckPermission returns an error, and Check converts it
	// to allowed=false + reason.
	resp, err := svc.Check(context.Background(), "permerr", &CheckRequest{
		Resource: "not a valid resource!",
		Action:   "read",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Allowed)
	assert.NotEmpty(t, resp.Reason)
}

func TestCheck_ValidResourceButDenied(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	createTestAdminWithRole(t, db, "deniedvalid", "pw", "ops")

	// Valid resource/action but the account has no grants: takes the
	// permission denied branch.
	resp, err := svc.Check(context.Background(), "deniedvalid", &CheckRequest{
		Resource: "game",
		Action:   "restart",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Allowed)
	assert.Equal(t, "permission denied", resp.Reason)
}
