package game

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// deriveGameDBName returns the physical database name for a (gameID, env)
// pair. It uses the Router's configured naming function when available,
// falling back to the canonical DefaultGameDBName.
func (s *Service) deriveGameDBName(gameID, env string) string {
	if s.svcCtx != nil && s.svcCtx.Router != nil {
		return s.svcCtx.Router.NameForGame(gameID, env)
	}
	return router.DefaultGameDBName(gameID, env)
}

// List retrieves a paginated list of games
func (s *Service) List(ctx context.Context, req *GamesListRequest) (*GamesListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看游戏列表", "admin:all", "games:read", "games:manage"); err != nil {
		return nil, err
	}

	opts := model.ListGamesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Status:   strings.TrimSpace(req.Status),
	}

	games, total, err := s.svcCtx.GameModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]GameInfo, 0, len(games))
	for i := range games {
		items = append(items, buildGameInfo(&games[i]))
	}

	return &GamesListResponse{
		Games: items,
		Total: int(total),
		Page:  req.Page,
		Size:  req.PageSize,
	}, nil
}

// Create creates a new game
func (s *Service) Create(ctx context.Context, req *GameCreateRequest) (*GameCreateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权创建游戏", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	name, err := sanitizeGameName(req.Name)
	if err != nil {
		return nil, err
	}
	exists, err := s.svcCtx.GameModel.ExistsByNameIgnoreCase(ctx, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errorx.NewConflict("game_id 已存在: " + name)
	}

	game := &model.Game{
		Name:        name,
		AliasName:   strings.TrimSpace(req.AliasName),
		Description: strings.TrimSpace(req.Description),
		Config:      strings.TrimSpace(req.Config),
		Status:      "dev",
		Enabled:     true,
	}

	if err := s.svcCtx.GameModel.Create(ctx, game); err != nil {
		return nil, err
	}

	s.svcCtx.InvalidateGameCache(ctx, game.ID)

	return &GameCreateResponse{
		Game: buildGameInfo(game),
	}, nil
}

// Detail retrieves details of a specific game
func (s *Service) Detail(ctx context.Context, req *GameDetailRequest) (*GameDetailResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看游戏详情", "admin:all", "games:read", "games:manage"); err != nil {
		return nil, err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	game, err := s.svcCtx.GetGameCached(ctx, id)
	if err != nil {
		return nil, err
	}

	bindings, _ := s.svcCtx.GameModel.ListEnvBindings(ctx, game.GameID)

	return &GameDetailResponse{
		Game: buildGameInfoWithBindings(game, bindings),
	}, nil
}

// Update updates an existing game
func (s *Service) Update(ctx context.Context, req *GameUpdateRequest) (*GameUpdateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权更新游戏", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})
	if v := strings.TrimSpace(req.Name); v != "" {
		name, err := sanitizeGameName(v)
		if err != nil {
			return nil, err
		}
		exists, err := s.svcCtx.GameModel.ExistsByNameIgnoreCase(ctx, name, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errorx.NewConflict("game_id 已存在: " + name)
		}
		updates["name"] = name
	}
	if v := strings.TrimSpace(req.AliasName); v != "" {
		updates["alias_name"] = v
	}
	if v := strings.TrimSpace(req.Description); v != "" {
		updates["description"] = v
	}
	if v := strings.TrimSpace(req.Config); v != "" {
		updates["config"] = v
	}
	if v, err := sanitizeStatus(req.Status); err != nil {
		return nil, err
	} else if v != "" {
		updates["status"] = v
	}

	if len(updates) == 0 {
		return nil, errorx.NewBadRequest("请提供需要更新的字段")
	}

	if err := s.svcCtx.GameModel.Update(ctx, id, updates); err != nil {
		return nil, err
	}

	s.svcCtx.InvalidateGameCache(ctx, id)

	game, err := s.svcCtx.GetGameCached(ctx, id)
	if err != nil {
		return nil, err
	}

	updateBindings, _ := s.svcCtx.GameModel.ListEnvBindings(ctx, game.GameID)

	return &GameUpdateResponse{
		Game: buildGameInfoWithBindings(game, updateBindings),
	}, nil
}

// Delete deletes a game
func (s *Service) Delete(ctx context.Context, req *GameDeleteRequest) error {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权删除游戏", "admin:all", "games:manage"); err != nil {
		return err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return err
	}

	// Fetch the game before deletion to get its business GameID for cleanup.
	// If the game is already gone, FindOne returns an error — proceed with
	// Delete anyway (it is idempotent) and skip Router/binding cleanup.
	game, findErr := s.svcCtx.GameModel.FindOne(ctx, id)

	if err := s.svcCtx.GameModel.Delete(ctx, id); err != nil {
		return err
	}

	if findErr == nil && game != nil {
		// Clean up all GameEnvBinding records for this game.
		if envs, err := game.GetEnvs(); err == nil {
			for _, env := range envs {
				_ = s.svcCtx.GameModel.RemoveEnvBinding(ctx, game.GameID, env.Env)
			}
		}

		// Close and forget all cached per-game DB connections for this game.
		if s.svcCtx.Router != nil {
			_ = s.svcCtx.Router.ForgetGame(game.GameID)
		}
	}

	s.svcCtx.InvalidateGameCache(ctx, id)

	return nil
}

// enrichedEnvs returns env items with databaseName merged from GameEnvBinding.
func (s *Service) enrichedEnvs(game *model.Game) []GameEnvItem {
	envs, err := game.GetEnvs()
	if err != nil {
		return []GameEnvItem{}
	}
	items := convertGameEnvs(envs)
	bindings, err := s.svcCtx.GameModel.ListEnvBindings(context.Background(), game.GameID)
	if err != nil {
		return items
	}
	return mergeBindingData(items, bindings)
}

// EnvsList retrieves the environments for a game
func (s *Service) EnvsList(ctx context.Context, req *GameEnvsListRequest) (*GameEnvsListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看游戏环境列表", "admin:all", "games:read", "games:manage"); err != nil {
		return nil, err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	game, err := s.svcCtx.GetGameCached(ctx, id)
	if err != nil {
		return nil, err
	}

	return &GameEnvsListResponse{
		Envs: s.enrichedEnvs(game),
	}, nil
}

// EnvAdd adds a new environment to a game
func (s *Service) EnvAdd(ctx context.Context, req *GameEnvAddRequest) (*GameEnvAddResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权添加游戏环境", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	game, err := s.svcCtx.GameModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	newEnv, err := ensureEnvName(req.Name)
	if err != nil {
		return nil, err
	}

	envs, err := game.GetEnvs()
	if err != nil {
		return nil, err
	}
	if findEnvIndex(envs, newEnv) >= 0 {
		return nil, errorx.NewConflict("环境 " + newEnv + " 已存在")
	}

	envs = append(envs, model.GameEnv{
		Env:         newEnv,
		Description: strings.TrimSpace(req.Type),
	})
	if err := game.SetEnvs(envs); err != nil {
		return nil, err
	}

	if err := s.svcCtx.GameModel.Update(ctx, id, map[string]interface{}{"envs": game.Envs}); err != nil {
		return nil, err
	}

	// Sync GameEnvBinding so the database-per-game router can resolve this
	// (gameID, env) to the correct physical database.
	dbName := s.deriveGameDBName(game.GameID, newEnv)
	if err := s.svcCtx.GameModel.AddEnvBinding(ctx, game.GameID, newEnv, dbName, strings.TrimSpace(req.Type), ""); err != nil {
		// Best-effort: log but don't fail the env-add — the JSON envs list is
		// already updated and is the primary UI source of truth.
		_ = err
	}

	s.svcCtx.InvalidateGameCache(ctx, id)

	return &GameEnvAddResponse{
		Envs: s.enrichedEnvs(game),
	}, nil
}

// EnvUpdate updates an existing environment in a game
func (s *Service) EnvUpdate(ctx context.Context, req *GameEnvUpdateRequest) (*GameEnvUpdateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权更新游戏环境", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	game, err := s.svcCtx.GameModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	envs, err := game.GetEnvs()
	if err != nil {
		return nil, err
	}

	idx := findEnvIndex(envs, req.EnvID)
	if idx < 0 {
		return nil, errorx.NewNotFound("环境 " + req.EnvID + " 不存在")
	}

	target := envs[idx]
	oldEnvName := target.Env
	if newName := strings.TrimSpace(req.Name); newName != "" {
		if other := findEnvIndex(envs, newName); other >= 0 && other != idx {
			return nil, errorx.NewConflict("环境 " + newName + " 已存在")
		}
		target.Env = newName
	}
	if v := strings.TrimSpace(req.Type); v != "" {
		target.Description = v
	}
	envs[idx] = target

	if err := game.SetEnvs(envs); err != nil {
		return nil, err
	}
	if err := s.svcCtx.GameModel.Update(ctx, id, map[string]interface{}{"envs": game.Envs}); err != nil {
		return nil, err
	}

	// Sync GameEnvBinding: if the env name changed, remove the old binding and
	// create a new one with the updated name.
	newEnvName := target.Env
	if oldEnvName != newEnvName || strings.TrimSpace(req.Type) != "" {
		_ = s.svcCtx.GameModel.RemoveEnvBinding(ctx, game.GameID, oldEnvName)
		dbName := s.deriveGameDBName(game.GameID, newEnvName)
		_ = s.svcCtx.GameModel.AddEnvBinding(ctx, game.GameID, newEnvName, dbName, target.Description, target.Color)
	}

	s.svcCtx.InvalidateGameCache(ctx, id)

	return &GameEnvUpdateResponse{
		Envs: s.enrichedEnvs(game),
	}, nil
}

// EnvDelete deletes an environment from a game
func (s *Service) EnvDelete(ctx context.Context, req *GameEnvDeleteRequest) (*GameEnvDeleteResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权删除游戏环境", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	game, err := s.svcCtx.GameModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	envs, err := game.GetEnvs()
	if err != nil {
		return nil, err
	}

	idx := findEnvIndex(envs, req.EnvID)
	if idx < 0 {
		return nil, errorx.NewNotFound("环境 " + req.EnvID + " 不存在")
	}

	removedEnv := envs[idx].Env
	envs = append(envs[:idx], envs[idx+1:]...)
	if err := game.SetEnvs(envs); err != nil {
		return nil, err
	}
	if err := s.svcCtx.GameModel.Update(ctx, id, map[string]interface{}{"envs": game.Envs}); err != nil {
		return nil, err
	}

	// Remove the corresponding GameEnvBinding so the router no longer routes
	// to the deleted environment's database.
	_ = s.svcCtx.GameModel.RemoveEnvBinding(ctx, game.GameID, removedEnv)

	// Close and forget the cached per-game DB connection for this env.
	if s.svcCtx.Router != nil {
		_ = s.svcCtx.Router.Forget(game.GameID, removedEnv)
	}

	s.svcCtx.InvalidateGameCache(ctx, id)

	return &GameEnvDeleteResponse{
		Envs: s.enrichedEnvs(game),
	}, nil
}
