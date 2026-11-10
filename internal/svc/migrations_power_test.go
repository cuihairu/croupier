package svc

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestMigratePowerRequestTables 0064 建 power_requests + power_request_targets，
// 重复执行幂等。
func TestMigratePowerRequestTables(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// :memory: 单连接，防连接池多句柄各见各的库（test 基建坑）。
	sqlDB.SetMaxOpenConns(1)
	ctx := context.Background()
	require.NoError(t, migratePowerRequestTables(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.PowerRequest{}))
	assert.True(t, db.Migrator().HasTable(&model.PowerRequestTarget{}))
	// 幂等：二次执行不报错、不重建
	require.NoError(t, migratePowerRequestTables(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.PowerRequest{}))
	assert.True(t, db.Migrator().HasTable(&model.PowerRequestTarget{}))
}
