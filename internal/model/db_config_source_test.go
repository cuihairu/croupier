package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var srcSeq int

func newSrcDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	srcSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:src%d?mode=memory&cache=shared", srcSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(models...))
	return db
}

func TestDBSourceModelCRUD(t *testing.T) {
	db := newSrcDB(t, &DBSource{})
	m := NewDBSourceModel(db)
	ctx := context.Background()

	src := &DBSource{Name: "main", Driver: "postgres", Kind: "self", DSN: "postgres://...", GameID: "demo", Env: "prod", Enabled: true}
	require.NoError(t, m.Create(ctx, src))
	assert.NotZero(t, src.ID)

	got, err := m.FindOne(ctx, src.ID)
	require.NoError(t, err)
	assert.Equal(t, "postgres", got.Driver)

	list, err := m.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, m.Update(ctx, src.ID, map[string]interface{}{"enabled": false}))

	require.NoError(t, m.Delete(ctx, src.ID))
	_, err = m.FindOne(ctx, src.ID)
	assert.Error(t, err)
}

func TestConfigSourceBindingModelCRUD(t *testing.T) {
	db := newSrcDB(t, &ConfigSourceBinding{})
	m := NewConfigSourceBindingModel(db)
	ctx := context.Background()

	b := &ConfigSourceBinding{GameID: "demo", Env: "prod", Name: "apollo", Type: "git", Config: `{"url":"http://..."}`}
	require.NoError(t, m.Create(ctx, b))
	assert.NotZero(t, b.ID)

	got, err := m.Get(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, "apollo", got.Name)

	list, err := m.ListByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	b.Name = "renamed"
	require.NoError(t, m.Update(ctx, b))

	require.NoError(t, m.Delete(ctx, b.ID))
	list, _ = m.ListByScope(ctx, "demo", "prod")
	assert.Len(t, list, 0)
}
