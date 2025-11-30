package utils

import (
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

// BuildPlayer converts model.Player to API Player.
func BuildPlayer(player *model.Player) types.Player {
	if player == nil {
		return types.Player{}
	}
	return types.Player{
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
		CreatedAt: FormatTimestamp(player.CreatedAt),
		UpdatedAt: FormatTimestamp(player.UpdatedAt),
	}
}
