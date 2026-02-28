package game

import (
	"regexp"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
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

var gameNamePattern = regexp.MustCompile(`^[A-Za-z0-9_@-]+$`)

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
		Color:       game.Color,
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
		return "", errorx.NewBadRequest("游戏名称不能为空")
	}
	if !gameNamePattern.MatchString(trimmed) {
		return "", errorx.NewBadRequest("游戏名称仅支持字母、数字和 _ - @")
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
		return "", errorx.NewBadRequest("无效的游戏状态: " + val)
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
		return "", errorx.NewBadRequest("环境名称不能为空")
	}
	return trimmed, nil
}
