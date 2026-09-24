package ticket

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/dbenum"
)

// 工单 ID 解析与创建字段消毒：非法输入 400，空白 trim，默认 status=open/source=api。

func TestParseTicketID(t *testing.T) {
	id, err := parseTicketID("42")
	require.NoError(t, err)
	assert.EqualValues(t, 42, id)

	_, err = parseTicketID("")
	require.Error(t, err)

	_, err = parseTicketID("abc")
	require.Error(t, err)

	_, err = parseTicketID("-1")
	require.Error(t, err)

	_, err = parseTicketID("0")
	require.Error(t, err)
}

func TestSanitizeTicketFields_Validation(t *testing.T) {
	// 标题空。
	_, err := sanitizeTicketFields(&CreateRequest{Content: "c", Category: "bug"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "标题")

	// 内容空。
	_, err = sanitizeTicketFields(&CreateRequest{Title: "  t  ", Category: "bug"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "内容")

	// 分类空（全空白也算空）。
	_, err = sanitizeTicketFields(&CreateRequest{Title: "t", Content: "c", Category: "   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "分类")
}

func TestSanitizeTicketFields_HappyPathAndTrim(t *testing.T) {
	ticket, err := sanitizeTicketFields(&CreateRequest{
		Title:       "  充值不到账  ",
		Content:     " 玩家反馈未到账 ",
		Category:    " payment ",
		Priority:    "high",
		Assignee:    " alice ",
		Tags:        []string{"vip", "urgent"},
		PlayerId:    " p1 ",
		Contact:     "13800000000",
		GameId:      " demo ",
		Env:         " prod ",
		ServerId:    " s1 ",
		PlayerLevel: 999999, // 超上限 clamp 到 10000
		DeviceOS:    "iOS",
		Language:    "zh-CN",
	})
	require.NoError(t, err)
	assert.Equal(t, "充值不到账", ticket.Title)
	assert.Equal(t, "玩家反馈未到账", ticket.Content)
	assert.Equal(t, "payment", ticket.Category)
	assert.Equal(t, "alice", ticket.Assignee)
	assert.Equal(t, "demo", ticket.GameID)
	assert.Equal(t, "prod", ticket.Env)
	assert.Equal(t, "p1", ticket.PlayerID)
	assert.Equal(t, "s1", ticket.ServerID)
	assert.Equal(t, 10000, ticket.PlayerLevel)
	assert.Equal(t, "api", ticket.Source)
	assert.Equal(t, dbenum.TicketStatusOpen, ticket.Status)
	assert.Equal(t, "high", ticket.Priority)
}

func TestSanitizePlayerLevel_Clamps(t *testing.T) {
	assert.Equal(t, 0, sanitizePlayerLevel(-5))
	assert.Equal(t, 0, sanitizePlayerLevel(0))
	assert.Equal(t, 50, sanitizePlayerLevel(50))
	assert.Equal(t, 10000, sanitizePlayerLevel(10001))
}
