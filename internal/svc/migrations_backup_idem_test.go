package svc

// 0068 backups.idempotency_key 列 + 唯一索引（AUD-053）：加列 + 建索引、
// 幂等、缺表跳过；并复刻「0067 时代老表（无该列/索引）」的存量库形态
// （迁移契约：回归用例须复刻存量形态，非 AutoMigrate 新表）。
import (
	"context"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

func TestBackupIdempotencyKeyMigration_AddsColumnAndIndexIdempotent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	ctx := context.Background()

	// 缺表跳过：无 backups 表时不报错、不建空壳。
	require.NoError(t, migrateBackupIdempotencyKey(ctx, sqlDB))
	assert.False(t, db.Migrator().HasTable(&model.Backup{}))

	// 建表后复刻老库形态：Drop 掉模型新列与索引（AutoMigrate 建出的新表
	// 已含两者），再跑迁移补齐——这才是存量库真实路径。
	require.NoError(t, db.AutoMigrate(&model.Backup{}))
	require.NoError(t, db.Migrator().DropIndex(&model.Backup{}, "IdempotencyKey"))
	require.NoError(t, db.Migrator().DropColumn(&model.Backup{}, "IdempotencyKey"))
	assert.False(t, db.Migrator().HasColumn(&model.Backup{}, "idempotency_key"))

	require.NoError(t, migrateBackupIdempotencyKey(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.Backup{}, "idempotency_key"))
	assert.True(t, db.Migrator().HasIndex(&model.Backup{}, "IdempotencyKey"))

	// 幂等：再跑不报错。
	require.NoError(t, migrateBackupIdempotencyKey(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.Backup{}, "idempotency_key"))

	// NULL 多行不冲突（无键备份是常态）：两行 NULL 键 + 一行带键可共存，
	// 同键第二行被唯一索引拒绝。
	key := "aud053-migration-key"
	require.NoError(t, db.Create(&model.Backup{BackupID: "b-1", Name: "n1", Type: "full", Status: "pending"}).Error)
	require.NoError(t, db.Create(&model.Backup{BackupID: "b-2", Name: "n2", Type: "full", Status: "pending"}).Error)
	require.NoError(t, db.Create(&model.Backup{BackupID: "b-3", Name: "n3", Type: "full", Status: "pending", IdempotencyKey: &key}).Error)
	assert.Error(t, db.Create(&model.Backup{BackupID: "b-4", Name: "n4", Type: "full", Status: "pending", IdempotencyKey: &key}).Error,
		"同键第二行必须被唯一索引拒绝（并发双写兜底）")
}

// 迁移注册表必须含 0068（防 registerSvcMigrations 漏配）。
func TestRegisterSvcMigrations_ContainsBackupIdempotencyKey(t *testing.T) {
	goose.ResetGlobalMigrations()
	registerSvcMigrations()
	err := goose.SetGlobalMigrations(backupIdempotencyKeyMigration())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version 68 already registered")
}
