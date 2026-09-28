package model

// #21 服务端下拉聚合的模型层补测（覆盖率巡检：ticket_model.go 79.2% 为
// internal/ 全树最低文件，缺口集中在 ListCategories/ListAssignees/
// listOptionStats——落地时只有前端消费测试）。
//
// 口径：distinct 非空值 + COUNT 降？不——listOptionStats 按 name ASC 排序、
// 空串排除、(game_id, env) 可选过滤。测试用唯一命名前缀 + 独立 scope，
// 与同包共享内存库（cache=shared）里其他用例的数据互不干扰。

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cuihairu/croupier/internal/dbenum"
)

func createOptionTicket(t *testing.T, m *TicketModel, category, assignee, gameID, env string) {
	t.Helper()
	require.NoError(t, m.Create(context.Background(), &Ticket{
		Title:    "opt-stat-" + category + assignee + gameID + env,
		Content:  "coverage",
		Category: category,
		Priority: "low",
		Status:   dbenum.TicketStatusOpen,
		Assignee: assignee,
		GameID:   gameID,
		Env:      env,
	}))
}

// distinct 非空聚合 + 计数 + name ASC 排序；空串既不计入也不成行。
func TestTicketModel_ListOptionStats_AggregatesDistinctSorted(t *testing.T) {
	db := setupTicketTestDB(t)
	m := NewTicketModel(db)
	ctx := context.Background()

	createOptionTicket(t, m, "cova-alpha", "cova-op-a", "covgame", "covenv")
	createOptionTicket(t, m, "cova-alpha", "cova-op-a", "covgame", "covenv")
	createOptionTicket(t, m, "cova-beta", "cova-op-b", "covgame", "covenv")
	createOptionTicket(t, m, "", "cova-op-a", "covgame", "covenv") // 空 category 不得成行

	cats, err := m.ListCategories(ctx, "covgame", "covenv")
	require.NoError(t, err)
	require.Len(t, cats, 2, "空 category 排除后仅两行，got %v", cats)
	assert.Equal(t, TicketOptionStat{Name: "cova-alpha", Count: 2}, cats[0], "name ASC 排序")
	assert.Equal(t, TicketOptionStat{Name: "cova-beta", Count: 1}, cats[1])

	assignees, err := m.ListAssignees(ctx, "covgame", "covenv")
	require.NoError(t, err)
	require.Len(t, assignees, 2)
	assert.Equal(t, TicketOptionStat{Name: "cova-op-a", Count: 3}, assignees[0])
	assert.Equal(t, TicketOptionStat{Name: "cova-op-b", Count: 1}, assignees[1])
}

// (game_id, env) 过滤分支：双带/单带/不带三种口径各自成立。
// 用独立 scope（covgame2 族）——同包测试共享内存库，避免跨用例数据串台。
func TestTicketModel_ListOptionStats_ScopeFiltering(t *testing.T) {
	db := setupTicketTestDB(t)
	m := NewTicketModel(db)
	ctx := context.Background()

	createOptionTicket(t, m, "cova-scope-a", "", "covgame2", "covenv2")
	createOptionTicket(t, m, "cova-scope-b", "", "covgame2", "prod2")
	createOptionTicket(t, m, "cova-scope-c", "", "othergame2", "prod2")

	both, err := m.ListCategories(ctx, "covgame2", "covenv2")
	require.NoError(t, err)
	require.Len(t, both, 1)
	assert.Equal(t, "cova-scope-a", both[0].Name)

	gameOnly, err := m.ListCategories(ctx, "covgame2", "")
	require.NoError(t, err)
	require.Len(t, gameOnly, 2, "仅按 game 过滤应含两个 env 的行")

	unscoped, err := m.ListCategories(ctx, "", "")
	require.NoError(t, err)
	names := map[string]int{}
	for _, r := range unscoped {
		names[r.Name] = r.Count
	}
	assert.Equal(t, 1, names["cova-scope-c"], "无 scope 时全量聚合")
}

// 表不存在（迁移未跑）：返回错误而非 panic。
func TestTicketModel_ListOptionStats_TableMissingErrors(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/ticket-bare.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	m := NewTicketModel(db)

	_, err = m.ListCategories(context.Background(), "covgame", "covenv")
	require.Error(t, err)
	_, err = m.ListAssignees(context.Background(), "covgame", "covenv")
	require.Error(t, err)
}
