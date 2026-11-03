package model

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCostBudgetModelCRUD(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CostBudget{}))
	m := NewCostBudgetModel(db)
	require.NoError(t, m.Upsert(context.Background(), &CostBudget{GameID: "g", Env: "e", MonthlyUSD: 100, NotifyChannelsJSON: `["ding"]`, UpdatedBy: "admin"}))
	b, err := m.Get(context.Background(), "g", "e")
	require.NoError(t, err)
	assert.Equal(t, 100.0, b.MonthlyUSD)
	assert.Equal(t, `["ding"]`, b.NotifyChannelsJSON)
	var channels []string
	require.NoError(t, json.Unmarshal([]byte(b.NotifyChannelsJSON), &channels))
	assert.Equal(t, []string{"ding"}, channels)
	require.NoError(t, m.Upsert(context.Background(), &CostBudget{GameID: "g", Env: "e", MonthlyUSD: 200, UpdatedBy: "admin2"}))
	b2, err := m.Get(context.Background(), "g", "e")
	require.NoError(t, err)
	assert.Equal(t, 200.0, b2.MonthlyUSD)
	assert.Equal(t, "", b2.NotifyChannelsJSON)
	_, err = m.Get(context.Background(), "g2", "e")
	assert.Error(t, err)
	_ = time.Now()
}
