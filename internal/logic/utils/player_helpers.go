package utils

import (
	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
)

// BuildPlayer converts model.Player to API Player.
func BuildPlayer(player *model.Player) Player {
	if player == nil {
		return Player{}
	}
	return Player{
		Id:        int64(player.ID),
		Username:  player.Username,
		Nickname:  player.Nickname,
		Email:     player.Email,
		Phone:     player.Phone,
		GameId:    player.GameID,
		Status:    player.Status,
		Balance:   player.Balance,
		Level:     player.Level,
		Vip:       player.VIP,
		CreatedAt: helper.FormatTimestamp(player.CreatedAt),
		UpdatedAt: helper.FormatTimestamp(player.UpdatedAt),
	}
}

// Local type for backward compatibility
type Player struct {
	Id        int64  `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	GameId    string `json:"gameId"`
	Status    int    `json:"status"`
	Balance   int64  `json:"balance"`
	Level     int    `json:"level"`
	Vip       int    `json:"vip"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
