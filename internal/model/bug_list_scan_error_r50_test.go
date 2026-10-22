package model

// 覆盖率巡检 R50 翻案：bug_error_paths_test.go 头注释第 2/3 条曾把
// ListTicketsByBug :374 / ListBugsByTicket :401 的 Scan 错误分支登记为
// 「sqlite 无法模拟连接/列级错误」的防御性不可达——但同文件自己的
// LinkBugTicket 用例（缺 bug_ticket_links 表触达 First 非 NotFound 错误）
// 已证明 DDL 级故障同样走 error 分支：两方法的 JOIN 均引用
// bug_ticket_links，缺表时 Scan 直接报 no such table。本文件以同一
// newBugErrTestDB 形态收口这两翼（脏数据/迁移半途的库正会呈现这种形态）。
import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBugModel_ListTicketsByBug_ScanError(t *testing.T) {
	db := newBugErrTestDB(t) // Bug+Ticket 建表，bug_ticket_links 缺失
	ctx := context.Background()
	m := NewBugModel(db)

	bug := Bug{Title: "scan-err-bug", Status: BugStatusTriage}
	require.NoError(t, db.Create(&bug).Error)

	items, err := m.ListTicketsByBug(ctx, bug.ID)
	require.Error(t, err, "JOIN 缺表应走 Scan 错误分支（bug_error_paths_test 登记翻案）")
	assert.Contains(t, err.Error(), "bug_ticket_links")
	assert.Nil(t, items)
}

func TestBugModel_ListBugsByTicket_ScanError(t *testing.T) {
	db := newBugErrTestDB(t) // Bug+Ticket 建表，bug_ticket_links 缺失
	ctx := context.Background()
	m := NewBugModel(db)

	ticket := Ticket{Title: "scan-err-ticket", Status: dbenum.TicketStatusOpen, Priority: "high"}
	require.NoError(t, db.Create(&ticket).Error)

	bugs, err := m.ListBugsByTicket(ctx, ticket.ID)
	require.Error(t, err, "JOIN 缺表应走 Scan 错误分支（bug_error_paths_test 登记翻案）")
	assert.Contains(t, err.Error(), "bug_ticket_links")
	assert.Nil(t, bugs)
}
