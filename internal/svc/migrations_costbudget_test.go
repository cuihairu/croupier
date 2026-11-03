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

func TestMigrateCostBudgetTables(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, migrateCostBudgetTables(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.CostBudget{}))
	require.NoError(t, migrateCostBudgetTables(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.CostBudget{}))
}
