package model

// 零散模型错误臂/过滤臂补测：ValidateDBSource 校验链、HashExternalToken、
// ExternalToken/CostBudget/Backup/Message/TaskRun/TaskSchedule/PageSpec 的
// 过滤与错误分支。
//
// 诚实跳过：
//   - CapabilitySemantics.StableContentDigest 的 marshal 失败臂（458）：
//     interface{} 字段全部经 normalizeJSONContent 归一（nil/JSON 值/字符串），
//     json.Marshal 恒成功。

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dbenum"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var errSmallGapFailpoint = errors.New("failpoint: store down")

// regSmallGapFailpoint 按表内第 n 次指定操作注入错误；update/create 走
// Before（写入不落地），query 走 After（调用方以 Error 为准）。
func regSmallGapFailpoint(t *testing.T, db *gorm.DB, table, op string, n int) {
	t.Helper()
	var hits int64
	name := "test:gap_" + table + "_" + op
	reg := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != table {
			return
		}
		hits++
		if hits == int64(n) {
			_ = tx.AddError(errSmallGapFailpoint)
		}
	}
	switch op {
	case "query":
		require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, reg))
	case "update":
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, reg))
	case "create":
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, reg))
	case "query-any":
		// 聚合 Scan 走 Row 处理器（不经 gorm:query 链），在核心回调前注入。
		require.NoError(t, db.Callback().Row().Before("gorm:row").Register(name, func(tx *gorm.DB) {
			hits++
			if hits == int64(n) {
				_ = tx.AddError(errSmallGapFailpoint)
			}
		}))
	default:
		t.Fatalf("unknown failpoint op %q", op)
	}
}

func newSmallGapDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:gap_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(models...))
	return db
}

// ValidateDBSource：gameId/env 缺失与 ownerAgentId 带首尾空白（59-68）。
func TestValidateDBSource_RequiredArms(t *testing.T) {
	base := DBSource{Name: "src", Driver: "mysql", Kind: "self", DSN: "dsn"}

	invalid := base
	invalid.GameID = " "
	require.ErrorContains(t, ValidateDBSource(&invalid), "gameId")

	invalid = base
	invalid.GameID = "g1"
	invalid.Env = ""
	require.ErrorContains(t, ValidateDBSource(&invalid), "env")

	invalid = base
	invalid.GameID = "g1"
	invalid.Env = "e1"
	invalid.OwnerAgentID = " agent-1 "
	require.ErrorContains(t, ValidateDBSource(&invalid), "ownerAgentId")
}

// HashExternalToken：sha256 十六进制定长输出（16-18）。
func TestHashExternalToken(t *testing.T) {
	sum := HashExternalToken("plain-token")
	assert.Len(t, sum, 64)
	assert.Equal(t, sum, HashExternalToken("plain-token"))
	assert.NotEqual(t, sum, HashExternalToken("other-token"))
}

// ExternalToken List：Count 查询失败透传（61-62）。
func TestExternalTokenModel_ListCountError(t *testing.T) {
	db := newSmallGapDB(t, &ExternalToken{})
	regSmallGapFailpoint(t, db, "external_tokens", "query", 1)
	_, _, err := NewExternalTokenModel(db).List(context.Background(), ListTokensOptions{Page: 1, PageSize: 10})
	require.ErrorIs(t, err, errSmallGapFailpoint)
}

// CostBudget Upsert：gameId/env 缺失拒绝（30-32）。
func TestCostBudgetModel_UpsertValidation(t *testing.T) {
	db := newSmallGapDB(t, &CostBudget{})
	m := NewCostBudgetModel(db)
	ctx := context.Background()

	require.ErrorContains(t, m.Upsert(ctx, &CostBudget{Env: "e1"}), "gameId and env required")
	require.ErrorContains(t, m.Upsert(ctx, &CostBudget{GameID: "g1"}), "gameId and env required")

	// 合法路径：Assign 按 scope 更新。
	require.NoError(t, m.Upsert(ctx, &CostBudget{GameID: "g1", Env: "e1", MonthlyUSD: 12.5, UpdatedBy: "tester"}))
}

// Backup List：GameID/Env 过滤分支（96-101）。
func TestBackupModel_ListScopeFilters(t *testing.T) {
	db := newSmallGapDB(t, &Backup{})
	m := NewBackupModel(db)
	ctx := context.Background()
	require.NoError(t, m.Create(ctx, &Backup{BackupID: "bk1", GameID: "g1", Env: "e1"}))
	require.NoError(t, m.Create(ctx, &Backup{BackupID: "bk2", GameID: "g2", Env: "e1"}))
	require.NoError(t, m.Create(ctx, &Backup{BackupID: "bk3", GameID: "g1", Env: "e2"}))

	rows, total, err := m.List(ctx, ListBackupsOptions{GameID: "g1"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, rows, 2)

	rows, total, err = m.List(ctx, ListBackupsOptions{GameID: "g1", Env: "e1"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, "bk1", rows[0].BackupID)
}

// Message List：Recipients 与 IncludeSource 的三形态（63-67）。
func TestMessageModel_ListRecipientsArms(t *testing.T) {
	db := setupMessageTestDB(t)
	m := NewMessageModel(db)
	ctx := context.Background()
	require.NoError(t, m.Create(ctx, &Message{To: "u1", Type: "alert", Title: "t1", Content: "c", Status: dbenum.MessageStatusUnread}))
	require.NoError(t, m.Create(ctx, &Message{To: "leader", Type: "alert", Source: "alert", Title: "t2", Content: "c", Status: dbenum.MessageStatusUnread}))

	// Recipients + IncludeSource：并集（63-64）
	_, total, err := m.List(ctx, ListMessagesOptions{Recipients: []string{"u1"}, IncludeSource: "alert"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	// 仅 Recipients（65-67）
	_, total, err = m.List(ctx, ListMessagesOptions{Recipients: []string{"u1"}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

// PageSpec UpdatePublishedPointer 只推进发布指针列（205-214）。
func TestPageSpecModel_UpdatePublishedPointer(t *testing.T) {
	db := newSmallGapDB(t, &PageSpec{})
	m := NewPageSpecModel(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&PageSpec{
		GameID: "g1", Env: "e1", PageKey: "p1", Type: "operation",
		Status: "draft", DraftRevision: 3, TitleJSON: `{"zh-CN":"标题"}`,
	}).Error)

	at := time.Now().UTC().Add(time.Minute)
	require.NoError(t, m.UpdatePublishedPointer(ctx, "g1", "e1", "p1", 7, at))

	var row PageSpec
	require.NoError(t, db.Where("game_id = ? AND page_key = ?", "g1", "p1").First(&row).Error)
	assert.Equal(t, "published", row.Status)
	assert.True(t, row.PublishedActive)
	assert.Equal(t, 7, row.PublishedVersion)
	assert.Equal(t, 3, row.DraftRevision, "草稿修订号不得被发布指针更新触碰")
	assert.Equal(t, `{"zh-CN":"标题"}`, row.TitleJSON, "标题不得被发布指针更新触碰")
}

// TaskRun：事务事件的状态守卫更新失败（110-112）、两种 Reap 的更新失败
// （158-160/184-186）、List 的 status 过滤（252-254）、OperatorOptions 扫描
// 失败（268-270）。
func TestTaskRunModelErrorArms(t *testing.T) {
	db := newSmallGapDB(t, &TaskRun{}, &TaskEvent{})
	m := NewTaskRunModel(db)
	ctx := context.Background()
	run := &TaskRun{TaskID: "t-gap-1", Status: "running", FunctionID: "fn.a", AgentID: "agent-1", GameID: "g1", Env: "e1"}
	require.NoError(t, m.Create(ctx, run))

	// 110-112：事务内 Updates 注错。
	regSmallGapFailpoint(t, db, "task_runs", "update", 1)
	_, err := m.RecordTaskEventTransactional(ctx, "t-gap-1", []string{"completed"},
		map[string]interface{}{"status": "completed"}, &TaskEvent{TaskID: "t-gap-1", Type: "progress"})
	require.ErrorIs(t, err, errSmallGapFailpoint)

	// 158-160：ReapByAgent 更新失败。
	regSmallGapFailpoint(t, db, "task_runs", "update", 1)
	_, err = m.ReapByAgent(ctx, run.AgentID, []string{"running"}, "failed", "reaped")
	require.ErrorIs(t, err, errSmallGapFailpoint)

	// 184-186：ReapStale 更新失败。
	regSmallGapFailpoint(t, db, "task_runs", "update", 1)
	_, err = m.ReapStale(ctx, []string{"running"}, time.Now().Add(-time.Hour), "failed", "stale")
	require.ErrorIs(t, err, errSmallGapFailpoint)

	// 252-254：List 的 Status 过滤分支。
	rows, total, err := m.List(ctx, ListTasksOptions{Status: "running"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, "t-gap-1", rows[0].TaskID)

	// 268-270：OperatorOptions 的 actor 聚合扫描失败（裸 Scan 无表名，按序注入）。
	regSmallGapFailpoint(t, db, "task_runs", "query-any", 1)
	_, err = m.OperatorOptions(ctx, ListTasksOptions{})
	require.ErrorIs(t, err, errSmallGapFailpoint)
}

// TaskSchedule GetRunLog：无记录返回 (nil, nil)（181-183）、查询失败透传（184-186）。
func TestTaskScheduleModel_GetRunLogArms(t *testing.T) {
	db := newSmallGapDB(t, &TaskScheduleRunLog{})
	m := NewTaskScheduleModel(db)
	ctx := context.Background()
	slot := time.Now().UTC().Truncate(time.Minute)

	// 无记录 → (nil, nil)
	log, err := m.GetRunLog(ctx, 42, slot)
	require.NoError(t, err)
	assert.Nil(t, log)

	// 查询失败 → 错误透传
	regSmallGapFailpoint(t, db, "task_schedule_run_logs", "query", 1)
	_, err = m.GetRunLog(ctx, 42, slot)
	require.ErrorIs(t, err, errSmallGapFailpoint)
}
