package admin

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"gorm.io/gorm"
)

type AdminGamesUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGamesUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGamesUpdateLogic {
	return &AdminGamesUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGamesUpdateLogic) AdminGamesUpdate(req *types.AdminGamesUpdateRequest) (*types.AdminGamesResponse, error) {
	targetAdminID, err := parseAdminID(req.ID)
	if err != nil {
		return nil, err
	}

	// Only super roles can update other admin scopes.
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
		return nil, errorx.NewForbidden("无权更新管理员游戏范围")
	}

	if l.svcCtx.DB == nil || l.svcCtx.GameModel == nil {
		return nil, errorx.NewInternalError("DB/GameModel 未初始化")
	}

	// Normalize input early.
	games := req.Games
	if games == nil {
		games = []types.AdminGame{}
	}

	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("admin_id = ?", targetAdminID).Delete(&model.AdminGameEnvScope{}).Error; err != nil {
			return err
		}
		if err := tx.Where("admin_id = ?", targetAdminID).Delete(&model.AdminGameScope{}).Error; err != nil {
			return err
		}

		for _, entry := range games {
			gameName := strings.TrimSpace(entry.GameId)
			if gameName == "" {
				continue
			}
			game, err := l.svcCtx.GameModel.FindByName(l.ctx, gameName)
			if err != nil || game == nil {
				return errorx.NewNotFound("game not found: " + gameName)
			}

			// Always insert game scope entry for quick allow (envs empty means all env).
			if err := tx.Create(&model.AdminGameScope{AdminID: targetAdminID, GameID: game.ID}).Error; err != nil {
				return err
			}

			for _, env := range entry.Envs {
				trimmed := strings.TrimSpace(env)
				if trimmed == "" {
					continue
				}
				if err := tx.Create(&model.AdminGameEnvScope{
					AdminID: targetAdminID,
					GameID:  game.ID,
					Env:     trimmed,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Return latest.
	getReq := &types.AdminGamesRequest{ID: req.ID}
	return NewAdminGamesLogic(l.ctx, l.svcCtx).AdminGames(getReq)
}
