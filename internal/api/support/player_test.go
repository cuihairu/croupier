package support

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newPlayerHandler(t *testing.T) *PlayerHandler {
	t.Helper()
	name := fmt.Sprintf("support_player_%s", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return NewPlayerHandler(&svc.ServiceContext{
		FAQModel:    model.NewFAQModel(db),
		TicketModel: model.NewTicketModel(db),
	})
}

func playerReq(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reader := strings.NewReader(body)
	if body == "" {
		reader = strings.NewReader("")
	}
	c.Request = httptest.NewRequest(method, target, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestPlayerFAQs_OnlyVisible(t *testing.T) {
	h := newPlayerHandler(t)
	require.NoError(t, h.svcCtx.FAQModel.Create(nil, &model.FAQ{
		Question: "怎么充值", Answer: "商店页", Category: "pay", Visible: true, Slug: "pay-how",
	}))
	hidden := &model.FAQ{Question: "隐藏条目", Answer: "内部", Category: "pay", Visible: false}
	require.NoError(t, h.svcCtx.FAQModel.Create(nil, hidden))
	// gorm default:true 会把 Create 时的零值 false 变 true，用 Update 显式隐藏
	require.NoError(t, h.svcCtx.FAQModel.Update(nil, hidden.ID, map[string]interface{}{"visible": false}))

	c, w := playerReq(http.MethodGet, "/public/support/faqs", "")
	h.ListFAQs(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "怎么充值")
	assert.NotContains(t, w.Body.String(), "隐藏条目")
}

func TestPlayerCreateTicket_Validation(t *testing.T) {
	h := newPlayerHandler(t)
	cases := []string{
		`{"title":"","content":"x","playerId":"p"}`,
		`{"title":"t","content":"","playerId":"p"}`,
		`{"title":"t","content":"c","playerId":""}`,
	}
	for _, body := range cases {
		c, w := playerReq(http.MethodPost, "/public/support/tickets", body)
		h.CreateTicket(c)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
	}
}

func TestPlayerTicketRoundTrip(t *testing.T) {
	h := newPlayerHandler(t)

	// Create from the game client with full player context.
	c, w := playerReq(http.MethodPost, "/public/support/tickets?gameId=demo&env=prod", `{
		"title":"背包打不开",
		"content":"点背包按钮没反应",
		"playerId":"p-100",
		"serverId":"s-1",
		"playerLevel": 88,
		"deviceOs":"Android 15",
		"deviceModel":"Pixel 9",
		"language":"zh-CN"
	}`)
	h.CreateTicket(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"status":"open"`)

	var stored model.Ticket
	require.NoError(t, h.svcCtx.TicketModel.DB().First(&stored).Error)
	assert.Equal(t, "player", stored.Source)
	assert.Equal(t, "demo", stored.GameID)
	assert.Equal(t, "s-1", stored.ServerID)
	assert.Equal(t, 88, stored.PlayerLevel)

	// Player lists their own tickets: public projection only.
	c, w = playerReq(http.MethodGet, "/public/support/tickets?playerId=p-100", "")
	h.ListMyTickets(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "背包打不开")
	assert.NotContains(t, w.Body.String(), "assignee", "internal fields must not leak")

	// Another player sees nothing.
	c, w = playerReq(http.MethodGet, "/public/support/tickets?playerId=p-999", "")
	h.ListMyTickets(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "背包打不开")
}
