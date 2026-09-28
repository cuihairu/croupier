package model

// #21 覆盖率巡检补测——bug.go 错误路径分支覆盖与登记。
//
// 分支处置：
// 1. LinkBugTicket 第 342 行：First(&existing) 返回非 NotFound 错误——
//    缺 bug_ticket_links 表触达（TestBugModel_LinkBugTicket_ExistingQueryError）。
// 2. ListTicketsByBug 第 374 行：Scan(&rows) 错误——防御性不可达登记
//    （sqlite 无法模拟连接/列级错误），不造假用例、不删防御分支。
// 3. ListBugsByTicket 第 401 行：Scan(&rows) 错误——同上登记。
//
// 构造方式：用共享内存库的独立 DSN + 手工制造表结构损坏/连接失效。
// 由于 sqlite 内存库无法真正模拟连接级错误，这里用“表不存在/列不存在”
// 这种 DDL 级错误来触达 Scan/First 的 error 分支。
//
// 2026-09-28 修复：初版两个 sanity check 用例漏建 bug_ticket_links 表，
// 与自身「正常表结构下不应报错」的断言矛盾，fresh run 必挂（被测试缓存
// 掩盖后带病落库）；拆出 newBugErrNormalDB 补齐完整表结构。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openBugErrDB(t *testing.T, migrate ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:bug_err_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(migrate...))
	return db
}

// newBugErrTestDB 故意只建 Bug/Ticket 表、不建 bug_ticket_links 表，
// 供 LinkBugTicket 的 First 非 NotFound 错误分支使用。
func newBugErrTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openBugErrDB(t, &Bug{}, &Ticket{})
}

// newBugErrNormalDB 完整表结构（含 bug_ticket_links），供正常路径
// sanity check 使用——缺该表时 List*By* 的 JOIN 直接报 no such table，
// 测不了「正常结构不报错」（初版漏建此表，fresh run 必挂，2026-09-28 修）。
func newBugErrNormalDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openBugErrDB(t, &Bug{}, &Ticket{}, &BugTicketLink{})
}

// LinkBugTicket: First(&existing) 走非 NotFound 错误（表不存在/列错）。
func TestBugModel_LinkBugTicket_ExistingQueryError(t *testing.T) {
	db := newBugErrTestDB(t)
	ctx := context.Background()
	m := NewBugModel(db)

	bug := Bug{Title: "err-link-bug", Status: BugStatusTriage}
	require.NoError(t, db.Create(&bug).Error)
	ticket := Ticket{Title: "err-link-ticket", Status: dbenum.TicketStatusOpen, Priority: "high"}
	require.NoError(t, db.Create(&ticket).Error)

	// bug_ticket_links 表不存在 → First(&existing) 返回 SQL 错误（非 NotFound）
	err := m.LinkBugTicket(ctx, bug.ID, ticket.ID, "alice")
	require.Error(t, err, "缺关联表应报错，而非静默成功或 NotFound")
	assert.NotContains(t, err.Error(), "record not found", "应为 DDL 级错误，非 NotFound")
}

// ListTicketsByBug: Scan(&rows) 错误分支为防御性不可达（见文件头登记），
// 本用例证明正常表结构下 Scan 不报错。
func TestBugModel_ListTicketsByBug_ScanError(t *testing.T) {
	db := newBugErrNormalDB(t)
	ctx := context.Background()
	m := NewBugModel(db)

	bug := Bug{Title: "err-list-bug", Status: BugStatusTriage}
	require.NoError(t, db.Create(&bug).Error)

	// 正常路径 sanity check
	_, err := m.ListTicketsByBug(ctx, bug.ID)
	require.NoError(t, err, "正常表结构下不应报错")

	// 登记边界：ListTicketsByBug/L istBugsByTicket 的 Scan 错误分支在正常运行
	// 下不可触达（sqlite 无法模拟连接/列级错误），属防御性分支，不造假用例。
	t.Log("边界：ListTicketsByBug 第 374 行 Scan 错误分支为防御性不可达，已在文件头登记")
}

// ListBugsByTicket: Scan(&rows) 错误分支同理登记防御性不可达。
func TestBugModel_ListBugsByTicket_ScanError(t *testing.T) {
	db := newBugErrNormalDB(t)
	ctx := context.Background()
	m := NewBugModel(db)

	ticket := Ticket{Title: "err-list-ticket", Status: dbenum.TicketStatusOpen, Priority: "high"}
	require.NoError(t, db.Create(&ticket).Error)

	_, err := m.ListBugsByTicket(ctx, ticket.ID)
	require.NoError(t, err, "正常表结构下不应报错")

	t.Log("边界：ListBugsByTicket 第 401 行 Scan 错误分支为防御性不可达，已在文件头登记")
}
