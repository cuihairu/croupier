package ticket

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #25：工单侧关联 bug 的端点回归（GET/POST/DELETE /tickets/:id/bugs）。
func TestTicketBugLinkEndpoints(t *testing.T) {
	db := newTicketTestDB(t)
	svcCtx := &svc.ServiceContext{
		TicketModel: model.NewTicketModel(db),
		BugModel:    model.NewBugModel(db),
	}
	h := NewHandler(NewService(svcCtx))

	ticket := model.Ticket{Title: "玩家无法进入副本", Status: dbenum.TicketStatusOpen, Priority: "urgent"}
	require.NoError(t, db.Create(&ticket).Error)
	bug := model.Bug{Title: "副本加载超时", Status: model.BugStatusTriage, Severity: "critical"}
	require.NoError(t, db.Create(&bug).Error)

	// 关联
	c, w := newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), `{"bugId":`+fmt.Sprint(bug.ID)+`}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.LinkBug(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// 列表带 bug 摘要
	c, w = newTicketRequest(http.MethodGet, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.ListBugs(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"title":"副本加载超时"`)
	assert.Contains(t, w.Body.String(), `"severity":"critical"`)

	// 幂等：重复关联仍 200
	c, w = newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), `{"bugId":`+fmt.Sprint(bug.ID)+`}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.LinkBug(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// 解除关联
	c, w = newTicketRequest(http.MethodDelete, fmt.Sprintf("/tickets/%d/bugs/%d", ticket.ID, bug.ID), "")
	c.Params = gin.Params{
		{Key: "id", Value: fmt.Sprint(ticket.ID)},
		{Key: "bugId", Value: fmt.Sprint(bug.ID)},
	}
	h.UnlinkBug(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	c, w = newTicketRequest(http.MethodGet, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.ListBugs(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"items":[]`)
}

func TestTicketBugLinkEndpoints_Validation(t *testing.T) {
	db := newTicketTestDB(t)
	svcCtx := &svc.ServiceContext{
		TicketModel: model.NewTicketModel(db),
		BugModel:    model.NewBugModel(db),
	}
	h := NewHandler(NewService(svcCtx))
	ticket := model.Ticket{Title: "x", Status: dbenum.TicketStatusOpen}
	require.NoError(t, db.Create(&ticket).Error)

	// 空 bugId
	c, w := newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), `{}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.LinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "bugId")

	// bug 不存在
	c, w = newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), `{"bugId":4242}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.LinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "bug 不存在")

	// 非法工单 id
	c, w = newTicketRequest(http.MethodGet, "/tickets/abc/bugs", "")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.ListBugs(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}
