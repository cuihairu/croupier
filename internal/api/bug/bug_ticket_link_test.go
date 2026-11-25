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

// #25: regression for the bug-side ticket linking endpoints (GET/POST/DELETE
// /bugs/:id/tickets).
func TestBugTicketLinkEndpoints(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)

	bug := model.Bug{Title: "结算金额错误", Status: model.BugStatusTriage}
	require.NoError(t, db.Create(&bug).Error)
	ticket := model.Ticket{Title: "玩家投诉结算", Status: dbenum.TicketStatusOpen, Priority: "high", GameID: "demo", Env: "prod"}
	require.NoError(t, db.Create(&ticket).Error)

	// Link
	c, w := bugRequest(http.MethodPost, fmt.Sprintf("/bugs/%d/tickets", bug.ID), `{"ticketId":`+fmt.Sprint(ticket.ID)+`}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.LinkTicket(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// List with ticket summaries
	c, w = bugRequest(http.MethodGet, fmt.Sprintf("/bugs/%d/tickets", bug.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.ListTickets(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"title":"玩家投诉结算"`)
	assert.Contains(t, w.Body.String(), `"gameId":"demo"`)
	assert.Contains(t, w.Body.String(), `"status":"open"`)

	// Idempotency: re-linking still returns 200
	c, w = bugRequest(http.MethodPost, fmt.Sprintf("/bugs/%d/tickets", bug.ID), `{"ticketId":`+fmt.Sprint(ticket.ID)+`}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.LinkTicket(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Unlink
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

	// Empty ticketId
	c, w := bugRequest(http.MethodPost, fmt.Sprintf("/bugs/%d/tickets", bug.ID), `{}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.LinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ticketId")

	// Ticket does not exist
	c, w = bugRequest(http.MethodPost, fmt.Sprintf("/bugs/%d/tickets", bug.ID), `{"ticketId":4242}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.LinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "工单不存在")

	// Invalid bug id
	c, w = bugRequest(http.MethodGet, "/bugs/abc/tickets", "")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.ListTickets(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// Additional #25 error branches: binding failures, invalid path params, model-layer
// errors, and non-sentinel errors passed through.
func TestBugTicketLinkErrorBranches(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	bug := model.Bug{Title: "x", Status: model.BugStatusTriage}
	require.NoError(t, db.Create(&bug).Error)

	// LinkTicket: invalid JSON body -> binding error
	c, w := bugRequest(http.MethodPost, fmt.Sprintf("/bugs/%d/tickets", bug.ID), `{invalid`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.LinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// LinkTicket: invalid bug id (parse failure)
	c, w = bugRequest(http.MethodPost, "/bugs/abc/tickets", `{"ticketId":1}`)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.LinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// UnlinkTicket: invalid bug id / invalid ticketId (the two service parse branches +
	// handler error pass-through)
	c, w = bugRequest(http.MethodDelete, "/bugs/abc/tickets/1", "")
	c.Params = gin.Params{{Key: "id", Value: "abc"}, {Key: "ticketId", Value: "1"}}
	h.UnlinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = bugRequest(http.MethodDelete, "/bugs/1/tickets/abc", "")
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "ticketId", Value: "abc"}}
	h.UnlinkTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// ListTickets: model-layer error (query fails after the underlying connection is
	// closed)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	c, w = bugRequest(http.MethodGet, fmt.Sprintf("/bugs/%d/tickets", bug.ID), "")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(bug.ID)}}
	h.ListTickets(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestTranslateBugLinkError(t *testing.T) {
	// Sentinel errors -> 4xx CodeError (the message keeps the original text)
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

	// Non-sentinel errors pass through unchanged (the upper layer handles them as 500)
	plain := errors.New("boom")
	assert.Same(t, plain, TranslateBugLinkError(plain))
	assert.Nil(t, TranslateBugLinkError(nil))
}
