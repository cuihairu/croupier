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

// 补 #25/#21 错误分支：绑定失败、路径参数非法、模型层错误、FilterOptions 错误透传。
func TestTicketBugLinkErrorBranches(t *testing.T) {
	db := newTicketTestDB(t)
	svcCtx := &svc.ServiceContext{
		TicketModel: model.NewTicketModel(db),
		BugModel:    model.NewBugModel(db),
	}
	h := NewHandler(NewService(svcCtx))
	ticket := model.Ticket{Title: "x", Status: dbenum.TicketStatusOpen}
	require.NoError(t, db.Create(&ticket).Error)

	// LinkBug：非法 JSON 体 → 绑定错误
	c, w := newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), `{invalid`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.LinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// LinkBug：非法工单 id（parse 失败）
	c, w = newTicketRequest(http.MethodPost, "/tickets/abc/bugs", `{"bugId":1}`)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.LinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// UnlinkBug：非法工单 id / 非法 bugId（handler 两处 parse 分支）
	c, w = newTicketRequest(http.MethodDelete, "/tickets/abc/bugs/1", "")
	c.Params = gin.Params{{Key: "id", Value: "abc"}, {Key: "bugId", Value: "1"}}
	h.UnlinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = newTicketRequest(http.MethodDelete, "/tickets/1/bugs/abc", "")
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "bugId", Value: "abc"}}
	h.UnlinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// ListBugs：模型层错误（关闭底层连接后查询失败）
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	c, w = newTicketRequest(http.MethodGet, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.ListBugs(c)
	assert.NotEqual(t, http.StatusOK, w.Code)

	// UnlinkBug：模型层错误（合法参数 + 关库 → service 错误透传）
	c, w = newTicketRequest(http.MethodDelete, fmt.Sprintf("/tickets/%d/bugs/1", ticket.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}, {Key: "bugId", Value: "1"}}
	h.UnlinkBug(c)
	assert.NotEqual(t, http.StatusOK, w.Code)

	// FilterOptions：模型层错误（同上关库）
	c, w = newTicketRequest(http.MethodGet, "/tickets/filter-options", "")
	h.FilterOptions(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}
