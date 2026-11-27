package svc

// 0069 business_operation_credentials 表（GAME-009 批准协议 D2 / GAME-010
// 平台半边）：新建表 CreateTable、幂等、缺表跳过；唯一作用域索引
// idx_biz_op_cred_scope 是「同 operationId 重复执行」判定的物理前提，
// 索引 tag 漂移会让 Register 恒判「新行」、幂等静默失效——同键第二行必须
// 撞唯一索引的断言一并保留（page_execute_idempotency 同款防线）。
import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

func TestBusinessOperationCredentialsMigration_CreatesTableIdempotent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	ctx := context.Background()

	// 新建表迁移：空库直接 CreateTable（0019/0028 模式；「缺表跳过」语义
	// 属加列迁移，建表迁移的空库行为就是建表）。
	require.NoError(t, migrateBusinessOperationCredentials(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable("business_operation_credentials"))

	// AutoMigrate 走开发路径（模型即真值）——0069 与其产出一致：表 + 唯一
	// 作用域索引齐备；再跑迁移幂等。
	require.NoError(t, db.AutoMigrate(&model.BusinessOperationCredential{}))
	require.NoError(t, migrateBusinessOperationCredentials(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.BusinessOperationCredential{}))
	if !db.Migrator().HasIndex(&model.BusinessOperationCredential{}, "idx_biz_op_cred_scope") {
		t.Fatal("unique index idx_biz_op_cred_scope missing: exactly-once relies on the physical unique key")
	}

	// 唯一作用域物理判据：同 (game_id, env, operation_id) 第二行必须撞索引。
	row := model.BusinessOperationCredential{
		GameID: "demo", Env: "prod", OperationID: "demo:prod:player:018f3b27-5c2e-7b10-8000-000000000001",
		Domain: "player", FunctionID: "player.ban", RequestFingerprint: "fp",
		Outcome: model.BizOpOutcomePending, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Create(&row).Error)
	row.ID = 0
	assert.Error(t, db.Create(&row).Error, "同 operationId 第二行必须被唯一索引拒绝")
	// 不同 scope 同字面 ID 互不影响。
	row.ID = 0
	row.GameID = "beta"
	assert.NoError(t, db.Create(&row).Error, "不同游戏同字面 operationId 不冲突")
}

// 迁移注册表必须含 0069（防 registerSvcMigrations 漏配：模型改了迁移漏配
// 即「本地全绿、上线即断」同族事故）。
func TestRegisterSvcMigrations_ContainsBusinessOperationCredentials(t *testing.T) {
	goose.ResetGlobalMigrations()
	registerSvcMigrations()
	err := goose.SetGlobalMigrations(businessOperationCredentialsTableMigration())
	require.Error(t, err, "0069 must already be registered; duplicate registration must fail")
}
