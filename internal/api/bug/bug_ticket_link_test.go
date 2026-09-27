package bug

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #25：缺陷侧关联工单的端点回归（GET/POST/DELETE /bugs/:id/tickets）。
func TestBugTicketLinkEndpoints(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)

	bug := model.Bug{Title: "结算金额错误", Status: model.BugStatusTriage}
	require.NoError(t, db.Create(&bug).Error)
	ticket := model.Ticket{Title: "玩家投诉结算", Status: dbenum.TicketStatusOpen, Priority: "high", GameID: "demo", Env: "prod"}
	require.NoError(t, db.Create(&ticket).Error)

	// 关联
	c, w := bugRequest(http.MethodPost, fmt.Sprintf("/bugs/%d/tickets", bug.ID), `{"ticketId":`+fmt.Sprint(ticket.ID)+`}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.LinkTicket(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// 列表带工单摘要
	c, w = bugRequest(http.MethodGet, fmt.Sprintf("/bugs/%d/tickets", bug.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.ListTickets(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"title":"玩家投诉结算"`)
	assert.Contains(t, w.Body.String(), `"gameId":"demo"`)
	assert.Contains(t, w.Body.String(), `"status":"open"`)

	// 幂等：重复关联仍 200
	c, w = bugRequest(http.MethodPost, fmt.Sprintf("/bugs/%d/tickets", bug.ID), `{"ticketId":`+fmt.Sprint(ticket.ID)+`}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.LinkTicket(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// 解除关联
	c, w = bugRequest(http.MethodDelete, fmt.Sprintf("/bugs/%d/tickets/%d", bug.ID, ticket.ID), "")
	c.Params = gin.Params{
		{Key: "id", Value: fmt.Sprint(bug.ID)},
		{Key: "ticketId", Value: fmt.Sprint(ticket.ID)},
	}
	h.UnlinkTicket(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	c, w = bugRequest(http.MethodGet, fmt.Sprintf("/bugs/%d/tickets", bug.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.ListTickets(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"items":[]`)
}

func TestBugTicketLinkEndpoints_Validation(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	bug := model.Bug{Title: "x", Status: model.BugStatusTriage}
	require.NoError(t, db.Create(&bug).Error)

	// 空 ticketId
	c, w := bugRequest(http.MethodPost, fmt.Sprintf("/bugs/%d/tickets", bug.ID), `{}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.LinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ticketId")

	// 工单不存在
	c, w = bugRequest(http.MethodPost, fmt.Sprintf("/bugs/%d/tickets", bug.ID), `{"ticketId":4242}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.LinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "工单不存在")

	// 非法 bug id
	c, w = bugRequest(http.MethodGet, "/bugs/abc/tickets", "")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.ListTickets(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// 补 #25 错误分支：绑定失败、路径参数非法、模型层错误、非哨兵错误透传。
func TestBugTicketLinkErrorBranches(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	bug := model.Bug{Title: "x", Status: model.BugStatusTriage}
	require.NoError(t, db.Create(&bug).Error)

	// LinkTicket：非法 JSON 体 → 绑定错误
	c, w := bugRequest(http.MethodPost, fmt.Sprintf("/bugs/%d/tickets", bug.ID), `{invalid`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.LinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// LinkTicket：非法 bug id（parse 失败）
	c, w = bugRequest(http.MethodPost, "/bugs/abc/tickets", `{"ticketId":1}`)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.LinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// UnlinkTicket：非法 bug id / 非法 ticketId（service 两处 parse 分支 + handler 错误透传）
	c, w = bugRequest(http.MethodDelete, "/bugs/abc/tickets/1", "")
	c.Params = gin.Params{{Key: "id", Value: "abc"}, {Key: "ticketId", Value: "1"}}
	h.UnlinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = bugRequest(http.MethodDelete, "/bugs/1/tickets/abc", "")
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "ticketId", Value: "abc"}}
	h.UnlinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// ListTickets：模型层错误（关闭底层连接后查询失败）
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	c, w = bugRequest(http.MethodGet, fmt.Sprintf("/bugs/%d/tickets", bug.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.ListTickets(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestTranslateBugLinkError(t *testing.T) {
	// 哨兵错误 → 4xx CodeError（消息保留原文）
	for _, sentinel := range []error{
		model.ErrBugLinkInvalidID,
		model.ErrBugLinkBugMissing,
		model.ErrBugLinkTicketMissing,
	} {
		mapped := TranslateBugLinkError(sentinel)
		var codeErr *errorx.CodeError
		require.ErrorAs(t, mapped, &codeErr)
		assert.Equal(t, http.StatusBadRequest, codeErr.Code)
		assert.Contains(t, codeErr.Message, sentinel.Error())
	}

	// 非哨兵错误原样透传（交由上层按 500 处理）
	plain := errors.New("boom")
	assert.Same(t, plain, TranslateBugLinkError(plain))
	assert.Nil(t, TranslateBugLinkError(nil))
}
