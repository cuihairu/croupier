package bug

import (
	"fmt"
	"net/http"
	"testing"

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
