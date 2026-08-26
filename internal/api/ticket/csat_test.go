package ticket

import (
	"context"
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

func TestRate_OnlyClosedTickets(t *testing.T) {
	db := newTicketTestDB(t)
	h := newTicketHandler(db)

	// Create an open ticket.
	c, w := newTicketRequest(http.MethodPost, "/tickets",
		`{"title":"t","content":"c","category":"x"}`)
	h.Create(c)
	require.Equal(t, http.StatusOK, w.Code)
	var stored model.Ticket
	require.NoError(t, db.First(&stored).Error)

	// Rating an open ticket → conflict.
	c, w = newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/rate", stored.ID), `{"rating":5}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(stored.ID)}}
	h.Rate(c)
	assert.Equal(t, http.StatusConflict, w.Code)

	// Resolve then rate → ok.
	require.NoError(t, db.Model(&model.Ticket{}).Where("id = ?", stored.ID).
		Update("status", dbenum.TicketStatusResolved).Error)
	c, w = newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/rate", stored.ID), `{"rating":4}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(stored.ID)}}
	h.Rate(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"rating":4`)

	// Invalid range → 400.
	c, w = newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/rate", stored.ID), `{"rating":9}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(stored.ID)}}
	h.Rate(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTransition_ReopenClearsRating(t *testing.T) {
	db := newTicketTestDB(t)
	h := newTicketHandler(db)

	c, w := newTicketRequest(http.MethodPost, "/tickets", `{"title":"t","content":"c","category":"x"}`)
	h.Create(c)
	require.Equal(t, http.StatusOK, w.Code)
	var stored model.Ticket
	require.NoError(t, db.First(&stored).Error)

	// Resolve + rate.
	ticketSvc := NewService(&svc.ServiceContext{TicketModel: model.NewTicketModel(db)})
	req := &TransitionRequest{ID: fmt.Sprint(stored.ID), Status: "resolved"}
	_, err := ticketSvc.Transition(context.Background(), req)
	require.NoError(t, err)
	c, w = newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/rate", stored.ID), `{"rating":5}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(stored.ID)}}
	h.Rate(c)
	require.Equal(t, http.StatusOK, w.Code)

	// Reopen → rating cleared.
	req = &TransitionRequest{ID: fmt.Sprint(stored.ID), Status: "open"}
	_, err = ticketSvc.Transition(context.Background(), req)
	require.NoError(t, err)
	var after model.Ticket
	require.NoError(t, db.First(&after, stored.ID).Error)
	assert.Zero(t, after.Rating)
	assert.Empty(t, after.RatedBy)
	assert.Nil(t, after.RatedAt)
}
