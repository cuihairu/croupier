package ticket

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToBug_CarriesPlayerContext(t *testing.T) {
	db := newTicketTestDB(t)
	svcCtx := &svc.ServiceContext{
		TicketModel: model.NewTicketModel(db),
		BugModel:    model.NewBugModel(db),
	}
	h := NewHandler(NewService(svcCtx))

	// Ticket with full player context.
	c, w := newTicketRequest(http.MethodPost, "/tickets", `{
		"title":"iOS 闪退",
		"content":"进入背包必闪退",
		"category":"bug",
		"priority":"high",
		"playerId":"p-7",
		"serverId":"s-2",
		"platform":"ios",
		"deviceOs":"iOS 18.1",
		"deviceModel":"iPhone 15",
		"extra":{"crashId":"cr-123"}
	}`)
	h.Create(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var ticket model.Ticket
	require.NoError(t, db.First(&ticket).Error)

	// Convert.
	c, w = newTicketRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/convert-bug", ticket.ID),
		`{"severity":"critical"}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
	h.ConvertToBug(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"bugId":"1"`)

	// Bug carries everything.
	var bug model.Bug
	require.NoError(t, db.First(&bug).Error)
	assert.Equal(t, "ticket", bug.Source)
	assert.Equal(t, ticket.ID, bug.SourceTicketID)
	assert.Equal(t, "p-7", bug.PlayerID)
	assert.Equal(t, "s-2", bug.ServerID)
	assert.Equal(t, "iPhone 15", bug.Device)
	assert.Equal(t, "iOS 18.1", bug.OS)
	assert.Equal(t, "critical", bug.Severity)
	assert.Equal(t, "high", bug.Priority) // inherited from ticket
	assert.Equal(t, model.BugStatusTriage, bug.Status)
	assert.Contains(t, bug.Title, "[工单#")
	assert.Contains(t, fmt.Sprint(bug.Extra), "crashId")

	// Ticket got the audit comment.
	var comments []model.TicketComment
	db.Where("ticket_id = ?", ticket.ID).Find(&comments)
	require.NotEmpty(t, comments)
	assert.Contains(t, comments[0].Content, "[升级缺陷]")
}

func TestConvertToBug_NotFound(t *testing.T) {
	db := newTicketTestDB(t)
	svcCtx := &svc.ServiceContext{
		TicketModel: model.NewTicketModel(db),
		BugModel:    model.NewBugModel(db),
	}
	h := NewHandler(NewService(svcCtx))
	c, w := newTicketRequest(http.MethodPost, "/tickets/999/convert-bug", `{}`)
	c.Params = gin.Params{{Key: "id", Value: "999"}}
	h.ConvertToBug(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}
