package auth

// Coverage sweep follow-up tests, batch 2 (13 of the 23 remaining blocks in the auth
// package): this file only adds tests and does not change source code. Grouped into
// three buckets by gap type:
//
//  1. Wiring wing (the GitHub/WeChat/custom OAuth three-way wiring in providers.go
//     Attach; service.go with local disabled but LDAP present -> the password cascade
//     is reduced to LDAP only);
//  2. Registration failure wing (missing admins table -> account creation fails via
//     audit + fallback text; missing default role binding table -> skip with a
//     warning; empty entries in the email domain whitelist; same-domain dirty rows
//     during alias comparison);
//  3. Email verification wing (verification model not wired / missing token table /
//     missing admins table causing the consume transaction to fail).
//
// Explicitly registered as **deterministically unreachable** (no test cases built
// here; the defense lines are kept):
//   - The three "provider init failed" degradations in providers.go: upstream already
//     intercepts empty credentials earlier in the same function and returns an error,
//     so the constructor cannot fail again at its only call site;
//   - "failed to generate verification token" in email_verification.go: crypto/rand.Read
//     no longer returns an error per contract on Go>=1.24 (go.mod go 1.26.6), which is
//     dead-by-contract - a stronger verdict than the earlier "no injection point"
//     registration, with false-positive risk eliminated;
//   - VerifyEmailToken's "!ok (concurrent consume)": R51 overturns and closes out the
//     original registration - a sqlite BEFORE UPDATE TRIGGER + RAISE(IGNORE) makes
//     Consume's conditional UPDATE silently affect 0 rows (RowsAffected=0 ->
//     consumed=false), deterministically reachable (see email_verification_r51_test.go;
//     the original "no hook" registration was broken open by the RAISE(IGNORE)
//     technique);
//   - The "empty settings singleton" wing for siteServerURL: closed out in R51 -
//     calling after ResetForTest sets layered=nil yields an empty string; in-package
//     t.Parallel cases do not read the settings singleton (zero references), so the
//     premise for interference does not hold (see email_verification_r51_test.go);
//   - The "same-domain row missing @ -> continue" branch in service.go alias
//     deduplication: FindEmailsByDomain's SQL predicate is email LIKE '%@'||domain,
//     so every returned row necessarily contains '@' and strings.Cut always succeeds;
//     dirty data is blocked at the SQL layer (the cases in this file prove it does
//     not falsely judge an email as taken).

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- 1. Wiring wing ----

// Attach wires the three redirect-type providers onto the Service: previously only
// the LDAP/OIDC two paths were ever wired, and GitHub/WeChat/custom OAuth had no
// coverage on the production assembly path.
func TestIdentityProviders_AttachRedirectProviders(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), jwtutil.DevSecret()).
		WithRecoveryDB(db).WithRoleModel(model.NewRoleModel(db))

	ip, err := BuildIdentityProviders(config.AuthProvidersConfig{
		GitHub: config.GitHubProviderConfig{
			Enabled: true, ClientID: "cid", ClientSecret: "cs",
			RedirectURL:  "https://gm.example.com/api/v1/auth/github/callback",
			DefaultRoles: []string{"viewer"}, LoginSuccessURL: "https://gm.example.com/gh-done",
		},
		WeChat: config.WeChatProviderConfig{
			Enabled: true, AppID: "wx1", AppSecret: "secret",
			RedirectURL:     "https://gm.example.com/api/v1/auth/wechat/callback",
			LoginSuccessURL: "https://gm.example.com/wx-done",
		},
		GenericOAuth: config.GenericOAuthProviderConfig{
			Enabled: true, ClientID: "cid", ClientSecret: "cs",
			RedirectURL:     "https://gm.example.com/api/v1/auth/generic/callback",
			AuthURL:         "https://idp.example.com/authorize",
			TokenURL:        "https://idp.example.com/token",
			UserInfoURL:     "https://idp.example.com/userinfo",
			LoginSuccessURL: "https://gm.example.com/g-done",
		},
	})
	require.NoError(t, err)
	attached := ip.Attach(svc)

	assert.True(t, attached.GitHubEnabled())
	assert.True(t, attached.WeChatEnabled())
	assert.True(t, attached.GenericOAuthEnabled())
	assert.Equal(t, "https://gm.example.com/gh-done", attached.GitHubSuccessURL())
	assert.Equal(t, "https://gm.example.com/wx-done", attached.WeChatSuccessURL())
	assert.Equal(t, "https://gm.example.com/g-done", attached.GenericOAuthSuccessURL())
}

// Local disabled but LDAP present: the password cascade is reduced to LDAP only (the
// local provider is removed wholesale; login is carried by the external identity source).
func TestRefreshIdentityProviders_LocalOffKeepsLDAPOnly(t *testing.T) {
	svc, err := refreshWith(t, config.AuthProvidersConfig{
		Local: config.LocalProviderConfig{Enabled: localOff()},
		LDAP: config.LDAPProviderConfig{
			Enabled: true, Addr: "ldap://x:389", BaseDN: "dc=e,dc=c",
			BindDN: "uid=svc", UserFilter: "(uid=%s)", DefaultRoles: []string{"viewer"},
		},
	})
	require.NoError(t, err)
	assert.False(t, svc.LocalEnabled())
	assert.True(t, svc.LDAPEnabled())

	passwords, _, _, roles := svc.snapshotProviders()
	require.Len(t, passwords, 1, "本地停用后密码级联只剩 LDAP 一项")
	assert.Equal(t, []string{"viewer"}, roles["ldap"])
}

// ---- 2. Registration failure wing ----

// registerGapFixture mirrors the in-package registerFixture but additionally returns
// the db handle - missing-table injection needs direct schema manipulation.
func registerGapFixture(t *testing.T, cfg config.AuthProvidersConfig) (*Service, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	require.NoError(t, model.NewRoleModel(db).Create(context.Background(), &model.Role{Name: "viewer"}))
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), jwtutil.DevSecret()).
		WithRecoveryDB(db).
		WithRoleModel(model.NewRoleModel(db))
	ip, err := BuildIdentityProviders(cfg)
	require.NoError(t, err)
	return ip.Attach(svc), db
}

// Missing admins table: dedup is skipped (err != nil does not count as existing) ->
// account creation fails -> a failure audit is recorded and the fallback text is
// returned (the raw underlying DB error is not thrown at the anonymous endpoint).
func TestRegister_AdminTableMissing(t *testing.T) {
	svc, db := registerGapFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})
	require.NoError(t, db.Migrator().DropTable(&model.Admin{}))

	admin, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "ghostuser", Password: "Str0ngPass!x",
	})
	require.Error(t, err)
	assert.Nil(t, admin)
	assert.Contains(t, err.Error(), "注册失败")
}

// Default role exists but the binding table is missing: role lookup succeeds, binding
// fails -> warning only, and registration itself succeeds (a binding failure must not
// roll back account creation).
func TestRegister_DefaultRoleAssignFailureIsNonFatal(t *testing.T) {
	svc, db := registerGapFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true, DefaultRoles: []string{"viewer"}},
	})
	require.NoError(t, db.Migrator().DropTable(&model.AdminRole{}))

	admin, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "rolefail1", Password: "Str0ngPass!x",
	})
	require.NoError(t, err, "角色绑定失败只告警，不回滚注册")
	assert.Equal(t, "rolefail1", admin.Username)
}

// Empty entries in the whitelist string (", ,example.com") are skipped rather than
// misjudged as disallowed domains.
func TestRegister_EmailWhitelistSkipsEmptyEntries(t *testing.T) {
	seedEmailPolicy(t, map[string]string{
		settings.KeyAuthEmailDomainWhitelist: `" , ,example.com"`,
	})
	svc := registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})

	admin, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "wlblank1", Password: "Str0ngPass!x", Email: "a@example.com",
	})
	require.NoError(t, err, "空条目跳过，example.com 命中白名单")
	assert.Equal(t, "a@example.com", admin.Email)

	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "wlblank2", Password: "Str0ngPass!x", Email: "b@other.com",
	})
	require.Error(t, err, "非白名单域仍被拒")
	assert.Contains(t, err.Error(), "不在允许清单内")
}

// With alias restriction enabled, dirty email rows in the database outside the same
// domain (no @) do not pollute the dedup result: a new email can still register and is
// not misjudged as "already taken by another account".
func TestRegister_EmailAliasRestrictionIgnoresNonMatchingDomainRows(t *testing.T) {
	seedEmailPolicy(t, map[string]string{settings.KeyAuthEmailAliasRestriction: `true`})
	svc := registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})
	// Write a dirty email without '@' directly: the same-domain LIKE predicate blocks it
	// at the SQL layer, so the alias comparison never sees it and cannot misjudge other
	// domains/dirty rows as "this email is already taken".
	require.NoError(t, svc.adminModel.Create(context.Background(),
		&model.Admin{Username: "dirtymail", Email: "corrupted", Status: 1}, "Str0ngPass!x"))

	admin, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "aliasok1", Password: "Str0ngPass!x", Email: "real.user@example.com",
	})
	require.NoError(t, err, "脏数据行被跳过，不误判邮箱已占用")
	assert.Equal(t, "real.user@example.com", admin.Email)
}

// ---- 3. Email verification wing ----

// Verification model not wired (early deployments / table not attached): issuing
// silently returns nil and verification reports "verification service unavailable" -
// no panic, and no token is ever accepted.
func TestEmailVerification_ModelNotWired(t *testing.T) {
	svc := registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})
	require.Nil(t, svc.verificationModel, "夹具刻意不装配验证模型")

	admin := &model.Admin{Username: "nomodel", Email: "a@example.com"}
	assert.NoError(t, svc.issueEmailVerification(context.Background(), admin),
		"未装配验证模型时签发静默成功（无令牌可签）")

	err := svc.VerifyEmailToken(context.Background(), "any-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "验证服务不可用")
}

// Missing token table: both issuing and verification fail; the automatic issue after
// registration only warns and does not block registration.
func TestEmailVerification_TokenTableMissing(t *testing.T) {
	svc, _, db := verificationFixture(t)
	sent, restore := hookVerificationSender(t, nil)
	defer restore()

	require.NoError(t, db.Migrator().DropTable(&model.EmailVerification{}))

	admin, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "notbl1", Password: "Str0ngPass!x", Email: "notbl1@example.com",
	})
	require.NoError(t, err, "签发失败只告警，注册结果不受影响")
	assert.False(t, admin.EmailVerified)
	assert.Empty(t, *sent, "签发失败时不会有邮件外发")

	err = svc.VerifyEmailToken(context.Background(), "whatever")
	require.Error(t, err, "令牌表缺时校验失败（不得放行）")
}

// Missing admins table: writing back admins.email_verified inside Consume's
// transaction fails -> everything rolls back and the error propagates (the token is
// not consumed; equivalent to verification not completed).
func TestEmailVerification_ConsumeFailureRollsBack(t *testing.T) {
	svc, _, db := verificationFixture(t)
	sent, restore := hookVerificationSender(t, nil)
	defer restore()

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "rollback1", Password: "Str0ngPass!x", Email: "rollback1@example.com",
	})
	require.NoError(t, err)
	require.NotEmpty(t, *sent)
	token := (*sent)[0].token

	require.NoError(t, db.Migrator().DropTable(&model.Admin{}))
	err = svc.VerifyEmailToken(context.Background(), token)
	require.Error(t, err, "消费事务失败须上抛")
	assert.NotContains(t, err.Error(), "验证链接无效", "是 DB 失败而非令牌无效")
}
