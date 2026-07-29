package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBuildLegacyWorkspaceReportWithoutTable(t *testing.T) {
	db := openLegacyWorkspaceReportTestDB(t)

	report, err := buildLegacyWorkspaceReport(context.Background(), db, 10)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.TableExists)
	assert.Zero(t, report.TotalCount)
	assert.Empty(t, report.Items)
}

func TestBuildLegacyWorkspaceReportMarksUnknownScopeWithoutColumns(t *testing.T) {
	db := openLegacyWorkspaceReportTestDB(t)
	require.NoError(t, db.Exec(`
		CREATE TABLE workspace_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			object_key TEXT NOT NULL,
			title TEXT,
			published BOOLEAN DEFAULT FALSE,
			menu_order INTEGER DEFAULT 0,
			config TEXT,
			updated_at DATETIME
		)
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO workspace_configs (object_key, published, updated_at) VALUES
			('player.manage', TRUE, ?),
			('mail.send', FALSE, ?)
	`, time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC), time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)).Error)
	require.NoError(t, db.AutoMigrate(&model.PageSpec{}, &model.PublishedPageSpec{}, &model.PageVersion{}))

	report, err := buildLegacyWorkspaceReport(context.Background(), db, 10)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.True(t, report.TableExists)
	assert.Equal(t, int64(2), report.TotalCount)
	assert.Equal(t, int64(0), report.ScopedCount)
	assert.Equal(t, int64(2), report.UnknownScopeCount)
	assert.Equal(t, int64(1), report.PublishedTrueCount)
	require.Len(t, report.Items, 2)
	assert.Equal(t, "player.manage", report.Items[0].ObjectKey)
	assert.Equal(t, "unknown", report.Items[0].ScopeStatus)

	var pageSpecCount int64
	require.NoError(t, db.Model(&model.PageSpec{}).Count(&pageSpecCount).Error)
	assert.Zero(t, pageSpecCount)
}

func TestBuildLegacyWorkspaceReportReadsExplicitScopeColumns(t *testing.T) {
	db := openLegacyWorkspaceReportTestDB(t)
	require.NoError(t, db.Exec(`
		CREATE TABLE workspace_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			game_id TEXT,
			env TEXT,
			object_key TEXT NOT NULL,
			published BOOLEAN DEFAULT FALSE,
			updated_at DATETIME
		)
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO workspace_configs (game_id, env, object_key, published, updated_at) VALUES
			('demo', 'prod', 'player.manage', TRUE, ?),
			('', '', 'mail.send', FALSE, ?)
	`, time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC), time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)).Error)

	report, err := buildLegacyWorkspaceReport(context.Background(), db, 10)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, int64(1), report.ScopedCount)
	assert.Equal(t, int64(1), report.UnknownScopeCount)
	require.Len(t, report.Items, 2)
	assert.Equal(t, "scoped", report.Items[0].ScopeStatus)
	assert.Equal(t, "demo", report.Items[0].GameID)
	assert.Equal(t, "prod", report.Items[0].Env)
	assert.Equal(t, "unknown", report.Items[1].ScopeStatus)
}

func TestRenderLegacyWorkspaceReport(t *testing.T) {
	report := &legacyWorkspaceReport{
		TableExists:        true,
		Columns:            []string{"env", "game_id", "object_key", "published", "updated_at"},
		TotalCount:         1,
		ScopedCount:        1,
		UnknownScopeCount:  0,
		PublishedTrueCount: 1,
		Items: []legacyWorkspaceItem{
			{
				ScopeStatus: "scoped",
				GameID:      "demo",
				Env:         "prod",
				ObjectKey:   "player.manage",
				Published:   true,
				UpdatedAt:   timePointer(time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)),
			},
		},
	}

	var out bytes.Buffer
	renderLegacyWorkspaceReport(&out, "etc/server.yaml", "sqlite", "data/croupier.db", report)

	rendered := out.String()
	assert.True(t, strings.Contains(rendered, "table_exists: true"))
	assert.True(t, strings.Contains(rendered, "scope=demo/prod"))
	assert.True(t, strings.Contains(rendered, "object_key=player.manage"))
}

func openLegacyWorkspaceReportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

func timePointer(ts time.Time) *time.Time {
	return &ts
}
