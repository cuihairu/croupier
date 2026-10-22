package game

import (
	"context"
	"errors"
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

// resolveGameID 兼容两种寻址形态：数字主键（历史形态）与业务 game_id（=Name，
// BeforeCreate 自动填充）。前端环境页自授权视图（/profile/games）驱动后不再
// 持有数字主键，环境端点因此接受业务串寻址。
func (s *Service) resolveGameID(ctx context.Context, raw string) (uint, error) {
	if id, err := parseGameID(raw); err == nil {
		return id, nil
	}
	game, err := s.svcCtx.GameModel.FindByGameIDString(ctx, strings.TrimSpace(raw))
	if errors.Is(err, model.ErrGameNotFound) {
		return 0, errorx.NewNotFound("游戏 " + strings.TrimSpace(raw) + " 不存在")
	}
	if err != nil {
		// 真实存储错误按原样上抛，禁止吞成 404。
		return 0, err
	}
	return game.ID, nil
}

// gameLookupErr 把模型层"游戏不存在"翻译成 404 契约错误。数字主键寻址
// 不做存在性预检（resolveGameID 对数字直接放行），载入失败时在这里统一
// 收口；其余错误（数据库故障等）原样上抛，禁止吞成 404。
func gameLookupErr(rawID string, err error) error {
	if errors.Is(err, model.ErrGameNotFound) {
		return errorx.NewNotFound("游戏 " + rawID + " 不存在")
	}
	return err
}

// gameEnvScopes 返回当前操作者在指定游戏上的授权环境集合。admin 角色直过
// （isAdmin=true，allowed 为 nil 表示不限制）；其余操作者按 admin_game_env_scopes
// 授权表过滤——RBAC 功能权限点（games:manage）只表达「能否管理游戏环境」，
// 该表表达「在哪些 (game, env) 上被授权」，环境写路径两道防线都必须过。
func (s *Service) gameEnvScopes(ctx context.Context, gameID uint) (map[string]struct{}, bool, error) {
	admin, roles, err := utils.LoadCurrentAdmin(ctx, s.svcCtx)
	if err != nil {
		return nil, false, err
	}
	if utils.HasAdminRole(utils.RoleNamesFromModels(roles)) {
		return nil, true, nil
	}
	scopes, err := s.svcCtx.AdminModel.GetAdminEnvScopes(ctx, admin.ID)
	if err != nil {
		return nil, false, errorx.NewInternalError("查询游戏环境授权失败")
	}
	allowed := make(map[string]struct{}, len(scopes))
	for _, sc := range scopes {
		if sc.GameID == gameID {
			allowed[strings.ToLower(strings.TrimSpace(sc.Env))] = struct{}{}
		}
	}
	return allowed, false, nil
}

// authorizeGameEnv 是环境写路径的授权防线：目标环境必须命中授权集合。
// envs 变参为空表示仅校验游戏维度（EnvAdd 场景——新增环境尚不存在，
// 不在授权表内，只要操作者在该游戏持有任一环境授权即可）。
func (s *Service) authorizeGameEnv(ctx context.Context, gameID uint, envs ...string) error {
	allowed, isAdmin, err := s.gameEnvScopes(ctx, gameID)
	if err != nil {
		return err
	}
	if isAdmin {
		return nil
	}
	if len(allowed) == 0 {
		return errorx.NewForbidden("无权操作该游戏的环境")
	}
	for _, env := range envs {
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(env))]; !ok {
			return errorx.NewForbidden("无权操作游戏环境 " + env)
		}
	}
	return nil
}

// filterEnvItemsByScopes 按授权环境集合过滤列表项（小写对齐 findEnvIndex 的
// EqualFold 语义）。
func filterEnvItemsByScopes(items []GameEnvItem, allowed map[string]struct{}) []GameEnvItem {
	filtered := make([]GameEnvItem, 0, len(items))
	for _, item := range items {
		if _, ok := allowed[strings.ToLower(item.Env)]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered
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
