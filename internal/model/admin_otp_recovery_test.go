package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var otpRecoveryDBSeq int

func setupOTPRecoveryDB(t *testing.T) (*gorm.DB, *AdminOTPRecoveryCodeModel) {
	t.Helper()
	otpRecoveryDBSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:admin_otp_recovery%d?mode=memory&cache=shared", otpRecoveryDBSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AdminOTPRecoveryCode{}))
	return db, NewAdminOTPRecoveryCodeModel(db)
}

func TestAdminOTPRecoveryCodeTableName(t *testing.T) {
	assert.Equal(t, "admin_otp_recovery_codes", AdminOTPRecoveryCode{}.TableName())
}

// TestOTPRecoveryReplaceReplacesAll 重发新码集时旧集必须整体消失——
// 用户以为作废的码不能再继续生效。
func TestOTPRecoveryReplaceReplacesAll(t *testing.T) {
	_, m := setupOTPRecoveryDB(t)
	ctx := context.Background()

	require.NoError(t, m.Replace(ctx, 7, []string{"h1", "h2", "h3"}))
	n, err := m.CountUsable(ctx, 7)
	require.NoError(t, err)
	assert.EqualValues(t, 3, n)

	require.NoError(t, m.Replace(ctx, 7, []string{"h4"}))
	codes, err := m.ListUsable(ctx, 7)
	require.NoError(t, err)
	require.Len(t, codes, 1)
	assert.Equal(t, "h4", codes[0].CodeHash)
}

// TestOTPRecoveryReplaceEmptyClears 空码集 = 清空（如禁用 MFA 前的过渡态）。
func TestOTPRecoveryReplaceEmptyClears(t *testing.T) {
	_, m := setupOTPRecoveryDB(t)
	ctx := context.Background()

	require.NoError(t, m.Replace(ctx, 8, []string{"h1", "h2"}))
	require.NoError(t, m.Replace(ctx, 8, nil))
	n, err := m.CountUsable(ctx, 8)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n)
}

// TestOTPRecoveryConsumeSingleUse 每码只能消费一次：重放与未知哈希均为 false。
func TestOTPRecoveryConsumeSingleUse(t *testing.T) {
	_, m := setupOTPRecoveryDB(t)
	ctx := context.Background()

	require.NoError(t, m.Replace(ctx, 9, []string{"hA", "hB"}))

	ok, err := m.Consume(ctx, 9, "hA")
	require.NoError(t, err)
	assert.True(t, ok, "首消费应成功")

	ok, err = m.Consume(ctx, 9, "hA")
	require.NoError(t, err)
	assert.False(t, ok, "重放同码必须失败（防并发复用）")

	ok, err = m.Consume(ctx, 9, "hZ")
	require.NoError(t, err)
	assert.False(t, ok, "未知哈希必须失败")

	codes, err := m.ListUsable(ctx, 9)
	require.NoError(t, err)
	require.Len(t, codes, 1)
	assert.Equal(t, "hB", codes[0].CodeHash)
}

// TestOTPRecoveryIsUsable UsedAt 为 nil 即可用，消费后不可用。
func TestOTPRecoveryIsUsable(t *testing.T) {
	code := &AdminOTPRecoveryCode{}
	assert.True(t, code.IsUsable())
	now := time.Now()
	code.UsedAt = &now
	assert.False(t, code.IsUsable())
}

// TestOTPRecoveryDeleteAll 禁用 MFA 时全部清除且为硬删除（Unscoped）。
func TestOTPRecoveryDeleteAll(t *testing.T) {
	db, m := setupOTPRecoveryDB(t)
	ctx := context.Background()

	require.NoError(t, m.Replace(ctx, 10, []string{"h1", "h2"}))
	ok, err := m.Consume(ctx, 10, "h1")
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, m.DeleteAll(ctx, 10))

	n, err := m.CountUsable(ctx, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n)

	var total int64
	require.NoError(t, db.Unscoped().Model(&AdminOTPRecoveryCode{}).Count(&total).Error)
	assert.EqualValues(t, 0, total, "DeleteAll 必须连已用行一并硬删")
}

// TestOTPRecoveryGuards 零值/nil 模型守卫：全部返回错误而非 panic。
func TestOTPRecoveryGuards(t *testing.T) {
	ctx := context.Background()

	var nilModel *AdminOTPRecoveryCodeModel
	_, err := nilModel.CountUsable(ctx, 1)
	assert.Error(t, err)
	_, err = nilModel.ListUsable(ctx, 1)
	assert.Error(t, err)
	_, err = nilModel.Consume(ctx, 1, "h")
	assert.Error(t, err)
	assert.Error(t, nilModel.Replace(ctx, 1, []string{"h"}))
	assert.Error(t, nilModel.DeleteAll(ctx, 1))

	emptyModel := NewAdminOTPRecoveryCodeModel(nil)
	_, err = emptyModel.CountUsable(ctx, 1)
	assert.Error(t, err)
	_, err = emptyModel.ListUsable(ctx, 1)
	assert.Error(t, err)
	_, err = emptyModel.Consume(ctx, 1, "h")
	assert.Error(t, err)
	assert.Error(t, emptyModel.Replace(ctx, 1, []string{"h"}))
	assert.Error(t, emptyModel.DeleteAll(ctx, 1))
}

// TestOTPRecoveryReplaceErrorBranches 覆盖 Replace 事务内的错误分支：
// Delete 失败（line 52）与 Create 失败（事务回滚）。
func TestOTPRecoveryReplaceErrorBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("delete error in transaction", func(t *testing.T) {
		// Use a DB with a trigger to block DELETE
		db, err := gorm.Open(sqlite.Open("file:otp_err_delete?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&AdminOTPRecoveryCode{}))
		// Pre-insert a row so DELETE has something to act on
		require.NoError(t, db.Create(&AdminOTPRecoveryCode{AdminID: 1, CodeHash: "h1"}).Error)
		// Trigger blocks DELETE
		require.NoError(t, db.Exec(`CREATE TRIGGER otp_block_del BEFORE DELETE ON admin_otp_recovery_codes BEGIN SELECT RAISE(ABORT, 'blocked'); END`).Error)

		m := NewAdminOTPRecoveryCodeModel(db)
		err = m.Replace(ctx, 1, []string{"h2"})
		assert.Error(t, err)
	})

	t.Run("create error in transaction", func(t *testing.T) {
		// Use a DB with a trigger to block INSERT
		db, err := gorm.Open(sqlite.Open("file:otp_err_create?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&AdminOTPRecoveryCode{}))
		// Trigger blocks INSERT after Delete succeeds
		require.NoError(t, db.Exec(`CREATE TRIGGER otp_block_ins BEFORE INSERT ON admin_otp_recovery_codes BEGIN SELECT RAISE(ABORT, 'blocked'); END`).Error)

		m := NewAdminOTPRecoveryCodeModel(db)
		err = m.Replace(ctx, 2, []string{"h1"})
		assert.Error(t, err)
		// Verify no rows were inserted (transaction rolled back)
		var count int64
		require.NoError(t, db.Model(&AdminOTPRecoveryCode{}).Count(&count).Error)
		assert.Equal(t, int64(0), count)
	})
}

// TestOTPRecoveryConsumeErrorBranch 覆盖 Consume 的 Update 错误分支（line 93）。
func TestOTPRecoveryConsumeErrorBranch(t *testing.T) {
	ctx := context.Background()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	require.NoError(t, sqlDB.Close())

	m := NewAdminOTPRecoveryCodeModel(db)
	consumed, err := m.Consume(ctx, 1, "h1")
	assert.Error(t, err)
	assert.False(t, consumed)
}
