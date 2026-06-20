package game

import (
	"regexp"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
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

func buildGameInfo(game *model.Game) GameInfo {
	envItems := make([]GameEnvItem, 0)
	if envs, err := game.GetEnvs(); err == nil {
		envItems = convertGameEnvs(envs)
	}

	return GameInfo{
		ID:          game.ID,
		GameID:      game.GameID,
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

// buildGameInfoWithBindings is like buildGameInfo but also enriches each env
// item with the databaseName from the corresponding GameEnvBinding records.
func buildGameInfoWithBindings(game *model.Game, bindings []model.GameEnvBinding) GameInfo {
	info := buildGameInfo(game)
	info.Envs = mergeBindingData(info.Envs, bindings)
	return info
}

func convertGameEnvs(envs []model.GameEnv) []GameEnvItem {
	items := make([]GameEnvItem, 0, len(envs))
	for _, env := range envs {
		items = append(items, GameEnvItem{
			Env:         env.Env,
			Description: env.Description,
			Color:       env.Color,
		})
	}
	return items
}

// mergeBindingData enriches env items with databaseName from GameEnvBinding
// records. Items without a matching binding are left unchanged.
func mergeBindingData(items []GameEnvItem, bindings []model.GameEnvBinding) []GameEnvItem {
	if len(bindings) == 0 {
		return items
	}
	dbNames := make(map[string]string, len(bindings))
	for _, b := range bindings {
		dbNames[strings.ToLower(b.Env)] = b.DatabaseName
	}
	for i := range items {
		if dbName, ok := dbNames[strings.ToLower(items[i].Env)]; ok {
			items[i].DatabaseName = dbName
		}
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
