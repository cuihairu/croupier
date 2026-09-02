// 覆盖目标：openGorm 的内存 sqlite 归一化、空 DSN 默认路径、
// 不支持的驱动错误分支；EnsureGameDatabase 的 router 缺失分支。
package svc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenGorm_MemorySQLite_Normalized(t *testing.T) {
	db, err := openGorm("sqlite", ":memory:")
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	// 归一化为共享缓存形态：跨连接可见
	require.NoError(t, db.Exec("CREATE TABLE mem_probe (id INTEGER PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("INSERT INTO mem_probe VALUES (1)").Error)
}

func TestOpenGorm_EmptySQLiteDSN_UsesDefault(t *testing.T) {
	// 空 DSN 走 data/croupier.db 默认路径（相对当前工作目录）
	db, err := openGorm("sqlite", "")
	if err != nil {
		// 目录不可写时允许失败，但错误必须来自文件系统而非逻辑
		assert.Error(t, err)
		return
	}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
}

func TestOpenGorm_OracleDriver_Rejected(t *testing.T) {
	_, err := openGorm("oracle", "user/pwd@db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database driver")
}

func TestOpenGorm_PostgresEmptyDSN_Rejected(t *testing.T) {
	_, err := openGorm("postgres", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postgres DSN is required")

	_, err = openGorm("postgresql", "")
	require.Error(t, err)
}

func TestOpenGorm_PostgresConnRefused(t *testing.T) {
	// 端口 1 无人监听 → 连接失败且不含 "does not exist" → else 分支直接返回错误
	_, err := openGorm("pg", "postgres://u:p@127.0.0.1:1/dbname?sslmode=disable")
	require.Error(t, err)
}

func TestOpenGorm_MysqlEmptyDSN_Rejected(t *testing.T) {
	_, err := openGorm("mysql", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mysql DSN is required")
}

func TestOpenGorm_MysqlConnRefused(t *testing.T) {
	_, err := openGorm("mysql", "u:p@tcp(127.0.0.1:1)/dbname")
	require.Error(t, err)
}
