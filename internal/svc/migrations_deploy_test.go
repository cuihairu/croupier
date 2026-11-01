package svc

import (
	"context"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

// TestMigrateDeployTables verifies 0055 creates deploy_packages + deployments
// once and is idempotent on re-run (HasTable guard).
func TestMigrateDeployTables(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, migrateDeployTables(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.DeployPackage{}))
	assert.True(t, db.Migrator().HasTable(&model.Deployment{}))
	// Idempotent second run.
	require.NoError(t, migrateDeployTables(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.DeployPackage{}))
	assert.True(t, db.Migrator().HasTable(&model.Deployment{}))
}
