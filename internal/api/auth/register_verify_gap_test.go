package auth

// 覆盖率巡检补测·第二批（auth 包残余 23 块中的 13 块）：本文件只补测试、
// 不改源码，按「缺口类型」分三组：
//
//  1. 装配翼（providers.go Attach 的 GitHub/WeChat/自定义 OAuth 三路接线；
//     service.go 本地停用但 LDAP 在 → 密码级联只剩 LDAP）；
//  2. 注册失败翼（admins 表缺 → 建号失败走审计 + 兜底文案；默认角色绑定
//     表缺 → 告警跳过；邮箱域白名单里的空条目；别名比对时同域脏数据）；
//  3. 邮箱验证翼（验证模型未装配 / 令牌表缺 / admins 表缺导致消费事务失败）。
//
// 明确登记为**确定性不可达**（不在此造例，防线保留）：
//   - providers.go 三处「provider init failed 降级」：上游已在同函数内先行
//     拦截空凭证返回错误，构造器在唯一调用点不可能再失败；
//   - email_verification.go「生成验证令牌失败」：crypto/rand.Read 在 Go≥1.24
//     合同上不再返回错误（go.mod go 1.26.6），属 dead-by-contract——比原
//     「无注入点」登记更强的判死，误报风险清零；
//   - VerifyEmailToken 的「!ok（并发消费）」：R51 翻案收口——sqlite
//     BEFORE UPDATE TRIGGER + RAISE(IGNORE) 让 Consume 的条件 UPDATE 静默
//     0 行（RowsAffected=0 → consumed=false），确定性触达（见
//     email_verification_r51_test.go，原「无钩子」登记被 RAISE(IGNORE) 技法
//     击破）；
//   - siteServerURL 的 settings 单例为空翼：R51 收口——ResetForTest 置
//     layered=nil 后调用即得空串，包内 t.Parallel 用例不读 settings 单例
//     （零引用），互扰前提不成立（见 email_verification_r51_test.go）；
//   - service.go 别名查重里的「同域行缺 @ → continue」：FindEmailsByDomain
//     的 SQL 谓词是 email LIKE '%@'||domain，每条返回行必然含 '@'，
//     strings.Cut 恒成功；脏数据在 SQL 层就被挡掉（本文件用例会证明它
//     不会误判邮箱已占用）。

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

// ---- 1. 装配翼 ----

// Attach 把三个重定向型提供方挂到 Service：此前只有 LDAP/OIDC 两路被装配
// 过，GitHub/WeChat/自定义 OAuth 三路在生产装配路径上无任何用例。
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

// 本地停用但 LDAP 在：密码级联只剩 LDAP（本地提供方整体移除，登录由外部
// 身份源承担）。
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

// ---- 2. 注册失败翼 ----

// registerGapFixture 与包内 registerFixture 同构，但额外返回 db 句柄——
// 缺表注入需要直接操作 schema。
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

// admins 表缺：查重跳过（err != nil 不算已存在）→ 建号失败 → 记失败审计
// 并返回兜底文案（不把底层 DB 错误原样抛给匿名端点）。
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

// 默认角色存在但绑定表缺：角色查找成功、绑定失败 → 仅告警，注册本身成功
// （绑定失败不得回滚建号）。
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

// 白名单串里的空条目（", ,example.com"）被跳过而非误判为不允许的域。
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

// 别名限制开启时，库里同域之外的脏邮箱行（无 @）不会污染查重结果：新邮箱仍
// 可注册，不被误判成「已被其他账号占用」。
func TestRegister_EmailAliasRestrictionIgnoresNonMatchingDomainRows(t *testing.T) {
	seedEmailPolicy(t, map[string]string{settings.KeyAuthEmailAliasRestriction: `true`})
	svc := registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})
	// 直写一条无 @ 的脏邮箱：同域 LIKE 谓词挡在 SQL 层，别名比对拿不到它，
	// 也就不会把别的域/脏行误判成「该邮箱已被占用」。
	require.NoError(t, svc.adminModel.Create(context.Background(),
		&model.Admin{Username: "dirtymail", Email: "corrupted", Status: 1}, "Str0ngPass!x"))

	admin, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "aliasok1", Password: "Str0ngPass!x", Email: "real.user@example.com",
	})
	require.NoError(t, err, "脏数据行被跳过，不误判邮箱已占用")
	assert.Equal(t, "real.user@example.com", admin.Email)
}

// ---- 3. 邮箱验证翼 ----

// 验证模型未装配（早期部署/未接表）：签发静默返回 nil，校验报「验证服务
// 不可用」——不得 panic，也不得放行任何令牌。
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

// 令牌表缺：签发与校验都失败；注册后的自动签发仅告警不阻断注册。
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

// admins 表缺：Consume 的事务里回写 admins.email_verified 失败 → 整体回滚
// 且错误上抛（令牌不会被消费掉，等价于验证未完成）。
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
