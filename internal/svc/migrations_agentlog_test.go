package svc

import (
	"context"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestMigrateAgentLogTables verifies 0056 creates agent_log_entries once and
// is idempotent on re-run.
func TestMigrateAgentLogTables(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, migrateAgentLogTables(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&reg.AgentLogEntryRecord{}))
	// Idempotent second run.
	require.NoError(t, migrateAgentLogTables(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&reg.AgentLogEntryRecord{}))
}
