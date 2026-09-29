package model

// 覆盖率巡检增量补测（email_verification.go）。同域主干用例由并行会话的
// cicd_email_models_test.go 覆盖（签发作废/四态查找/消费幂等/频控时间源/
// 缺表错误翼），本文件只补其**未覆盖**的两处，避免重复投入：
//  1. TableName 契约（无既有用例断言表名）；
//  2. CreateInvalidatingActive 事务内**第二段写失败**（tx.Create）的错误翼
//     ——缺表注入只能打中第一段 UPDATE，Create 那一行此前零覆盖。此处用
//     sqlite BEFORE INSERT trigger 拦写，并锁定关键不变量：作废与新建在
//     同一事务内，第二段失败必须整体回滚——旧令牌不得被"吃掉"成已用态，
//     否则用户手上的有效链接会因一次写失败而失效。
//  3. Consume 事务**首段**（used_at UPDATE）的 DB 错误回传。

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

var emailVerifGapSeq int

func setupEmailVerifGapDB(t *testing.T) (*gorm.DB, *EmailVerificationModel) {
	t.Helper()
	emailVerifGapSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:emailverifgap%d?mode=memory&cache=shared", emailVerifGapSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&EmailVerification{}))
	return db, NewEmailVerificationModel(db)
}

func TestEmailVerificationTableName(t *testing.T) {
	assert.Equal(t, "email_verifications", EmailVerification{}.TableName())
}

func TestCreateInvalidatingActive_CreateErrorRollsBackInvalidation(t *testing.T) {
	db, m := setupEmailVerifGapDB(t)
	ctx := context.Background()

	require.NoError(t, m.CreateInvalidatingActive(ctx, &EmailVerification{
		AdminID: 9, TokenHash: "h-live", Purpose: "register", ExpiresAt: time.Now().Add(time.Hour),
	}))

	// 拦下后续 INSERT：作废 UPDATE 先执行并成功，Create 随后失败。
	require.NoError(t, db.Exec(
		`CREATE TRIGGER block_email_verif_insert BEFORE INSERT ON email_verifications
		 BEGIN SELECT RAISE(ABORT, 'injected create failure'); END;`).Error)

	assert.Error(t, m.CreateInvalidatingActive(ctx, &EmailVerification{
		AdminID: 9, TokenHash: "h-next", Purpose: "register", ExpiresAt: time.Now().Add(time.Hour),
	}))

	require.NoError(t, db.Exec(`DROP TRIGGER block_email_verif_insert`).Error)
	var live EmailVerification
	require.NoError(t, db.Where("token_hash = ?", "h-live").First(&live).Error)
	assert.Nil(t, live.UsedAt, "回滚后旧令牌须仍未使用（未被误作废）")
	var n int64
	require.NoError(t, db.Model(&EmailVerification{}).Count(&n).Error)
	assert.EqualValues(t, 1, n, "新令牌不得残留")
}

// Consume 事务**首段**（used_at UPDATE）的 DB 错误回传：缺表即命中，
// 与「第二段写失败回滚」（上例）是两条不同错误路径。
func TestConsume_TokenTableMissingSurfacesDBError(t *testing.T) {
	db, m := setupEmailVerifGapDB(t)
	require.NoError(t, db.Migrator().DropTable(&EmailVerification{}))

	consumed, err := m.Consume(context.Background(), &EmailVerification{ID: 1, AdminID: 1})
	assert.False(t, consumed)
	assert.Error(t, err)
}
