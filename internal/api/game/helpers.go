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

// explicitGameIDPattern is aligned with the semantics of svc.sanitizeGameID:
// lowercase letters, digits, underscores and hyphens. Hyphens are preserved
// as-is (consistent with the gameId registered on the SDK side).
var explicitGameIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

func parseGameID(id string) (uint, error) {
	return utils.ParseUintID(id, "游戏ID")
}

// resolveGameID accepts both addressing forms: the numeric primary key (the
// legacy form) and the business game_id (= Name, auto-filled by BeforeCreate).
// After the frontend environment page moved to the self-authorized view
// (/profile/games) it no longer holds numeric primary keys, so the
// environment endpoints accept business-string addressing.
func (s *Service) resolveGameID(ctx context.Context, raw string) (uint, error) {
	if id, err := parseGameID(raw); err == nil {
		return id, nil
	}
	game, err := s.svcCtx.GameModel.FindByGameIDString(ctx, strings.TrimSpace(raw))
	if errors.Is(err, model.ErrGameNotFound) {
		return 0, errorx.NewNotFound("游戏 " + strings.TrimSpace(raw) + " 不存在")
	}
	if err != nil {
		// A genuine storage error is propagated as-is; never swallow it into
		// a 404.
		return 0, err
	}
	return game.ID, nil
}

// gameLookupErr translates the model-layer "game not found" into a 404
// contract error. Numeric primary-key addressing performs no existence
// pre-check (resolveGameID lets numbers pass through directly), so load
// failures are consolidated here; other errors (database failures, etc.)
// are propagated as-is and never swallowed into a 404.
func gameLookupErr(rawID string, err error) error {
	if errors.Is(err, model.ErrGameNotFound) {
		return errorx.NewNotFound("游戏 " + rawID + " 不存在")
	}
	return err
}

// gameEnvScopes returns the set of environments the current operator is
// authorized for on the given game. The admin role passes directly
// (isAdmin=true, with allowed nil meaning unrestricted); all other operators
// are filtered against the admin_game_env_scopes authorization table -- the
// RBAC functional permission point (games:manage) only expresses "may manage
// game environments", while this table expresses "authorized on which
// (game, env) pairs"; both lines of defense must pass on the environment
// write path.
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

// authorizeGameEnv is the authorization gate on the environment write path:
// the target environment must hit the authorized set. An empty envs variadic
// means only the game dimension is validated (the EnvAdd case -- a new
// environment does not exist yet and is not in the authorization table, so
// it suffices that the operator holds any environment authorization on that
// game).
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

// filterEnvItemsByScopes filters list items by the authorized environment set
// (lowercased, matching findEnvIndex's EqualFold semantics).
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

// sanitizeExplicitGameID normalizes an explicitly submitted gameId: empty
// input returns the empty string (not submitted), otherwise it lowercases and
// folds spaces into underscores per sanitizeGameID semantics, and constrains
// both the charset and length (the games.game_id column is size:64).
// Hyphens are allowed (demo-game).
func sanitizeExplicitGameID(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	normalized := strings.ReplaceAll(strings.ToLower(trimmed), " ", "_")
	if !explicitGameIDPattern.MatchString(normalized) {
		return "", errorx.NewBadRequest("游戏 ID 仅支持小写字母、数字和 _ -（连字符可用）")
	}
	if len(normalized) > 64 {
		return "", errorx.NewBadRequest("游戏 ID 长度不能超过 64 个字符")
	}
	return normalized, nil
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
