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

// #25: regression for the ticket-side bug association endpoints
// (GET/POST/DELETE /tickets/:id/bugs).
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

	// Link
	c, w := newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), `{"bugId":`+fmt.Sprint(bug.ID)+`}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.LinkBug(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// List carries bug summary
	c, w = newTicketRequest(http.MethodGet, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.ListBugs(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"title":"副本加载超时"`)
	assert.Contains(t, w.Body.String(), `"severity":"critical"`)

	// Idempotent: repeating the link still returns 200
	c, w = newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), `{"bugId":`+fmt.Sprint(bug.ID)+`}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.LinkBug(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Unlink
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

	// Empty bugId
	c, w := newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), `{}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.LinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "bugId")

	// Bug does not exist
	c, w = newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), `{"bugId":4242}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.LinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "bug 不存在")

	// Invalid ticket id
	c, w = newTicketRequest(http.MethodGet, "/tickets/abc/bugs", "")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.ListBugs(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// Fills in the #25/#21 error branches: bind failures, invalid path params,
// model-layer errors, and FilterOptions error propagation.
func TestTicketBugLinkErrorBranches(t *testing.T) {
	db := newTicketTestDB(t)
	svcCtx := &svc.ServiceContext{
		TicketModel: model.NewTicketModel(db),
		BugModel:    model.NewBugModel(db),
	}
	h := NewHandler(NewService(svcCtx))
	ticket := model.Ticket{Title: "x", Status: dbenum.TicketStatusOpen}
	require.NoError(t, db.Create(&ticket).Error)

	// LinkBug: invalid JSON body → bind error
	c, w := newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), `{invalid`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.LinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// LinkBug: invalid ticket id (parse failure)
	c, w = newTicketRequest(http.MethodPost, "/tickets/abc/bugs", `{"bugId":1}`)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.LinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// UnlinkBug: invalid ticket id / invalid bugId (the handler's two parse branches)
	c, w = newTicketRequest(http.MethodDelete, "/tickets/abc/bugs/1", "")
	c.Params = gin.Params{{Key: "id", Value: "abc"}, {Key: "bugId", Value: "1"}}
	h.UnlinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = newTicketRequest(http.MethodDelete, "/tickets/1/bugs/abc", "")
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "bugId", Value: "abc"}}
	h.UnlinkBug(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// ListBugs: model-layer error (query fails after closing the underlying connection)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	c, w = newTicketRequest(http.MethodGet, fmt.Sprintf("/tickets/%d/bugs", ticket.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.ListBugs(c)
	assert.NotEqual(t, http.StatusOK, w.Code)

	// UnlinkBug: model-layer error (valid params + closed DB → service error propagated)
	c, w = newTicketRequest(http.MethodDelete, fmt.Sprintf("/tickets/%d/bugs/1", ticket.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}, {Key: "bugId", Value: "1"}}
	h.UnlinkBug(c)
	assert.NotEqual(t, http.StatusOK, w.Code)

	// FilterOptions: model-layer error (same closed-DB setup as above)
	c, w = newTicketRequest(http.MethodGet, "/tickets/filter-options", "")
	h.FilterOptions(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}
