package ticket

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandler_CreateWithPlayerContext verifies the game-support P1 context
// columns (server/level/device/os/language/extra) round-trip through the API.
func TestHandler_CreateWithPlayerContext(t *testing.T) {
	db := newTicketTestDB(t)
	h := newTicketHandler(db)

	body := `{
		"title": "recharge not arrived",
		"content": "paid 30 minutes ago, nothing in game",
		"category": "payment",
		"priority": "high",
		"playerId": "p-10086",
		"serverId": "s-12",
		"playerLevel": 45,
		"deviceOs": "iOS 18.1",
		"deviceModel": "iPhone 15 Pro",
		"language": "zh-CN",
		"extra": {"orderId": "ord-777", "sku": "gem60", "vipLevel": 3}
	}`
	c, w := newTicketRequest(http.MethodPost, "/tickets", body)
	h.Create(c)
	require.Equal(t, http.StatusOK, w.Code)

	// DB level: context persisted.
	var stored model.Ticket
	require.NoError(t, db.Where("player_id = ?", "p-10086").First(&stored).Error)
	assert.Equal(t, "s-12", stored.ServerID)
	assert.Equal(t, 45, stored.PlayerLevel)
	assert.Equal(t, "iOS 18.1", stored.DeviceOS)
	assert.Equal(t, "iPhone 15 Pro", stored.DeviceModel)
	assert.Equal(t, "zh-CN", stored.Language)
	assert.Contains(t, string(stored.Extra), "ord-777")

	// API level: detail response carries context back out.
	id := fmt.Sprint(stored.ID)
	c2, w2 := newTicketRequest(http.MethodGet, "/tickets/"+id, "")
	c2.Params = gin.Params{{Key: "id", Value: id}}
	h.Get(c2)
	require.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), `"serverId":"s-12"`)
	assert.Contains(t, w2.Body.String(), `"playerLevel":45`)
	assert.Contains(t, w2.Body.String(), `"deviceOs":"iOS 18.1"`)
	assert.Contains(t, w2.Body.String(), `"language":"zh-CN"`)
	assert.Contains(t, w2.Body.String(), "ord-777")
}

// TestHandler_CreateWithContextSanitization verifies invalid context values
// are clamped instead of rejected (support must never block on bad context).
func TestHandler_CreateWithContextSanitization(t *testing.T) {
	db := newTicketTestDB(t)
	h := newTicketHandler(db)

	body := `{
		"title": "bug report",
		"content": "stuck on loading",
		"category": "bug",
		"serverId": "  s-1  ",
		"playerLevel": -5
	}`
	c, w := newTicketRequest(http.MethodPost, "/tickets", body)
	h.Create(c)
	require.Equal(t, http.StatusOK, w.Code)

	var stored model.Ticket
	require.NoError(t, db.Where("title = ?", "bug report").First(&stored).Error)
	assert.Equal(t, "s-1", stored.ServerID)
	assert.Equal(t, 0, stored.PlayerLevel)
}
