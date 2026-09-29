package model

// 覆盖率巡检补测：本包 177 条未覆盖语句中 171 条集中在三个近零文件——
// cicd_build.go（1.3%）、cicd_integration.go（2%）、email_verification.go（2%）。
// api/cicd 域（handler/service 层）归并行会话在途，本文件只测 model 层。
//
// 注入口径同 svc C 批：读翼缺表即错；写翼 PRAGMA query_only 拒写
// （锁 MaxOpenConns=1 保证复用同一被拒连接）。写翼的「查得到行但写被拒」
// 分支必须先铺行再开 query_only（空表查询直接 ErrRecordNotFound 会走错翼）。

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// refuseWrites 连接级只读开关（PRAGMA 作用于单连接，锁死池大小 1）。
func refuseWrites(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)
}

// ---------- cicd_build.go ----------

func TestNormalizeCicdBuildStatus_ClosureAndAliases(t *testing.T) {
	cases := map[string]string{
		// 闭集原样
		"queued": "queued", "running": "running", "success": "success",
		"failed": "failed", "cancelled": "cancelled",
		// 别名收敛
		"pending": "queued", "created": "queued", "waiting": "queued",
		"waiting_for_resource": "queued", "preparing": "queued", "queued_or_running": "queued",
		"in_progress": "running", "building": "running",
		"succeeded": "success", "ok": "success", "passed": "success",
		"done": "success", "complete": "success", "completed_ok": "success",
		"failure": "failed", "error": "failed", "errored": "failed", "unstable": "failed",
		"canceled": "cancelled", "aborted": "cancelled", "skipped": "cancelled",
		// 归一：大小写 + 空白；未识别 → unknown
		"  SUCCESS ": "success", "Pending": "queued", "garbage": "unknown", "": "unknown",
	}
	for in, want := range cases {
		assert.Equal(t, want, NormalizeCicdBuildStatus(in), "input %q", in)
	}
}

func TestValidateCicdWebhook_Branches(t *testing.T) {
	_, err := ValidateCicdWebhook("job", "  ", "success")
	require.ErrorContains(t, err, "externalId 不能为空")

	_, err = ValidateCicdWebhook("job", strings.Repeat("x", 256), "success")
	require.ErrorContains(t, err, "externalId 过长")

	status, err := ValidateCicdWebhook("job", "build-42", "Succeeded")
	require.NoError(t, err)
	assert.Equal(t, CicdBuildSuccess, status)
}

func seedBuild(t *testing.T, db *gorm.DB, b *CicdBuild) uint {
	t.Helper()
	require.NoError(t, db.Create(b).Error)
	return b.ID
}

func TestCicdBuildModel_UpsertByExternalKey(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewCicdBuildModel(db)
	ctx := context.Background()

	// 新键 → 建行
	started := time.Now().Add(-time.Hour)
	id1, err := m.UpsertByExternalKey(ctx, &CicdBuild{
		GameID: "demo", Env: "prod", IntegrationID: 7, ExternalID: "b-1", Kind: "jenkins",
		Pipeline: "game-pack", Status: CicdBuildQueued, TriggeredBy: "api",
	})
	require.NoError(t, err)
	require.NotZero(t, id1)

	// 同键 → 更新状态/URL/产物/Raw 与时间戳，行 ID 不变
	finished := time.Now()
	raw := datatypes.JSONMap{"result": "SUCCESS"}
	id2, err := m.UpsertByExternalKey(ctx, &CicdBuild{
		IntegrationID: 7, ExternalID: "b-1", Status: CicdBuildSuccess,
		WebURL: "https://ci/job/1", ArtifactURL: "https://art/1.tgz",
		ArtifactChecksum: "sha256:abc", StartedAt: &started, FinishedAt: &finished, Raw: raw,
	})
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "同 (integration,external) 幂等键应复用行")

	var row CicdBuild
	require.NoError(t, db.First(&row, id1).Error)
	assert.Equal(t, CicdBuildSuccess, row.Status)
	assert.Equal(t, "https://ci/job/1", row.WebURL)
	assert.Equal(t, "sha256:abc", row.ArtifactChecksum)
	assert.NotNil(t, row.StartedAt)
	assert.NotNil(t, row.FinishedAt)
	assert.Equal(t, "SUCCESS", row.Raw["result"])

	// 首查失败翼：缺表（非 ErrRecordNotFound 的查询错误直传）
	missed := setupAllModelsDB(t)
	require.NoError(t, missed.Migrator().DropTable(&CicdBuild{}))
	_, err = NewCicdBuildModel(missed).UpsertByExternalKey(ctx, &CicdBuild{IntegrationID: 1, ExternalID: "x"})
	require.Error(t, err, "缺表时首查应报错")

	// 建行失败翼：表在、连接拒写 → First 走 ErrRecordNotFound、Create 被拒
	roDB := setupAllModelsDB(t)
	refuseWrites(t, roDB)
	_, err = NewCicdBuildModel(roDB).UpsertByExternalKey(ctx, &CicdBuild{IntegrationID: 1, ExternalID: "x"})
	require.Error(t, err, "空表 + 拒写连接：Create 应被拒")

	// 更新失败翼：行在、连接拒写 → First 命中、Updates 被拒（先铺行再开只读）
	updDB := setupAllModelsDB(t)
	seedBuild(t, updDB, &CicdBuild{IntegrationID: 9, ExternalID: "e", Status: CicdBuildQueued})
	refuseWrites(t, updDB)
	_, err = NewCicdBuildModel(updDB).UpsertByExternalKey(ctx, &CicdBuild{IntegrationID: 9, ExternalID: "e", Status: CicdBuildFailed})
	require.Error(t, err, "已有行 + 拒写连接：Updates 应被拒")
}

func TestCicdBuildModel_ListFilterMatrixAndPaging(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewCicdBuildModel(db)
	ctx := context.Background()

	rows := []CicdBuild{
		{GameID: "demo", Env: "prod", IntegrationID: 1, ExternalID: "a", Kind: "jenkins", Pipeline: "pack-main", Status: CicdBuildSuccess, Version: "v1.0.0"},
		{GameID: "demo", Env: "dev", IntegrationID: 2, ExternalID: "b", Kind: "gitlab-ci", Pipeline: "pack-hotfix", Status: CicdBuildFailed, Version: "v1.1.0"},
		{GameID: "rpg", Env: "prod", IntegrationID: 1, ExternalID: "c", Kind: "jenkins", Pipeline: "pack-main", Status: CicdBuildRunning, Version: "v2.0.0"},
		{GameID: "demo", Env: "prod", IntegrationID: 3, ExternalID: "d", Kind: "generic", Pipeline: "custom", Status: CicdBuildSuccess, Version: "v1.0.0"},
	}
	for i := range rows {
		require.NoError(t, db.Create(&rows[i]).Error)
	}

	// 逐过滤维度 + 空白 trim
	for _, tc := range []struct {
		name string
		opts CicdBuildQueryOptions
		want []string // external_id 期望集，按 id DESC
	}{
		{"game", CicdBuildQueryOptions{GameID: " demo "}, []string{"d", "b", "a"}},
		{"env", CicdBuildQueryOptions{Env: "prod"}, []string{"d", "c", "a"}},
		{"integration", CicdBuildQueryOptions{IntegrationID: 2}, []string{"b"}},
		{"kind", CicdBuildQueryOptions{Kind: "jenkins"}, []string{"c", "a"}},
		{"status", CicdBuildQueryOptions{Status: "success"}, []string{"d", "a"}},
		{"pipeline", CicdBuildQueryOptions{Pipeline: "pack-main"}, []string{"c", "a"}},
		{"version", CicdBuildQueryOptions{Version: "v1.0.0"}, []string{"d", "a"}},
		{"combined", CicdBuildQueryOptions{GameID: "demo", Env: "prod", Status: "success"}, []string{"d", "a"}},
		{"no_filter", CicdBuildQueryOptions{}, []string{"d", "c", "b", "a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, total, err := m.List(ctx, tc.opts)
			require.NoError(t, err)
			require.Equal(t, int64(len(tc.want)), total)
			ids := make([]string, 0, len(got))
			for _, r := range got {
				ids = append(ids, r.ExternalID)
			}
			assert.Equal(t, tc.want, ids, "新构建在前（id DESC）")
		})
	}

	// 分页：offset 跳过 + limit 截断
	got, _, err := m.List(ctx, CicdBuildQueryOptions{Limit: 2, Offset: 1})
	require.NoError(t, err)
	assert.Equal(t, []string{"c", "b"}, []string{got[0].ExternalID, got[1].ExternalID})

	// 负 offset 归零
	got, _, err = m.List(ctx, CicdBuildQueryOptions{Limit: 1, Offset: -5})
	require.NoError(t, err)
	assert.Equal(t, []string{"d"}, []string{got[0].ExternalID})

	// limit 异常值收敛（≤0 或 >200 → 50）：铺到 51 行验证只回 50
	for i := 0; i < 51; i++ {
		require.NoError(t, db.Create(&CicdBuild{
			GameID: "bulk", IntegrationID: 100, ExternalID: fmt.Sprintf("bulk-%03d", i),
		}).Error)
	}
	bulk, total, err := m.List(ctx, CicdBuildQueryOptions{GameID: "bulk", Limit: 999})
	require.NoError(t, err)
	assert.Equal(t, int64(51), total)
	assert.Len(t, bulk, 50, "limit>200 收敛为 50")

	// 错误翼：缺表（Count 即失败）
	missed := setupAllModelsDB(t)
	require.NoError(t, missed.Migrator().DropTable(&CicdBuild{}))
	_, _, err = NewCicdBuildModel(missed).List(ctx, CicdBuildQueryOptions{})
	require.Error(t, err)

	// 错误翼：Count 过、Find 挂——用无 id 列的视图顶替表，count(*) 正常
	// 而 ORDER BY id 报 no such column（deleted_at 以 NULL 恒假列满足软删过滤）
	viewDB := setupAllModelsDB(t)
	require.NoError(t, viewDB.Migrator().DropTable(&CicdBuild{}))
	require.NoError(t, viewDB.Exec(
		"CREATE VIEW cicd_builds AS SELECT NULL AS deleted_at, type AS status FROM sqlite_master").Error)
	_, _, err = NewCicdBuildModel(viewDB).List(ctx, CicdBuildQueryOptions{})
	require.Error(t, err, "count 通过后 Find 失败应直传错误")
}

func TestCicdBuildModel_GetByIDAndUpdateStatus(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewCicdBuildModel(db)
	ctx := context.Background()

	id := seedBuild(t, db, &CicdBuild{IntegrationID: 1, ExternalID: "s-1", Status: CicdBuildQueued})

	got, err := m.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, CicdBuildQueued, got.Status)

	_, err = m.GetByID(ctx, 999999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// 全字段回写（webURL + 双时间戳）
	started, finished := time.Now().Add(-time.Minute), time.Now()
	updated, err := m.UpdateStatus(ctx, id, CicdBuildSuccess, "https://ci/s-1", &started, &finished)
	require.NoError(t, err)
	assert.Equal(t, CicdBuildSuccess, updated.Status)
	assert.Equal(t, "https://ci/s-1", updated.WebURL)
	require.NotNil(t, updated.StartedAt)
	require.NotNil(t, updated.FinishedAt)

	// 条件列缺省：空 webURL / nil 时间戳不覆盖
	updated, err = m.UpdateStatus(ctx, id, CicdBuildFailed, "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, CicdBuildFailed, updated.Status)
	assert.Equal(t, "https://ci/s-1", updated.WebURL, "空 URL 不清列")
	assert.NotNil(t, updated.StartedAt)

	// 错误翼：拒写连接（行在，Updates 被拒）
	refuseWrites(t, db)
	_, err = m.UpdateStatus(ctx, id, CicdBuildCancelled, "", nil, nil)
	require.Error(t, err)
}

func TestCicdBuildModel_DeleteByIntegration(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewCicdBuildModel(db)
	ctx := context.Background()

	keep := seedBuild(t, db, &CicdBuild{IntegrationID: 1, ExternalID: "keep"})
	seedBuild(t, db, &CicdBuild{IntegrationID: 2, ExternalID: "gone"})

	require.NoError(t, m.DeleteByIntegration(ctx, 2))

	var n int64
	db.Model(&CicdBuild{}).Count(&n)
	assert.Equal(t, int64(1), n, "只删目标接入的构建")
	_, err := m.GetByID(ctx, keep)
	require.NoError(t, err)
}

// ---------- cicd_integration.go ----------

func TestValidateCicdIntegration_Branches(t *testing.T) {
	valid := func() *CicdIntegration {
		return &CicdIntegration{Kind: "jenkins", Name: "ci", Endpoint: "https://ci.example.com"}
	}

	// 正常 + 三字段 trim
	in := &CicdIntegration{Kind: " jenkins ", Name: " ci ", Endpoint: " https://ci.example.com "}
	require.NoError(t, ValidateCicdIntegration(in))
	assert.Equal(t, "jenkins", in.Kind)
	assert.Equal(t, "ci", in.Name)
	assert.Equal(t, "https://ci.example.com", in.Endpoint)

	in = valid()
	in.Name = ""
	require.ErrorContains(t, ValidateCicdIntegration(in), "名称不能为空")

	in = valid()
	in.Name = strings.Repeat("n", 129)
	require.ErrorContains(t, ValidateCicdIntegration(in), "名称过长")

	in = valid()
	in.Kind = "teamcity"
	require.ErrorContains(t, ValidateCicdIntegration(in), "未知的 CI/CD 类型")

	in = valid()
	in.Endpoint = "ftp://ci.example.com"
	require.ErrorContains(t, ValidateCicdIntegration(in), "http(s) URL")

	in = valid()
	in.Endpoint = "https://" + strings.Repeat("e", 510)
	require.ErrorContains(t, ValidateCicdIntegration(in), "服务端点过长")

	in = valid()
	in.Token = strings.Repeat("t", 513)
	require.ErrorContains(t, ValidateCicdIntegration(in), "凭据过长")
}

func TestCicdIntegrationModel_CRUDAndScope(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewCicdIntegrationModel(db)
	ctx := context.Background()

	// 建三条：name 倒序写入验证 List 的 name ASC 稳定序
	for _, in := range []*CicdIntegration{
		{GameID: "demo", Env: "prod", Kind: "jenkins", Name: "zeta", Endpoint: "https://z", Enabled: true, CreatedBy: "ops"},
		{GameID: "demo", Env: "dev", Kind: "gitlab-ci", Name: "mid", Endpoint: "https://m"},
		{GameID: "rpg", Env: "prod", Kind: "jenkins", Name: "alpha", Endpoint: "https://a"},
	} {
		require.NoError(t, m.Create(ctx, in))
	}

	got, err := m.GetByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "zeta", got.Name)

	// scope 过滤 + trim
	list, err := m.List(ctx, " demo ", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"mid", "zeta"}, []string{list[0].Name, list[1].Name}, "name 升序")

	list, err = m.List(ctx, "demo", "prod")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "zeta", list[0].Name)

	list, err = m.List(ctx, "", "")
	require.NoError(t, err)
	assert.Len(t, list, 3, "空 scope 不限")

	// Update 覆写（Save 全行）
	got.Name = "zeta-2"
	got.Enabled = false
	require.NoError(t, m.Update(ctx, got))
	after, err := m.GetByID(ctx, got.ID)
	require.NoError(t, err)
	assert.Equal(t, "zeta-2", after.Name)
	assert.False(t, after.Enabled)

	// CountByKind 分组
	counts, err := m.CountByKind(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{"jenkins": 2, "gitlab-ci": 1}, counts)

	// Delete
	require.NoError(t, m.Delete(ctx, got.ID))
	_, err = m.GetByID(ctx, got.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// 错误翼：List / CountByKind 缺表直传
	missed := setupAllModelsDB(t)
	require.NoError(t, missed.Migrator().DropTable(&CicdIntegration{}))
	mm := NewCicdIntegrationModel(missed)
	_, err = mm.List(ctx, "", "")
	require.Error(t, err)
	_, err = mm.CountByKind(ctx)
	require.Error(t, err)
}

func TestMaskToken(t *testing.T) {
	assert.Equal(t, "", MaskToken(""))
	assert.Equal(t, "", MaskToken("   "))
	assert.Equal(t, "****", MaskToken("abc"))
	assert.Equal(t, "****", MaskToken("abcd"))
	assert.Equal(t, "****1234", MaskToken("secret-token-1234"))
	assert.Equal(t, "****1234", MaskToken("  secret-token-1234  "), "首尾空白先归一")
}

// ---------- email_verification.go ----------

func TestEmailVerificationModel_NilGuards(t *testing.T) {
	ctx := context.Background()
	var nilModel *EmailVerificationModel
	require.ErrorContains(t, nilModel.CreateInvalidatingActive(ctx, &EmailVerification{}), "未初始化")
	_, err := nilModel.FindValidByTokenHash(ctx, "h")
	require.ErrorContains(t, err, "未初始化")
	_, err = nilModel.Consume(ctx, &EmailVerification{})
	require.ErrorContains(t, err, "未初始化")
	_, err = nilModel.LatestCreatedAtFor(ctx, 1, "register")
	require.ErrorContains(t, err, "未初始化")

	nilDB := &EmailVerificationModel{}
	_, err = nilDB.LatestCreatedAtFor(ctx, 1, "register")
	require.ErrorContains(t, err, "未初始化")
}

func issueVerification(t *testing.T, db *gorm.DB, adminID uint, purpose, hash string) {
	t.Helper()
	require.NoError(t, NewEmailVerificationModel(db).CreateInvalidatingActive(context.Background(), &EmailVerification{
		AdminID: adminID, TokenHash: hash, Purpose: purpose, ExpiresAt: time.Now().Add(time.Hour),
	}))
}

func TestEmailVerificationModel_CreateInvalidatesActive(t *testing.T) {
	db := setupAllModelsDB(t)
	ctx := context.Background()

	issueVerification(t, db, 1, "register", "hash-1")
	issueVerification(t, db, 1, "register", "hash-2")
	issueVerification(t, db, 2, "register", "hash-3")
	issueVerification(t, db, 1, "reset", "hash-4")

	// 同账号同用途旧令牌全部作废；新令牌与他账号/他用途令牌不动
	var unusedOld, unusedNew, unusedOther, unusedReset int64
	db.Model(&EmailVerification{}).Where("token_hash = ? AND used_at IS NULL", "hash-1").Count(&unusedOld)
	db.Model(&EmailVerification{}).Where("token_hash = ? AND used_at IS NULL", "hash-2").Count(&unusedNew)
	db.Model(&EmailVerification{}).Where("token_hash = ? AND used_at IS NULL", "hash-3").Count(&unusedOther)
	db.Model(&EmailVerification{}).Where("token_hash = ? AND used_at IS NULL", "hash-4").Count(&unusedReset)
	assert.Zero(t, unusedOld, "重发即作废：同账号同用途旧令牌标记 used")
	assert.Equal(t, int64(1), unusedNew, "新签发令牌保持可用")
	assert.Equal(t, int64(1), unusedOther, "他账号令牌不受影响")
	assert.Equal(t, int64(1), unusedReset, "他用途令牌不受影响")

	// 错误翼：缺表
	missed := setupAllModelsDB(t)
	require.NoError(t, missed.Migrator().DropTable(&EmailVerification{}))
	err := NewEmailVerificationModel(missed).CreateInvalidatingActive(ctx, &EmailVerification{AdminID: 1, TokenHash: "x", ExpiresAt: time.Now().Add(time.Hour)})
	require.Error(t, err)
}

func TestEmailVerificationModel_FindValidByTokenHash(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewEmailVerificationModel(db)
	ctx := context.Background()

	now := time.Now()
	require.NoError(t, db.Create(&EmailVerification{AdminID: 1, TokenHash: "valid", Purpose: "register", ExpiresAt: now.Add(time.Hour)}).Error)
	require.NoError(t, db.Create(&EmailVerification{AdminID: 1, TokenHash: "used", Purpose: "register", ExpiresAt: now.Add(time.Hour), UsedAt: &now}).Error)
	require.NoError(t, db.Create(&EmailVerification{AdminID: 1, TokenHash: "expired", Purpose: "register", ExpiresAt: now.Add(-time.Minute)}).Error)

	hit, err := m.FindValidByTokenHash(ctx, "valid")
	require.NoError(t, err)
	require.NotNil(t, hit)
	assert.Equal(t, "valid", hit.TokenHash)

	// 已用/过期/不存在统一 nil,nil（不区分原因，防令牌探测）
	for _, hash := range []string{"used", "expired", "bogus"} {
		got, err := m.FindValidByTokenHash(ctx, hash)
		require.NoError(t, err, hash)
		assert.Nil(t, got, hash)
	}

	// 错误翼：缺表
	missed := setupAllModelsDB(t)
	require.NoError(t, missed.Migrator().DropTable(&EmailVerification{}))
	_, err = NewEmailVerificationModel(missed).FindValidByTokenHash(ctx, "valid")
	require.Error(t, err)
}

func TestEmailVerificationModel_Consume(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewEmailVerificationModel(db)
	ctx := context.Background()

	admin := &Admin{Username: "verify-me", Nickname: "v", Status: 1, PasswordHash: "x", EmailVerified: false}
	require.NoError(t, db.Create(admin).Error)

	row := &EmailVerification{AdminID: admin.ID, TokenHash: "consume-me", Purpose: "register", ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, db.Create(row).Error)

	// 首次消费：令牌置 used + 账号 email_verified 置真
	ok, err := m.Consume(ctx, row)
	require.NoError(t, err)
	assert.True(t, ok)
	var after Admin
	require.NoError(t, db.First(&after, admin.ID).Error)
	assert.True(t, after.EmailVerified)

	// 重复消费（已 used）：RowsAffected=0 → false 且不动账号
	ok, err = m.Consume(ctx, row)
	require.NoError(t, err)
	assert.False(t, ok, "已用令牌的并发复用被单条 UPDATE 拒绝")

	// 账号更新失败翼：令牌在、admins 表缺 → 事务回滚报错
	issueVerification(t, db, admin.ID, "register", "no-admin-table")
	var pending EmailVerification
	require.NoError(t, db.Where("token_hash = ?", "no-admin-table").First(&pending).Error)
	require.NoError(t, db.Migrator().DropTable(&Admin{}))
	_, err = m.Consume(ctx, &pending)
	require.Error(t, err, "admins 表缺失时事务应报错回滚")
}

func TestEmailVerificationModel_LatestCreatedAtFor(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewEmailVerificationModel(db)
	ctx := context.Background()

	// 从未发过：零值无错
	ts, err := m.LatestCreatedAtFor(ctx, 42, "register")
	require.NoError(t, err)
	assert.True(t, ts.IsZero())

	issueVerification(t, db, 7, "register", "lat-1")
	issueVerification(t, db, 7, "register", "lat-2")
	ts, err = m.LatestCreatedAtFor(ctx, 7, "register")
	require.NoError(t, err)
	assert.False(t, ts.IsZero(), "重发频控查询应拿到最近一次发令牌时间")

	// 他用途不算
	ts, err = m.LatestCreatedAtFor(ctx, 7, "reset")
	require.NoError(t, err)
	assert.True(t, ts.IsZero())

	// 错误翼：缺表
	missed := setupAllModelsDB(t)
	require.NoError(t, missed.Migrator().DropTable(&EmailVerification{}))
	_, err = NewEmailVerificationModel(missed).LatestCreatedAtFor(ctx, 7, "register")
	require.Error(t, err)
}
