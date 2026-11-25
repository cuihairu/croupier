package game

import (
	"context"
	"errors"
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
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看游戏列表", "admin:all", "games:read", "games:manage", "games:write"); err != nil {
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
	// Game creation is separated from environment management: games:write
	// governs game create/update/delete, games:manage governs environments.
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权创建游戏", "admin:all", "games:write"); err != nil {
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

	// Explicit gameId: hyphens are kept as-is (matching the SDK-side
	// registration convention); when omitted, fall back to deriving it from
	// Name (BeforeCreate normalizes hyphens to underscores).
	explicitID, err := sanitizeExplicitGameID(req.GameID)
	if err != nil {
		return nil, err
	}
	if explicitID != "" {
		if _, err := s.svcCtx.GameModel.FindByGameIDString(ctx, explicitID); err == nil {
			return nil, errorx.NewConflict("game_id 已存在: " + explicitID)
		} else if !errors.Is(err, model.ErrGameNotFound) {
			return nil, err
		}
	}

	aliasName := strings.TrimSpace(req.AliasName)
	if aliasName == "" {
		// Default the display name to the game identifier: alias_name has a
		// unique index, and several empty strings would collide with each
		// other.
		aliasName = name
	}

	game := &model.Game{
		Name:        name,
		AliasName:   aliasName,
		Icon:        strings.TrimSpace(req.Icon),
		Description: strings.TrimSpace(req.Description),
		Config:      strings.TrimSpace(req.Config),
		Status:      "dev",
		Enabled:     true,
	}
	if explicitID != "" {
		game.GameID = explicitID
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
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看游戏详情", "admin:all", "games:read", "games:manage", "games:write"); err != nil {
		return nil, err
	}

	id, err := s.resolveGameID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	game, err := s.svcCtx.GetGameCached(ctx, id)
	if err != nil {
		return nil, gameLookupErr(req.ID, err)
	}

	bindings, _ := s.svcCtx.GameModel.ListEnvBindings(ctx, game.GameID)

	return &GameDetailResponse{
		Game: buildGameInfoWithBindings(game, bindings),
	}, nil
}

// Update updates an existing game
func (s *Service) Update(ctx context.Context, req *GameUpdateRequest) (*GameUpdateResponse, error) {
	// Editing the game body itself (display name, icon, etc.) goes through
	// games:write, separated from environment management permission.
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权更新游戏", "admin:all", "games:write"); err != nil {
		return nil, err
	}

	id, err := s.resolveGameID(ctx, req.ID)
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
	// Explicit gameId change: only allowed for games without bound
	// environments (renaming after binding would leave game_envs and the
	// pre-existing data routed by game_id dangling).
	newGameID, err := sanitizeExplicitGameID(req.GameID)
	if err != nil {
		return nil, err
	}
	oldGameID := ""
	if newGameID != "" {
		current, err := s.svcCtx.GameModel.FindOne(ctx, id)
		if err != nil {
			return nil, gameLookupErr(req.ID, err)
		}
		if current.GameID != newGameID {
			bindings, err := s.svcCtx.GameModel.ListEnvBindings(ctx, current.GameID)
			if err != nil {
				return nil, err
			}
			if len(bindings) > 0 {
				return nil, errorx.NewConflict("游戏已绑定环境，不能修改 game_id；请先清空该游戏的全部环境")
			}
			if envs, err := current.GetEnvs(); err == nil && len(envs) > 0 {
				return nil, errorx.NewConflict("游戏已绑定环境，不能修改 game_id；请先清空该游戏的全部环境")
			}
			if found, err := s.svcCtx.GameModel.FindByGameIDString(ctx, newGameID); err == nil && found != nil {
				return nil, errorx.NewConflict("game_id 已存在: " + newGameID)
			} else if err != nil && !errors.Is(err, model.ErrGameNotFound) {
				return nil, err
			}
			updates["game_id"] = newGameID
			oldGameID = current.GameID
		}
	}
	if v := strings.TrimSpace(req.AliasName); v != "" {
		updates["alias_name"] = v
	}
	if req.Icon != nil {
		// Pointer semantics: an explicit submission takes effect, and an empty
		// string clears the icon (rendering falls back to the default dice).
		updates["icon"] = strings.TrimSpace(*req.Icon)
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

	// After a game_id change, drop the router's cached old (gameID, env)
	// connection so subsequent requests do not keep landing on the old
	// database name.
	if oldGameID != "" && s.svcCtx.Router != nil {
		_ = s.svcCtx.Router.ForgetGame(oldGameID)
	}

	s.svcCtx.InvalidateGameCache(ctx, id)

	game, err := s.svcCtx.GetGameCached(ctx, id)
	if err != nil {
		return nil, gameLookupErr(req.ID, err)
	}

	updateBindings, _ := s.svcCtx.GameModel.ListEnvBindings(ctx, game.GameID)

	return &GameUpdateResponse{
		Game: buildGameInfoWithBindings(game, updateBindings),
	}, nil
}

// Delete deletes a game
func (s *Service) Delete(ctx context.Context, req *GameDeleteRequest) error {
	// Deleting a game belongs to games:write; games:manage (environment
	// management) cannot delete games.
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权删除游戏", "admin:all", "games:write"); err != nil {
		return err
	}

	id, err := s.resolveGameID(ctx, req.ID)
	if err != nil {
		return err
	}

	// Fetch the game before deletion to get its business GameID for cleanup.
	// If the game is already gone, FindOne returns an error — proceed with
	// Delete anyway (it is idempotent) and skip Router/binding cleanup.
	game, findErr := s.svcCtx.GameModel.FindOne(ctx, id)

	if findErr == nil && game != nil {
		// Environments must be emptied before a game can be deleted: both the
		// binding table and the Envs metadata must be 0 to proceed.
		bindings, err := s.svcCtx.GameModel.ListEnvBindings(ctx, game.GameID)
		if err != nil {
			return err
		}
		if len(bindings) > 0 {
			return errorx.NewConflict("请先删除该游戏的全部环境，再删除游戏")
		}
		if envs, err := game.GetEnvs(); err == nil && len(envs) > 0 {
			return errorx.NewConflict("请先删除该游戏的全部环境，再删除游戏")
		}

		if err := s.svcCtx.GameModel.DeleteWithEnvBindings(ctx, id, game.GameID); err != nil {
			return err
		}

		// Close and forget all cached per-game DB connections for this game.
		if s.svcCtx.Router != nil {
			_ = s.svcCtx.Router.ForgetGame(game.GameID)
		}
	} else if err := s.svcCtx.GameModel.Delete(ctx, id); err != nil {
		return err
	}

	s.svcCtx.InvalidateGameCache(ctx, id)

	return nil
}

// enrichedEnvs returns env items with databaseName merged from GameEnvBinding.
func (s *Service) enrichedEnvs(ctx context.Context, game *model.Game) []GameEnvItem {
	envs, err := game.GetEnvs()
	if err != nil {
		return []GameEnvItem{}
	}
	items := convertGameEnvs(envs)
	bindings, err := s.svcCtx.GameModel.ListEnvBindings(ctx, game.GameID)
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

	id, err := s.resolveGameID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	game, err := s.svcCtx.GetGameCached(ctx, id)
	if err != nil {
		return nil, gameLookupErr(req.ID, err)
	}

	envItems := s.enrichedEnvs(ctx, game)
	allowed, isAdmin, err := s.gameEnvScopes(ctx, id)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		envItems = filterEnvItemsByScopes(envItems, allowed)
	}

	return &GameEnvsListResponse{
		Envs: envItems,
	}, nil
}

// EnvAdd adds a new environment to a game
func (s *Service) EnvAdd(ctx context.Context, req *GameEnvAddRequest) (*GameEnvAddResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权添加游戏环境", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	id, err := s.resolveGameID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	game, err := s.svcCtx.GameModel.FindOne(ctx, id)
	if err != nil {
		return nil, gameLookupErr(req.ID, err)
	}

	newEnv, err := ensureEnvName(req.Name)
	if err != nil {
		return nil, err
	}

	// Adding an environment is a game-level operation (the target environment
	// does not exist yet), so authorize on the game dimension.
	if err := s.authorizeGameEnv(ctx, id); err != nil {
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
	// Design-debt cleanup: Game.SetEnvs only Marshals a []GameEnv of pure
	// string fields and always returns nil (already argued in the model-layer
	// comment), so the original err branch was unreachable and removed
	// (likewise below).
	_ = game.SetEnvs(envs)

	if err := s.svcCtx.GameModel.UpdateEnvsAndBindings(
		ctx,
		game.GameID,
		id,
		game.Envs,
		nil,
		[]model.GameEnvBinding{{
			Env:          newEnv,
			DatabaseName: s.deriveGameDBName(game.GameID, newEnv),
			Description:  strings.TrimSpace(req.Type),
		}},
	); err != nil {
		return nil, err
	}

	s.svcCtx.InvalidateGameCache(ctx, id)

	return &GameEnvAddResponse{
		Envs: s.enrichedEnvs(ctx, game),
	}, nil
}

// EnvUpdate updates an existing environment in a game
func (s *Service) EnvUpdate(ctx context.Context, req *GameEnvUpdateRequest) (*GameEnvUpdateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权更新游戏环境", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	id, err := s.resolveGameID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	game, err := s.svcCtx.GameModel.FindOne(ctx, id)
	if err != nil {
		return nil, gameLookupErr(req.ID, err)
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
	// In the rename case both the old and the new environment must be within
	// the grant, otherwise renaming becomes privilege escalation (renaming an
	// authorized environment to an arbitrary new name).
	if err := s.authorizeGameEnv(ctx, id, oldEnvName, target.Env); err != nil {
		return nil, err
	}
	if v := strings.TrimSpace(req.Type); v != "" {
		target.Description = v
	}
	envs[idx] = target

	// Design-debt cleanup: SetEnvs always returns nil (see the note in
	// EnvAdd).
	_ = game.SetEnvs(envs)
	newEnvName := target.Env
	currentBinding, err := s.svcCtx.GameModel.FindEnvBinding(ctx, game.GameID, oldEnvName)
	if err != nil {
		return nil, err
	}
	databaseName := s.deriveGameDBName(game.GameID, newEnvName)
	if oldEnvName == newEnvName && currentBinding != nil {
		databaseName = currentBinding.DatabaseName
	}
	removeEnvs := []string(nil)
	if oldEnvName != newEnvName {
		removeEnvs = []string{oldEnvName}
	}
	if err := s.svcCtx.GameModel.UpdateEnvsAndBindings(
		ctx,
		game.GameID,
		id,
		game.Envs,
		removeEnvs,
		[]model.GameEnvBinding{{
			Env:          newEnvName,
			DatabaseName: databaseName,
			Description:  target.Description,
			Color:        target.Color,
		}},
	); err != nil {
		return nil, err
	}

	s.svcCtx.InvalidateGameCache(ctx, id)

	return &GameEnvUpdateResponse{
		Envs: s.enrichedEnvs(ctx, game),
	}, nil
}

// EnvDelete deletes an environment from a game
func (s *Service) EnvDelete(ctx context.Context, req *GameEnvDeleteRequest) (*GameEnvDeleteResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权删除游戏环境", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	id, err := s.resolveGameID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	game, err := s.svcCtx.GameModel.FindOne(ctx, id)
	if err != nil {
		return nil, gameLookupErr(req.ID, err)
	}

	envs, err := game.GetEnvs()
	if err != nil {
		return nil, err
	}

	idx := findEnvIndex(envs, req.EnvID)
	if idx < 0 {
		return nil, errorx.NewNotFound("环境 " + req.EnvID + " 不存在")
	}

	if err := s.authorizeGameEnv(ctx, id, envs[idx].Env); err != nil {
		return nil, err
	}

	removedEnv := envs[idx].Env
	envs = append(envs[:idx], envs[idx+1:]...)
	// Design-debt cleanup: SetEnvs always returns nil (see the note in
	// EnvAdd).
	_ = game.SetEnvs(envs)
	if err := s.svcCtx.GameModel.UpdateEnvsAndBindings(
		ctx,
		game.GameID,
		id,
		game.Envs,
		[]string{removedEnv},
		nil,
	); err != nil {
		return nil, err
	}

	// Close and forget the cached per-game DB connection for this env.
	if s.svcCtx.Router != nil {
		_ = s.svcCtx.Router.Forget(game.GameID, removedEnv)
	}

	s.svcCtx.InvalidateGameCache(ctx, id)

	return &GameEnvDeleteResponse{
		Envs: s.enrichedEnvs(ctx, game),
	}, nil
}
