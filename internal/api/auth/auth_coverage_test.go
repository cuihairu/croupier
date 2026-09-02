// 覆盖目标：handler 层 MFA 三个端点与 Login 的 mfa_required 分支、
// mfa.go 全部错误分支与审计记录、RefreshIdentityProviders、
// Login/Logout/Check/OIDC state 校验的错误与降级路径。
// 错误注入手段：PRAGMA query_only（读成功写失败）、drop 表、测试替身。
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

// readOnlyDB 把连接池收敛到单连接并置 query_only：读操作照常、写操作
// 必失败，用于覆盖"读成功后写失败"的告警分支。
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
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")
	ctx := context.Background()
	createTestAdminWithRole(t, db, "mfahttp", "CorrectPass123", "admin")

	setup, err := svc.MFASetup(ctx, "mfahttp")
	require.NoError(t, err)
	require.NoError(t, svc.MFAConfirm(ctx, "mfahttp", currentTOTP(t, setup.Secret)))

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
	c.Set("username", 12345) // 非字符串
	assert.Empty(t, mfaUsername(c))

	c, _ = newAuthTestContext("POST", "/x", "")
	c.Set("username", "alice")
	assert.Equal(t, "alice", mfaUsername(c))
}

func TestHandler_MFAEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")
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

	// 未登录：401。
	for _, path := range []string{"/mfa/setup", "/mfa/confirm", "/mfa/disable"} {
		rec := post(path, `{}`, nil)
		assertHTTPStatus(t, rec, http.StatusUnauthorized)
	}

	// setup 成功：返回 secret。
	rec := post("/mfa/setup", ``, "mfaep")
	assertHTTPStatus(t, rec, http.StatusOK)
	var setup MFASetupResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &setup))
	require.NotEmpty(t, setup.Secret)

	// confirm：缺 code 绑定失败 400；用户不存在 400；正确 code 200。
	assertHTTPStatus(t, post("/mfa/confirm", `{}`, "mfaep"), http.StatusBadRequest)
	assertHTTPStatus(t, post("/mfa/confirm", `{"code":"000000"}`, "ghost"), http.StatusBadRequest)
	assertHTTPStatus(t, post("/mfa/confirm", `{"code":"`+currentTOTP(t, setup.Secret)+`"}`, "mfaep"), http.StatusOK)

	// disable：缺字段 400；错 code 400；正确 code+密码 200。
	assertHTTPStatus(t, post("/mfa/disable", `{"code":"000000"}`, "mfaep"), http.StatusBadRequest)
	assertHTTPStatus(t, post("/mfa/disable", `{"code":"000000","password":"x"}`, "mfaep"), http.StatusBadRequest)
	assertHTTPStatus(t, post("/mfa/disable",
		`{"code":"`+currentTOTP(t, setup.Secret)+`","password":"CorrectPass123"}`, "mfaep"), http.StatusOK)

	// setup 时 service 错误（用户不存在）：400。
	assertHTTPStatus(t, post("/mfa/setup", ``, "ghost"), http.StatusBadRequest)
}

// setup 端点在 body 为空时也能正常工作（无绑定字段）。

func TestMFAService_ErrorBranches(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")
	ctx := context.Background()
	createTestAdminWithRole(t, db, "mfabranch", "CorrectPass123", "admin")

	// 用户不存在。
	_, err := svc.MFASetup(ctx, "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户不存在")
	require.Error(t, svc.MFAConfirm(ctx, "ghost", "000000"))
	require.Error(t, svc.MFADisable(ctx, "ghost", "000000", "pw"))

	// 影子账号（外部身份源，PasswordHash 为空）。
	shadow := &model.Admin{Username: "extshadow", Status: 1}
	require.NoError(t, db.Create(shadow).Error)
	_, err = svc.MFASetup(ctx, "extshadow")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "外部身份源账号")

	// confirm 未先 setup：提示先获取密钥。
	err = svc.MFAConfirm(ctx, "mfabranch", "000000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "请先获取二次验证密钥")

	// disable 未启用。
	err = svc.MFADisable(ctx, "mfabranch", "000000", "pw")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未启用二次验证")

	// 启用后 disable 错误验证码。
	setup, err := svc.MFASetup(ctx, "mfabranch")
	require.NoError(t, err)
	require.NoError(t, svc.MFAConfirm(ctx, "mfabranch", currentTOTP(t, setup.Secret)))
	err = svc.MFADisable(ctx, "mfabranch", "000000", "CorrectPass123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "验证码错误")
}

func TestMFAService_StoreWriteFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("setup save secret fails", func(t *testing.T) {
		db := setupTestDB(t)
		adminModel := model.NewAdminModel(db)
		svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")
		createTestAdminWithRole(t, db, "saveme", "CorrectPass123", "admin")

		readOnlyDB(t, db)
		_, err := svc.MFASetup(ctx, "saveme")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "保存密钥失败")
	})

	t.Run("confirm enable fails", func(t *testing.T) {
		db := setupTestDB(t)
		adminModel := model.NewAdminModel(db)
		svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")
		createTestAdminWithRole(t, db, "enableme", "CorrectPass123", "admin")

		setup, err := svc.MFASetup(ctx, "enableme")
		require.NoError(t, err)
		readOnlyDB(t, db)
		err = svc.MFAConfirm(ctx, "enableme", currentTOTP(t, setup.Secret))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "启用失败")
	})

	t.Run("disable fails", func(t *testing.T) {
		db := setupTestDB(t)
		adminModel := model.NewAdminModel(db)
		svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")
		createTestAdminWithRole(t, db, "disableme", "CorrectPass123", "admin")

		setup, err := svc.MFASetup(ctx, "disableme")
		require.NoError(t, err)
		require.NoError(t, svc.MFAConfirm(ctx, "disableme", currentTOTP(t, setup.Secret)))
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
		svc := withTableAudit(t, db, NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret"))
		createTestAdminWithRole(t, db, "auditmfa", "CorrectPass123", "admin")

		setup, err := svc.MFASetup(ctx, "auditmfa")
		require.NoError(t, err)
		require.NoError(t, svc.MFAConfirm(ctx, "auditmfa", currentTOTP(t, setup.Secret)))

		row := lastAuditRow(t, db)
		assert.Equal(t, string(audit.EventMFAEnabled), row["event_type"])
		assert.Equal(t, "auditmfa", row["actor_id"])
	})

	t.Run("audit store failure logs and continues", func(t *testing.T) {
		db := setupTestDB(t)
		svc := withTableAudit(t, db, NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret"))
		require.NoError(t, db.Migrator().DropTable("audit_records"))

		// Log 失败：告警分支，不 panic。
		assert.NotPanics(t, func() {
			svc.recordMfaAudit(ctx, "someone", audit.EventMFADisabled)
		})
	})
}

func TestBuildIdentityProviders_LDAPRequiresBindContext(t *testing.T) {
	// LDAP 启用且有 addr，但 baseDn/bindDn/userDnTemplate 全缺：报错。
	_, err := buildIdentityProviders(config.AuthProvidersConfig{
		LDAP: config.LDAPProviderConfig{Enabled: true, Addr: "ldap://x:389"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "baseDn")
}

func TestRefreshIdentityProviders_RebuildAndInvalid(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret")
	ctx := context.Background()

	// 无效配置：返回错误且现状不变。
	require.False(t, svc.LDAPEnabled())
	err := svc.RefreshIdentityProviders(config.AuthProvidersConfig{
		LDAP: config.LDAPProviderConfig{Enabled: true, Addr: "ldap://x:389"},
	})
	require.Error(t, err)
	assert.False(t, svc.LDAPEnabled(), "failed refresh must not change providers")

	// 合法 LDAP 配置：级联重建（本地保留首位），默认角色生效。
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
	// 本地提供方仍是级联首位，本地登录不受刷新影响。
	createTestAdminWithRole(t, db, "refreshlocal", "pw", "ops")
	_, err = svc.Login(ctx, &LoginRequest{Username: "refreshlocal", Password: "pw"})
	require.NoError(t, err)

	// OIDC 装配成功路径（本地假发现服务器，无外部网络依赖）。
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
		svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret")
		createTestAdminWithRole(t, db, "resetwarn", "pw", "ops")
		readOnlyDB(t, db)

		// 密码正确：ResetLoginFailures 写失败仅告警，登录仍成功。
		resp, err := svc.Login(ctx, &LoginRequest{Username: "resetwarn", Password: "pw"})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Token)
	})

	t.Run("record failure warn only", func(t *testing.T) {
		db := setupTestDB(t)
		svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret")
		createTestAdminWithRole(t, db, "recordwarn", "pw", "ops")
		readOnlyDB(t, db)

		// 密码错误：RecordLoginFailure 写失败仅告警，仍返回通用错误。
		_, err := svc.Login(ctx, &LoginRequest{Username: "recordwarn", Password: "bad"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "用户名或密码错误")
	})
}

func TestLogin_LocalProviderRecordVanished(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret")

	// 本地 kind 的替身认证通过，但库里无此记录：视为凭证失效。
	svc.WithPasswordProvider(stubPasswordProvider{kind: identity.KindLocal, username: "vanish"})
	_, err := svc.Login(context.Background(), &LoginRequest{Username: "vanish", Password: "pw"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "登录失败")
}

func TestLogin_BackfillUpdateFailsWarnsOnly(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")

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
	// Update 写失败仅告警，登录成功。
	resp, err := svc.Login(context.Background(), &LoginRequest{Username: "backfill", Password: "ldappw"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
}

func TestLogin_AssignDefaultRoles_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("nil role model skips assignment", func(t *testing.T) {
		db := setupTestDB(t)
		// 未 WithRoleModel：JIT 建号后直接跳过角色赋权。
		svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret")
		svc.WithPasswordProvider(stubPasswordProvider{kind: identity.KindLDAP, username: "norolemodel"})
		resp, err := svc.Login(ctx, &LoginRequest{Username: "norolemodel", Password: "x"})
		require.NoError(t, err)
		assert.Empty(t, resp.User.Roles)
	})

	t.Run("role list failure skips missing roles", func(t *testing.T) {
		db := setupTestDB(t)
		svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").
			WithRoleModel(model.NewRoleModel(db))
		svc.WithPasswordProvider(stubPasswordProvider{kind: identity.KindLDAP, username: "listfail"})
		svc.WithProviderDefaultRoles(identity.KindLDAP, []string{"viewer"})
		require.NoError(t, db.Migrator().DropTable("roles"))

		// findRoleByName 查询失败仅告警跳过；随后 GetAdminRoles 因同一
		// 缺表失败，登录整体报错（目标分支已执行）。
		_, err := svc.Login(ctx, &LoginRequest{Username: "listfail", Password: "x"})
		require.Error(t, err)
	})

	t.Run("assign role failure warns only", func(t *testing.T) {
		db := setupTestDB(t)
		svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").
			WithRoleModel(model.NewRoleModel(db))
		svc.WithPasswordProvider(stubPasswordProvider{kind: identity.KindLDAP, username: "assignfail"})
		svc.WithProviderDefaultRoles(identity.KindLDAP, []string{"viewer"})

		require.NoError(t, model.NewRoleModel(db).Create(ctx, &model.Role{Name: "viewer"}))
		require.NoError(t, db.Migrator().DropTable("admin_roles"))

		// AssignRole 写失败仅告警；GetAdminRoles 读同一缺表导致登录报错
		//（目标分支已执行）。
		_, err := svc.Login(ctx, &LoginRequest{Username: "assignfail", Password: "x"})
		require.Error(t, err)
	})
}

func TestLogin_EmptySecretTokenGenerationFails(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "")
	createTestAdminWithRole(t, db, "signfail", "pw", "ops")

	_, err := svc.Login(context.Background(), &LoginRequest{Username: "signfail", Password: "pw"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "生成 token 失败")
}

func TestOIDCLoginCallback_ProvisionFails(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret")
	fake := &fakeOAuthProvider{
		authURL: "https://idp/auth",
		ident:   &identity.Identity{Provider: identity.KindOIDC, Username: "provisionfail"},
	}
	svc.WithOIDCProvider(fake, nil, "")

	// admins 表不可用：JIT 解析失败 → 登录失败。
	require.NoError(t, db.Migrator().DropTable("admins"))
	state, err := svc.newOIDCState()
	require.NoError(t, err)

	_, err = svc.OIDCLoginCallback(context.Background(), "code", state, &LoginRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "登录失败")
}

func TestVerifyOIDCState_MalformedPayloads(t *testing.T) {
	svc := newOIDCService(t, nil, "")

	// payload 非法 base64url。
	assert.False(t, svc.verifyOIDCState("!!!.deadbeef"))

	b64 := base64.RawURLEncoding.EncodeToString([]byte("abc"))
	sig := svc.signState("abc")
	// payload 合法 base64 但缺 nonce.timestamp 结构。
	assert.False(t, svc.verifyOIDCState(b64+"."+sig))

	b64 = base64.RawURLEncoding.EncodeToString([]byte("abc.xyz"))
	sig = svc.signState("abc.xyz")
	// timestamp 非数字。
	assert.False(t, svc.verifyOIDCState(b64+"."+sig))
}

func TestValidLastScope_EnvScopeQueryFails(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "secret").
		WithGameModel(gameModel)

	game := &model.Game{Name: "scopegame", Status: "running"}
	require.NoError(t, gameModel.Create(context.Background(), game))
	require.NoError(t, gameModel.AddEnvBinding(context.Background(), game.GameID, "prod", "test", "", ""))

	admin := &model.Admin{Username: "scopeviewer", Nickname: "V", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "pw"))
	role := &model.Role{Name: "viewer"}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	// scope 表查询失败：回落空 scope，不 panic。
	require.NoError(t, db.Migrator().DropTable("admin_game_env_scopes"))
	gameID, env := svc.validLastScope(context.Background(), admin.ID, []string{"viewer"}, game.GameID, "prod")
	assert.Empty(t, gameID)
	assert.Empty(t, env)
}

func TestLogout_BumpsTokenVersion(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")
	createTestAdminWithRole(t, db, "bumpme", "pw", "ops")

	before, err := adminModel.FindByUsername(ctx, "bumpme")
	require.NoError(t, err)

	_, err = svc.Logout(ctx, &LogoutRequest{Username: "bumpme"})
	require.NoError(t, err)

	after, err := adminModel.FindByUsername(ctx, "bumpme")
	require.NoError(t, err)
	assert.Equal(t, before.TokenVersion+1, after.TokenVersion)

	// 用户不存在的 Logout：跳过 bump，正常返回。
	_, err = svc.Logout(ctx, &LogoutRequest{Username: "ghost"})
	require.NoError(t, err)

	// bump 写失败：仅告警，不返回错误（换角色名避开 name 唯一冲突）。
	createTestAdminWithRole(t, db, "bumpfail", "pw", "ops2")
	readOnlyDB(t, db)
	_, err = svc.Logout(ctx, &LogoutRequest{Username: "bumpfail"})
	require.NoError(t, err)
}

func TestCheck_PermissionServiceError(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret")
	createTestAdminWithRole(t, db, "permerr", "pw", "ops")

	// 非法 resource：CheckPermission 返回错误，Check 转 allowed=false + reason。
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
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret")
	createTestAdminWithRole(t, db, "deniedvalid", "pw", "ops")

	// 合法 resource/action 但账号无任何授权：走 permission denied 分支。
	resp, err := svc.Check(context.Background(), "deniedvalid", &CheckRequest{
		Resource: "game",
		Action:   "restart",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Allowed)
	assert.Equal(t, "permission denied", resp.Reason)
}
