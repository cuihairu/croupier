package svc

// 覆盖率巡检第十一轮（wt-api）：svc 包残余 19 块收口——
// migrations.go 8 块（0036 announcement_games / 0037 email_verifications /
// 0038 cicd 双表的 wrapGorm 探针失败翼 + CreateTable/AddColumn 拒写翼，
// 完全复用 C/D 批注入口径）；
// service_context.go 11 块（LogRetention.ResolveDays 闭包在 settings
// 单例空窗期的 (0,0) 短路、引导管理员 phone 档案回填主链与回填被拦
// 降级翼、AuthMiddleware.Handle 触达 mfaGate 的 403 返回、
// cachedOTPEnabled 缓存命中 / TTL 过期回源 / 存储故障 fail-open）。
// 回避他会话在途域（git status ?? 清单）：api/announcement、api/auth×2、
// api/extension、api/sitesettings、security/identity——与本文件目录不相交。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------- migrations.go：0036/0037/0038 三迁移错误翼 ----------

// 探针失败连接：三迁移体的 wrapGorm err 透传分支。
func TestCoverageE_Migrations0036To0038_WrapGormProbeError(t *testing.T) {
	db := probeFailingDBC()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	require.Error(t, migrateAnnouncementGamesTable(ctx, db))
	require.Error(t, migrateEmailVerification(ctx, db))
	require.Error(t, migrateCicdTables(ctx, db))
}

// 0036：announcement_games 缺表 → CreateTable 失败（query_only 拒写）。
func TestCoverageE_Mig0036_CreateTableError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateAnnouncementGamesTable(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0036")
}

// 0037：email_verifications 缺表 → CreateTable 失败。
func TestCoverageE_Mig0037_CreateEmailVerificationError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateEmailVerification(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0037")
}

// 0037：ev 表已在（跳过建表）、admins 为手工最小表（无 email_verified 列）
// → AddColumn 失败。手工建表而非 AutoMigrate——后者会带出全部列，
// HasColumn 恒真到不了 AddColumn 分支。
func TestCoverageE_Mig0037_AddColumnError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	require.NoError(t, db.Migrator().CreateTable(&model.EmailVerification{}))
	require.NoError(t, db.Exec("CREATE TABLE admins (id integer primary key)").Error)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateEmailVerification(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0037")
}

// 0038：cicd_integrations 缺表 → 首个 CreateTable 失败。
func TestCoverageE_Mig0038_CreateIntegrationsError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateCicdTables(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0038")
}

// 0038：integrations 已在（跳过）、cicd_builds 缺表 → 第二个 CreateTable 失败。
func TestCoverageE_Mig0038_CreateBuildsError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	require.NoError(t, db.Migrator().CreateTable(&model.CicdIntegration{}))
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateCicdTables(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0038")
}

// ---------- service_context.go：ResolveDays 闭包 settings 空窗 ----------

// TestLogRetentionResolveDaysSettingsLifecycle 闭包两翼：settings 单例
// 空窗期短路 (0,0)（回落静态保留期），settings 就绪后读取 L3
// RetentionDays；两阶段 Sweep 均不 panic、零删除。不并跑——重置
// settings 单例与并行用例互斥。
func TestLogRetentionResolveDaysSettingsLifecycle(t *testing.T) {
	svcCtx := NewServiceContext(newSvcConfig(t, false))
	require.NotNil(t, svcCtx.LogRetention, "ExecutionLog 默认开启应构建 LogRetention")

	t.Cleanup(settings.ResetForTest)

	// 阶段一：清理器先于 settings 就绪的单例空窗 → 闭包短路 (0,0)
	settings.ResetForTest()
	summary := svcCtx.LogRetention.Sweep(context.Background())
	assert.Zero(t, summary.ExecutionLogsDeleted)
	assert.Zero(t, summary.TaskRunsDeleted)

	// 阶段二：settings 就绪（空 L2/L3 配置）→ 闭包读取 RetentionDays
	store := model.NewPlatformSettingModel(svcCtx.DB)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	summary = svcCtx.LogRetention.Sweep(context.Background())
	assert.Zero(t, summary.ExecutionLogsDeleted)
	assert.Zero(t, summary.TaskRunsDeleted)
}

// ---------- service_context.go：引导管理员 phone 档案回填 ----------

// newBackfillFixture 写三份引导配置文件 + 建 ServiceContext（含真实
// AdminManager/AdminModel），返回可直接调 seedBootstrapAdmins 的上下文。
func newBackfillFixture(t *testing.T, usersJSON string) *ServiceContext {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	write("admins.json", usersJSON)
	write("permissions.json", `[]`)
	write("roles.json", `[]`)

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	ctx := &ServiceContext{
		DB:           db,
		AdminManager: NewAdminManager(dir),
		AdminModel:   model.NewAdminModel(db),
		RoleModel:    model.NewRoleModel(db),
	}
	require.NoError(t, ctx.AdminManager.Initialize())
	return ctx
}

// TestSeedBootstrapAdminsBackfillsPhoneProfile 存量行（status 已同步、
// nickname/email/phone 全空）经引导配置回填三字段；二次执行幂等不重建。
func TestSeedBootstrapAdminsBackfillsPhoneProfile(t *testing.T) {
	svcCtx := newBackfillFixture(t,
		`[{"username":"admin","password":"admin123","nickname":"尼克","email":"a@b.c","phone":"13800000000"}]`)

	existing := &model.Admin{Username: "admin", Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(context.Background(), existing, "admin123"))

	require.NoError(t, seedBootstrapAdmins(svcCtx))

	found, err := svcCtx.AdminModel.FindByUsername(context.Background(), "admin")
	require.NoError(t, err)
	assert.Equal(t, "13800000000", found.Phone, "phone 空字段应被回填")
	assert.Equal(t, "尼克", found.Nickname)
	assert.Equal(t, "a@b.c", found.Email)

	// 二次执行：字段已非空不重复回填、不重建行
	require.NoError(t, seedBootstrapAdmins(svcCtx))
	again, err := svcCtx.AdminModel.FindByUsername(context.Background(), "admin")
	require.NoError(t, err)
	assert.Equal(t, found.ID, again.ID)
}

// TestSeedBootstrapAdminsBackfillUpdateBlocked 回填 UPDATE 被触发器拦下：
// 错误只 warn 不上抛（seedBootstrapAdmins 仍返回 nil），档案保持原样。
func TestSeedBootstrapAdminsBackfillUpdateBlocked(t *testing.T) {
	svcCtx := newBackfillFixture(t,
		`[{"username":"admin","password":"admin123","nickname":"尼克","phone":"13800000000"}]`)

	existing := &model.Admin{Username: "admin", Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(context.Background(), existing, "admin123"))

	require.NoError(t, svcCtx.DB.Exec(
		"CREATE TRIGGER blk_backfill BEFORE UPDATE ON admins BEGIN SELECT RAISE(ABORT, 'blocked'); END;").Error)

	require.NoError(t, seedBootstrapAdmins(svcCtx), "回填失败应降级为 warn，不阻断引导")

	found, err := svcCtx.AdminModel.FindByUsername(context.Background(), "admin")
	require.NoError(t, err)
	assert.Empty(t, found.Phone, "UPDATE 被拦时档案不得变化")
	assert.Empty(t, found.Nickname)
}

// ---------- service_context.go：Handle → mfaGate 403 ----------

// TestAuthMiddlewareHandle_MFAGate403 完整 Handle 链：合法 JWT（版本对齐）
// 过认证后，MFA 策略开启 + 未绑定账号在非白名单端点被 mfaGate 403 拦截。
func TestAuthMiddlewareHandle_MFAGate403(t *testing.T) {
	secret := "test-secret-r11-svc"
	defer jwtutil.ResetGlobalSecretForTesting(secret)()
	m, unbound := newMFAGateFixture(t, true)

	token, err := jwtutil.Sign(secret, "unbound", nil, unbound.ID, 0, time.Now())
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(m.Handle)
	r.GET("/api/v1/games", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/games", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "mfa_required")
}

// ---------- service_context.go：cachedOTPEnabled 三翼 ----------

func TestCachedOTPEnabled_HitExpiryAndStoreFailure(t *testing.T) {
	m, unbound := newMFAGateFixture(t, false)
	ctx := context.Background()

	// 缓存命中（新鲜）：直接按缓存版本回答，不回源
	m.otpEnabledCache.Store(unbound.ID, tokenVersionEntry{version: 0, fetchedAt: time.Now()})
	assert.False(t, m.cachedOTPEnabled(ctx, unbound.ID), "缓存 version=0 → 未启用")
	m.otpEnabledCache.Store(unbound.ID, tokenVersionEntry{version: 1, fetchedAt: time.Now()})
	assert.True(t, m.cachedOTPEnabled(ctx, unbound.ID), "缓存 version=1 → 已启用")

	// 缓存过期：回落 FindOne 回源，并刷新缓存（随后改库值也不再影响结果）
	m.otpEnabledCache.Store(unbound.ID, tokenVersionEntry{version: 1, fetchedAt: time.Now().Add(-2 * tokenVersionCacheTTL)})
	assert.False(t, m.cachedOTPEnabled(ctx, unbound.ID), "过期缓存回源：库中未绑定 → false")
	require.NoError(t, m.svcCtx.AdminModel.Update(ctx, unbound.ID, map[string]interface{}{"otp_enabled": true}))
	assert.False(t, m.cachedOTPEnabled(ctx, unbound.ID), "回源后缓存已刷新：库值翻转不影响缓存命中")

	// 存储故障：admins 表缺失 → fail-open 放行
	brokenDB, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(brokenDB))
	require.NoError(t, brokenDB.Migrator().DropTable(&model.Admin{}))
	m2 := NewAuthMiddlewareImpl(&ServiceContext{AdminModel: model.NewAdminModel(brokenDB)})
	assert.True(t, m2.cachedOTPEnabled(ctx, 7), "FindOne 出错应 fail-open")
}
