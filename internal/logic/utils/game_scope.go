package utils

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

// RequireGameEnvScope enforces that the current admin is allowed to operate on the given game/env.
// Non-admin users must provide gameID; env is strongly recommended and enforced when env scopes exist.
func RequireGameEnvScope(ctx context.Context, svcCtx *svc.ServiceContext, adminID uint, roleNames []string, gameID string, env string) error {
	if HasAdminRole(roleNames) {
		return nil
	}

	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if gameID == "" {
		return errorx.NewBadRequest("game_id is required")
	}
	if env == "" {
		return errorx.NewBadRequest("env is required")
	}
	if svcCtx == nil || svcCtx.GameModel == nil || svcCtx.PermissionService == nil || svcCtx.DB == nil {
		return errorx.NewInternalError("scope checker not initialized")
	}

	game, err := svcCtx.GameModel.FindByName(ctx, gameID)
	if err != nil || game == nil {
		return errorx.NewNotFound("game not found")
	}

	// If the admin has any env scopes for this game, we enforce env-level checks.
	var envScopeCount int64
	if err := svcCtx.DB.WithContext(ctx).
		Table("admin_game_env_scopes").
		Where("admin_id = ? AND game_id = ?", adminID, game.ID).
		Count(&envScopeCount).Error; err != nil {
		return errorx.NewInternalError("check env scope failed")
	}

	if envScopeCount > 0 {
		ok, err := svcCtx.PermissionService.CheckGameEnvScope(ctx, adminID, game.ID, env)
		if err != nil {
			return err
		}
		if !ok {
			return errorx.NewForbidden("无权访问该环境")
		}
		return nil
	}

	// Otherwise, fall back to game scope.
	ok, err := svcCtx.PermissionService.CheckGameScope(ctx, adminID, game.ID)
	if err != nil {
		return err
	}
	if !ok {
		return errorx.NewForbidden("无权访问该游戏")
	}
	return nil
}
