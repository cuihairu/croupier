// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProfileGamesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取我的游戏
func NewProfileGamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfileGamesLogic {
	return &ProfileGamesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProfileGamesLogic) ProfileGames(req *types.ProfileGamesRequest) (resp *types.ProfileGamesResponse, err error) {
	admin, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	roleNames := utils.RoleNamesFromModels(roles)
	isAdmin := utils.HasAdminRole(roleNames)

	catalog := l.loadGameCatalog()

	respGames := []types.ProfileGame(nil)
	if l.svcCtx.ProfileModel != nil {
		scopedRecords, err := l.svcCtx.ProfileModel.ListGames(l.ctx, admin.ID)
		if err != nil {
			return nil, fmt.Errorf("获取游戏列表失败: %w", err)
		}
		respGames = enrichProfileGames(scopedRecords, catalog)
	}

	// Fallback: derive from admin_game_scopes tables.
	if len(respGames) == 0 && !isAdmin {
		respGames, err = l.deriveGamesFromScopes(admin.ID, catalog)
		if err != nil {
			return nil, err
		}
	}

	if len(respGames) == 0 && isAdmin {
		respGames = buildGamesFromCatalog(catalog)
	}

	return &types.ProfileGamesResponse{
		Games: respGames,
	}, nil
}

func (l *ProfileGamesLogic) deriveGamesFromScopes(adminID uint, catalog []model.Game) ([]types.ProfileGame, error) {
	if l.svcCtx == nil || l.svcCtx.DB == nil || l.svcCtx.GameModel == nil {
		return nil, errors.New("DB/GameModel 未初始化")
	}

	lookup := buildGameLookup(catalog)
	type envRow struct {
		GameID   uint
		GameName string
		Alias    string
		Env      string
	}
	var rows []envRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Table("admin_game_env_scopes").
		Select("admin_game_env_scopes.game_id as game_id, games.name as game_name, games.alias_name as alias, admin_game_env_scopes.env as env").
		Joins("INNER JOIN games ON games.id = admin_game_env_scopes.game_id").
		Where("admin_game_env_scopes.admin_id = ?", adminID).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("获取游戏范围失败: %w", err)
	}

	envsByName := make(map[string][]string)
	nameByName := make(map[string]string)
	for _, r := range rows {
		gameID := strings.TrimSpace(r.GameName)
		if gameID == "" {
			continue
		}
		env := strings.TrimSpace(r.Env)
		if env != "" {
			envsByName[gameID] = append(envsByName[gameID], env)
		}
		display := strings.TrimSpace(r.Alias)
		if display == "" {
			display = gameID
		}
		nameByName[gameID] = display
	}

	// Include game-only scopes (all envs).
	var gameScopes []model.AdminGameScope
	if err := l.svcCtx.DB.WithContext(l.ctx).Where("admin_id = ?", adminID).Find(&gameScopes).Error; err != nil {
		return nil, fmt.Errorf("获取游戏范围失败: %w", err)
	}
	for _, scope := range gameScopes {
		game, err := l.svcCtx.GameModel.FindOne(l.ctx, scope.GameID)
		if err != nil || game == nil {
			continue
		}
		if _, ok := envsByName[game.Name]; ok {
			continue
		}
		envsByName[game.Name] = []string{}
		if _, ok := nameByName[game.Name]; !ok {
			nameByName[game.Name] = strings.TrimSpace(game.AliasName)
		}
	}

	if len(envsByName) == 0 {
		return nil, nil
	}

	resp := make([]types.ProfileGame, 0, len(envsByName))
	for gameID, envs := range envsByName {
		display := strings.TrimSpace(nameByName[gameID])
		if display == "" {
			display = gameID
		}
		// If envs are not specified, fall back to all envs in catalog metadata when possible.
		if len(envs) == 0 {
			if game, ok := lookup[strings.ToLower(gameID)]; ok {
				meta, names := buildEnvMeta(game, nil)
				envs = names
				resp = append(resp, types.ProfileGame{
					GameId:      gameID,
					GameName:    display,
					Color:       game.Color,
					Envs:        envs,
					EnvMeta:     meta,
					Permissions: []string{},
				})
				continue
			}
		}

		if game, ok := lookup[strings.ToLower(gameID)]; ok {
			meta, names := buildEnvMeta(game, envs)
			resp = append(resp, types.ProfileGame{
				GameId:      gameID,
				GameName:    display,
				Color:       game.Color,
				Envs:        names,
				EnvMeta:     meta,
				Permissions: []string{},
			})
		} else {
			resp = append(resp, types.ProfileGame{
				GameId:      gameID,
				GameName:    display,
				Envs:        envs,
				EnvMeta:     buildFallbackEnvMeta(envs),
				Permissions: []string{},
			})
		}
	}
	return resp, nil
}

func (l *ProfileGamesLogic) loadGameCatalog() []model.Game {
	if l.svcCtx.GameModel == nil {
		return nil
	}
	games, err := l.svcCtx.ListAllGamesCached(l.ctx)
	if err != nil {
		logx.Errorf("failed to load game catalog: %v", err)
		return nil
	}
	return games
}

func enrichProfileGames(records []model.ProfileGame, catalog []model.Game) []types.ProfileGame {
	if len(records) == 0 {
		return nil
	}
	lookup := buildGameLookup(catalog)
	resp := make([]types.ProfileGame, 0, len(records))
	for _, record := range records {
		envNames := utils.DecodeStringSlice(record.Envs)
		perms := utils.DecodeStringSlice(record.Permissions)
		gameName := strings.TrimSpace(record.GameName)
		if gameName == "" {
			gameName = record.GameID
		}
		item := types.ProfileGame{
			GameId:      record.GameID,
			GameName:    gameName,
			Color:       record.Color,
			Envs:        envNames,
			Permissions: perms,
		}

		if game, ok := lookup[strings.ToLower(record.GameID)]; ok {
			if item.Color == "" {
				item.Color = game.Color
			}
			meta, names := buildEnvMeta(game, envNames)
			if len(meta) > 0 {
				item.EnvMeta = meta
			}
			if len(names) > 0 {
				item.Envs = names
			} else if len(item.Envs) == 0 && len(meta) > 0 {
				item.Envs = extractEnvNames(meta)
			}
			if item.GameName == "" {
				item.GameName = safeAlias(game)
			}
		} else if len(envNames) > 0 {
			item.EnvMeta = buildFallbackEnvMeta(envNames)
		}

		resp = append(resp, item)
	}
	return resp
}

func buildGamesFromCatalog(catalog []model.Game) []types.ProfileGame {
	if len(catalog) == 0 {
		return nil
	}
	resp := make([]types.ProfileGame, 0, len(catalog))
	for _, game := range catalog {
		meta, names := buildEnvMeta(game, nil)
		resp = append(resp, types.ProfileGame{
			GameId:      game.Name,
			GameName:    safeAlias(game),
			Color:       game.Color,
			Envs:        names,
			EnvMeta:     meta,
			Permissions: []string{},
		})
	}
	return resp
}

func buildGameLookup(catalog []model.Game) map[string]model.Game {
	if len(catalog) == 0 {
		return nil
	}
	lookup := make(map[string]model.Game, len(catalog))
	for _, game := range catalog {
		key := strings.ToLower(strings.TrimSpace(game.Name))
		if key == "" {
			continue
		}
		lookup[key] = game
	}
	return lookup
}

func buildEnvMeta(game model.Game, allowed []string) ([]types.GameEnvItem, []string) {
	envRecords, err := game.GetEnvs()
	if err != nil {
		return buildFallbackEnvMeta(allowed), cloneStrings(allowed)
	}
	if len(envRecords) == 0 {
		return buildFallbackEnvMeta(allowed), cloneStrings(allowed)
	}

	if len(allowed) == 0 {
		meta := convertEnvRecords(envRecords)
		return meta, extractEnvNames(meta)
	}

	allowedLookup := make(map[string]struct{}, len(allowed))
	for _, env := range allowed {
		if trimmed := strings.TrimSpace(env); trimmed != "" {
			allowedLookup[strings.ToLower(trimmed)] = struct{}{}
		}
	}
	envMap := make(map[string]model.GameEnv, len(envRecords))
	for _, env := range envRecords {
		envMap[strings.ToLower(env.Env)] = env
	}

	meta := make([]types.GameEnvItem, 0, len(allowed))
	names := make([]string, 0, len(allowed))
	for _, env := range allowed {
		trimmed := strings.TrimSpace(env)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		names = append(names, trimmed)

		if record, ok := envMap[key]; ok {
			meta = append(meta, types.GameEnvItem{
				Env:         record.Env,
				Description: record.Description,
				Color:       record.Color,
			})
		} else {
			meta = append(meta, types.GameEnvItem{
				Env:   trimmed,
				Color: fallbackEnvColor(trimmed),
			})
		}
		delete(allowedLookup, key)
	}

	return meta, names
}

func convertEnvRecords(envs []model.GameEnv) []types.GameEnvItem {
	meta := make([]types.GameEnvItem, 0, len(envs))
	for _, env := range envs {
		meta = append(meta, types.GameEnvItem{
			Env:         env.Env,
			Description: env.Description,
			Color:       env.Color,
		})
	}
	return meta
}

func extractEnvNames(meta []types.GameEnvItem) []string {
	if len(meta) == 0 {
		return nil
	}
	names := make([]string, 0, len(meta))
	for _, item := range meta {
		names = append(names, item.Env)
	}
	return names
}

func buildFallbackEnvMeta(envs []string) []types.GameEnvItem {
	if len(envs) == 0 {
		return nil
	}
	meta := make([]types.GameEnvItem, 0, len(envs))
	for _, env := range envs {
		if trimmed := strings.TrimSpace(env); trimmed != "" {
			meta = append(meta, types.GameEnvItem{
				Env:   trimmed,
				Color: fallbackEnvColor(trimmed),
			})
		}
	}
	return meta
}

func fallbackEnvColor(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return "#13c2c2"
	case "stage", "staging":
		return "#fa8c16"
	case "test", "testing", "qa":
		return "#722ed1"
	case "dev", "development":
		return "#1677ff"
	default:
		return "#8c8c8c"
	}
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func safeAlias(game model.Game) string {
	if alias := strings.TrimSpace(game.AliasName); alias != "" {
		return alias
	}
	return strings.TrimSpace(game.Name)
}
