package model

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRetentionModel(t *testing.T) {
	db := setupTestDB(t)
	model := NewRetentionModel(db)
	assert.NotNil(t, model)
	assert.Same(t, db, model.db)
}

func TestUpsertCohort(t *testing.T) {
	db := setupTestDB(t)
	model := NewRetentionModel(db)
	ctx := context.Background()

	cohort := &RetentionCohort{
		GameID:      "test-game",
		Env:         "dev",
		Cohort:      "test-cohort",
		Users:       100,
		WindowStart: time.Now().Add(-24 * time.Hour),
		WindowEnd:   time.Now(),
	}

	// 测试创建
	err := model.UpsertCohort(ctx, cohort)
	assert.NoError(t, err)
	assert.NotZero(t, cohort.ID)

	// 测试更新
	cohort.Users = 95
	err = model.UpsertCohort(ctx, cohort)
	assert.NoError(t, err)
}

func TestListCohorts(t *testing.T) {
	db := setupTestDB(t)
	model := NewRetentionModel(db)
	ctx := context.Background()

	// 清理之前测试的数据
	db.Exec("DELETE FROM retention_cohorts")

	// 创建测试数据
	now := time.Now()

	cohorts := []*RetentionCohort{
		{
			GameID:      "game1",
			Env:         "dev",
			Cohort:      "day1",
			Users:       100,
			WindowStart: now.Add(-48 * time.Hour),
			WindowEnd:   now.Add(-24 * time.Hour),
		},
		{
			GameID:      "game1",
			Env:         "dev",
			Cohort:      "day7",
			Users:       80,
			WindowStart: now.Add(-168 * time.Hour),
			WindowEnd:   now.Add(-144 * time.Hour),
		},
		{
			GameID:      "game2",
			Env:         "prod",
			Cohort:      "day1",
			Users:       200,
			WindowStart: now.Add(-24 * time.Hour),
			WindowEnd:   now,
		},
	}

	for _, cohort := range cohorts {
		err := model.UpsertCohort(ctx, cohort)
		assert.NoError(t, err)
	}

	// 测试查询所有
	all, err := model.ListCohorts(ctx, "", "", "")
	assert.NoError(t, err)
	assert.Len(t, all, 3)

	// 测试按 game_id 过滤
	byGame, err := model.ListCohorts(ctx, "game1", "", "")
	assert.NoError(t, err)
	assert.Len(t, byGame, 2)

	// 测试按 env 过滤
	byEnv, err := model.ListCohorts(ctx, "", "dev", "")
	assert.NoError(t, err)
	assert.Len(t, byEnv, 2)

	// 测试按 cohort 过滤
	byCohort, err := model.ListCohorts(ctx, "", "", "day1")
	assert.NoError(t, err)
	assert.Len(t, byCohort, 2)

	// 测试组合过滤
	combined, err := model.ListCohorts(ctx, "game1", "dev", "day1")
	assert.NoError(t, err)
	assert.Len(t, combined, 1)

	// 测试空结果
	empty, err := model.ListCohorts(ctx, "nonexistent", "", "")
	assert.NoError(t, err)
	assert.Len(t, empty, 0)
}
