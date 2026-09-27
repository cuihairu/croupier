package model

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var bugLinkDBSeq uint64

// newBugLinkTestDB 每用例独立 DSN：共享 cache=shared 的内存库会被同包其他
// 用例污染（先例：TestFAQModel_ListCategories 的隔离 DSN 修正）。
func newBugLinkTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("buglink_%d", atomic.AddUint64(&bugLinkDBSeq, 1))
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Bug{}, &Ticket{}, &BugTicketLink{}))
	return db
}

func seedBugForLink(t *testing.T, db *gorm.DB, title string) Bug {
	t.Helper()
	bug := Bug{Title: title, Status: BugStatusTriage}
	require.NoError(t, db.Create(&bug).Error)
	return bug
}

func seedTicketForLink(t *testing.T, db *gorm.DB, title string) Ticket {
	t.Helper()
	ticket := Ticket{Title: title, Status: dbenum.TicketStatusOpen, Priority: "high"}
	require.NoError(t, db.Create(&ticket).Error)
	return ticket
}

func TestBugTicketLink_IdempotentRelink(t *testing.T) {
	db := newBugLinkTestDB(t)
	ctx := context.Background()
	bug := seedBugForLink(t, db, "登录崩溃")
	ticket := seedTicketForLink(t, db, "玩家反馈登录失败")
	m := NewBugModel(db)

	require.NoError(t, m.LinkBugTicket(ctx, bug.ID, ticket.ID, "alice"))
	// 重复关联同一对：幂等成功，不产生第二行
	require.NoError(t, m.LinkBugTicket(ctx, bug.ID, ticket.ID, "bob"))

	var links []BugTicketLink
	require.NoError(t, db.Find(&links).Error)
	require.Len(t, links, 1)
	// 首次创建者的审计字段不被重复关联覆盖
	assert.Equal(t, "alice", links[0].CreatedBy)
}

func TestBugTicketLink_RequiresBothSides(t *testing.T) {
	db := newBugLinkTestDB(t)
	ctx := context.Background()
	m := NewBugModel(db)
	bug := seedBugForLink(t, db, "孤立 bug")
	ticket := seedTicketForLink(t, db, "孤立工单")

	assert.ErrorContains(t, m.LinkBugTicket(ctx, bug.ID, 999, ""), "工单不存在")
	assert.ErrorContains(t, m.LinkBugTicket(ctx, 999, ticket.ID, ""), "bug 不存在")
	assert.ErrorContains(t, m.LinkBugTicket(ctx, 0, ticket.ID, ""), "均不能为空")
	assert.ErrorContains(t, m.LinkBugTicket(ctx, bug.ID, 0, ""), "均不能为空")

	var links []BugTicketLink
	require.NoError(t, db.Find(&links).Error)
	assert.Empty(t, links)
}

func TestBugTicketLink_UnlinkIdempotent(t *testing.T) {
	db := newBugLinkTestDB(t)
	ctx := context.Background()
	m := NewBugModel(db)
	bug := seedBugForLink(t, db, "充值失败")
	ticket := seedTicketForLink(t, db, "充值投诉")

	require.NoError(t, m.LinkBugTicket(ctx, bug.ID, ticket.ID, "alice"))
	require.NoError(t, m.UnlinkBugTicket(ctx, bug.ID, ticket.ID))
	// 解除不存在的关联同样是幂等 no-op（客户端重试安全）
	require.NoError(t, m.UnlinkBugTicket(ctx, bug.ID, ticket.ID))

	var links []BugTicketLink
	require.NoError(t, db.Find(&links).Error)
	assert.Empty(t, links)
}

func TestBugTicketLink_DualListBriefs(t *testing.T) {
	db := newBugLinkTestDB(t)
	ctx := context.Background()
	m := NewBugModel(db)
	bug := seedBugForLink(t, db, "战斗掉线")
	ticket := seedTicketForLink(t, db, "工单A")
	ticket2 := seedTicketForLink(t, db, "工单B")
	other := seedBugForLink(t, db, "无关 bug")

	require.NoError(t, m.LinkBugTicket(ctx, bug.ID, ticket.ID, "alice"))
	require.NoError(t, m.LinkBugTicket(ctx, bug.ID, ticket2.ID, "alice"))
	require.NoError(t, m.LinkBugTicket(ctx, other.ID, ticket.ID, "alice"))

	tickets, err := m.ListTicketsByBug(ctx, bug.ID)
	require.NoError(t, err)
	require.Len(t, tickets, 2)
	assert.Equal(t, "工单A", tickets[0].Title)
	assert.Equal(t, dbenum.TicketStatusOpen.String(), tickets[0].Status)
	assert.Equal(t, "工单B", tickets[1].Title)

	bugs, err := m.ListBugsByTicket(ctx, ticket.ID)
	require.NoError(t, err)
	require.Len(t, bugs, 2)
	assert.Equal(t, "战斗掉线", bugs[0].Title)
	assert.Equal(t, BugStatusTriage, bugs[0].Status)
	assert.Equal(t, "无关 bug", bugs[1].Title)

	// 软删除的工单/缺陷不出现在对侧列表
	require.NoError(t, db.Delete(&ticket).Error)
	tickets, err = m.ListTicketsByBug(ctx, bug.ID)
	require.NoError(t, err)
	require.Len(t, tickets, 1)
	assert.Equal(t, "工单B", tickets[0].Title)

	require.NoError(t, db.Delete(&bug).Error)
	bugs, err = m.ListBugsByTicket(ctx, ticket.ID)
	require.NoError(t, err)
	require.Len(t, bugs, 1)
	assert.Equal(t, "无关 bug", bugs[0].Title)
}
