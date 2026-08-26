package feedback

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func convertFixture(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	db := newFeedbackTestDB(t)
	svcCtx := &svc.ServiceContext{
		FeedbackModel: model.NewFeedbackModel(db),
		TicketModel:   model.NewTicketModel(db),
	}
	return NewHandler(NewService(svcCtx)), db
}

func convertRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reader := strings.NewReader(body)
	if body == "" {
		reader = strings.NewReader("{}")
	}
	c.Request = httptest.NewRequest(method, target, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestConvertToTicket_RoundTrip(t *testing.T) {
	h, db := convertFixture(t)

	// Seed one feedback.
	fb := &model.Feedback{
		PlayerID: "p-9", Contact: "wx:foo", Content: "充值没到账\n已经等了30分钟",
		Category: "payment", GameID: "demo", Env: "prod", Rating: 2,
	}
	require.NoError(t, db.Create(fb).Error)

	// Convert.
	c, w := convertRequest(http.MethodPost, fmt.Sprintf("/feedback/%d/convert", fb.ID),
		`{"note":"高优先级跟进"}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(fb.ID)}}
	h.ConvertToTicket(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"ticketId":"1"`)

	// Ticket carries player context + source + original content.
	var ticket model.Ticket
	require.NoError(t, db.First(&ticket).Error)
	assert.Equal(t, "p-9", ticket.PlayerID)
	assert.Equal(t, "feedback", ticket.Source)
	assert.Equal(t, "demo", ticket.GameID)
	assert.Contains(t, ticket.Content, "充值没到账")
	assert.Contains(t, ticket.Content, "高优先级跟进")
	assert.Contains(t, string(ticket.Extra), "feedbackId")

	// Feedback marked triaged with the marker.
	var fbAfter model.Feedback
	require.NoError(t, db.First(&fbAfter, fb.ID).Error)
	assert.Equal(t, dbenum.FeedbackStatusTriaged, fbAfter.Status)
	assert.Contains(t, fbAfter.Reply, "[已转工单 #")

	// Idempotent: converting again returns the same ticket, no duplicate.
	c, w = convertRequest(http.MethodPost, fmt.Sprintf("/feedback/%d/convert", fb.ID), `{}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(fb.ID)}}
	h.ConvertToTicket(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"alreadyConverted":true`)
	var count int64
	db.Model(&model.Ticket{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestConvertToTicket_NotFound(t *testing.T) {
	h, _ := convertFixture(t)
	c, w := convertRequest(http.MethodPost, "/feedback/999/convert", `{}`)
	c.Params = gin.Params{{Key: "id", Value: "999"}}
	h.ConvertToTicket(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}
