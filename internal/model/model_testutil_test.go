package model

import (
	"fmt"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sync/atomic"
	"testing"
	"time"
)

// Split from model_test.go; test bodies moved verbatim.

// modelTestDBSeq 给每次调用发互不重名的内存库。匿名 file::memory:?cache=shared
// 全进程共用一个实例：-count=3 下上一遍连接关尽即销毁，但同包其他用例的池仍开着
// 时会命中上一遍残留数据（admins.username 唯一约束冲突）。
var modelTestDBSeq atomic.Int64

func modelTestDBName() string {
	return fmt.Sprintf("file:model_%d?mode=memory&cache=shared", modelTestDBSeq.Add(1))
}

// setupTestDB creates an in-memory SQLite database for testing

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Migrate all models
	err = db.AutoMigrate(
		&Admin{},
		&AdminRole{},
		&AdminGameScope{},
		&AdminGameEnvScope{},
		&Game{},
		&Role{},
		&RolePermission{},
		&Permission{},
		&Player{},
		&Function{},
		&FunctionDescriptor{},
		&FunctionInstance{},
		&FunctionPermission{},
		&PendingFunction{},
		&Alert{},
		&AlertSilence{},
		&AgentSessionDB{},
		&Message{},
		&Node{},
		&NodeCommand{},
		&Ticket{},
		&TicketComment{},
		&Feedback{},
		&FAQ{},
		&FAQCategory{},
		&Backup{},
		&Certificate{},
		&CertificateAlert{},
		&ConfigVersion{},
		&ProfilePermission{},
		&ProfileGame{},
		&RateLimit{},
		&TermDictionary{},
		&RetentionCohort{},
		&SupportTicket{},
		&SupportComment{},
		&SupportFAQ{},
		&SupportFeedback{},
		&FunctionPolicy{},
		&Descriptor{},
		&GameEnvBinding{},
		&DeployPackage{},
		&Deployment{},
	)
	require.NoError(t, err)

	return db
}

// ===== AdminModel Tests =====

func setupAlertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&Alert{}, &AlertSilence{})
	require.NoError(t, err)
	return db
}

func setupMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:msg_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	err = db.AutoMigrate(&Message{})
	require.NoError(t, err)
	return db
}

func setupNodeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&Node{}, &NodeCommand{})
	require.NoError(t, err)
	return db
}

func setupTicketTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&Ticket{}, &TicketComment{})
	require.NoError(t, err)
	return db
}

func setupFeedbackTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&Feedback{})
	require.NoError(t, err)
	return db
}

func setupFAQTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&FAQ{}, &FAQCategory{})
	require.NoError(t, err)
	return db
}

func setupProfileTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&ProfilePermission{}, &ProfileGame{})
	require.NoError(t, err)
	return db
}

func setupRateLimitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&RateLimit{})
	require.NoError(t, err)
	return db
}

func setupBackupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&Backup{})
	require.NoError(t, err)
	return db
}

func setupCertificateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&Certificate{}, &CertificateAlert{})
	require.NoError(t, err)
	return db
}

func setupConfigVersionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&ConfigVersion{})
	require.NoError(t, err)
	return db
}

func setupTermDictionaryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&TermDictionary{})
	require.NoError(t, err)
	return db
}

func setupAgentSessionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(&AgentSessionDB{})
	require.NoError(t, err)
	return db
}
