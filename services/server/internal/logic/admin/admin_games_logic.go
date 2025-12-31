package admin

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGamesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGamesLogic {
	return &AdminGamesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGamesLogic) AdminGames(req *types.AdminGamesRequest) (*types.AdminGamesResponse, error) {
	adminID, err := parseAdminID(req.ID)
	if err != nil {
		return nil, err
	}

	// Only super roles can read other admin scopes.
	_, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if !utils.HasAdminRole(roleNames) && !utils.HasPermissionID(permIDs, "admin:all") && !utils.HasPermissionID(permIDs, "*") && !utils.HasPermissionID(permIDs, "user:write") {
		return nil, errorx.NewForbidden("无权查看管理员游戏范围")
	}

	if l.svcCtx.DB == nil || l.svcCtx.GameModel == nil {
		return nil, errorx.NewInternalError("DB/GameModel 未初始化")
	}

	type row struct {
		GameID   uint
		GameName string
		Alias    string
		Env      string
	}
	var rows []row
	err = l.svcCtx.DB.WithContext(l.ctx).
		Table("admin_game_env_scopes").
		Select("admin_game_env_scopes.game_id as game_id, games.name as game_name, games.alias_name as alias, admin_game_env_scopes.env as env").
		Joins("INNER JOIN games ON games.id = admin_game_env_scopes.game_id").
		Where("admin_game_env_scopes.admin_id = ?", adminID).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query env scopes: %w", err)
	}

	envByGame := make(map[uint][]string)
	gameMeta := make(map[uint]types.AdminGame)
	for _, r := range rows {
		env := strings.TrimSpace(r.Env)
		if env == "" {
			continue
		}
		envByGame[r.GameID] = append(envByGame[r.GameID], env)
		if _, ok := gameMeta[r.GameID]; !ok {
			name := strings.TrimSpace(r.GameName)
			if name == "" {
				name = strings.TrimSpace(r.Alias)
			}
			gameMeta[r.GameID] = types.AdminGame{
				GameId:   r.GameName,
				GameName: name,
				Envs:     []string{},
			}
		}
	}

	// Also include game-only scopes (all envs).
	var gameScopes []model.AdminGameScope
	if err := l.svcCtx.DB.WithContext(l.ctx).Where("admin_id = ?", adminID).Find(&gameScopes).Error; err != nil {
		return nil, fmt.Errorf("query game scopes: %w", err)
	}
	for _, s := range gameScopes {
		if _, ok := gameMeta[s.GameID]; ok {
			continue
		}
		game, err := l.svcCtx.GameModel.FindOne(l.ctx, s.GameID)
		if err != nil || game == nil {
			continue
		}
		gameMeta[s.GameID] = types.AdminGame{
			GameId:   game.Name,
			GameName: strings.TrimSpace(game.AliasName),
			Envs:     []string{},
		}
	}

	items := make([]types.AdminGame, 0, len(gameMeta))
	for gid, item := range gameMeta {
		envs := uniqueStrings(envByGame[gid])
		sort.Strings(envs)
		item.Envs = envs
		if item.GameName == "" {
			item.GameName = item.GameId
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GameId < items[j].GameId })
	return &types.AdminGamesResponse{Games: items}, nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return out
}
