package svc

import (
	"context"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"gorm.io/gorm"
)

// listGameEnvScopes 枚举 game_envs 绑定的全部 (gameID, env)，供留痕
// 保留期清理逐 game 库执行。
func listGameEnvScopes(db *gorm.DB) func(ctx context.Context) ([]executionlog.GameScopeRef, error) {
	return func(ctx context.Context) ([]executionlog.GameScopeRef, error) {
		var bindings []model.GameEnvBinding
		if err := db.WithContext(ctx).Find(&bindings).Error; err != nil {
			return nil, err
		}
		scopes := make([]executionlog.GameScopeRef, 0, len(bindings))
		for _, b := range bindings {
			scopes = append(scopes, executionlog.GameScopeRef{GameID: b.GameID, Env: b.Env})
		}
		return scopes, nil
	}
}
