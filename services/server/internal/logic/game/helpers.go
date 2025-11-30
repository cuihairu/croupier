package game

import (
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

var allowedGameStatuses = map[string]struct{}{
	"dev":         {},
	"test":        {},
	"running":     {},
	"online":      {},
	"offline":     {},
	"maintenance": {},
}

func parseGameID(id string) (uint, error) {
	return utils.ParseUintID(id, "游戏ID")
}

func buildGameInfo(game *model.Game) types.GameInfo {
	envItems := make([]types.GameEnvItem, 0)
	if envs, err := game.GetEnvs(); err == nil {
		envItems = convertGameEnvs(envs)
	}

	return types.GameInfo{
		ID:          game.ID,
		Name:        game.Name,
		Icon:        game.Icon,
		Description: game.Description,
		Enabled:     game.Enabled,
		AliasName:   game.AliasName,
		Homepage:    game.Homepage,
		Status:      game.Status,
		GameType:    game.GameType,
		GenreCode:   game.GenreCode,
		Envs:        envItems,
		CreatedAt:   utils.FormatTimestamp(game.CreatedAt),
		UpdatedAt:   utils.FormatTimestamp(game.UpdatedAt),
	}
}

func convertGameEnvs(envs []model.GameEnv) []types.GameEnvItem {
	items := make([]types.GameEnvItem, 0, len(envs))
	for _, env := range envs {
		items = append(items, types.GameEnvItem{
			Env:         env.Env,
			Description: env.Description,
			Color:       env.Color,
		})
	}
	return items
}

func sanitizeGameName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("游戏名称不能为空")
	}
	return trimmed, nil
}

func sanitizeStatus(status string) (string, error) {
	if status == "" {
		return "", nil
	}
	val := strings.TrimSpace(status)
	if val == "" {
		return "", nil
	}
	if _, ok := allowedGameStatuses[val]; !ok {
		return "", fmt.Errorf("无效的游戏状态: %s", val)
	}
	return val, nil
}

func findEnvIndex(envs []model.GameEnv, env string) int {
	for idx, item := range envs {
		if strings.EqualFold(item.Env, env) {
			return idx
		}
	}
	return -1
}

func ensureEnvName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("环境名称不能为空")
	}
	return trimmed, nil
}
