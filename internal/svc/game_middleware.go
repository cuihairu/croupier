package svc

import (
	"context"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/gin-gonic/gin"
	"log/slog"
)

// GameDBHeader is the canonical header carrying the game business identifier.
const GameDBHeader = "X-Game-ID"

// EnvHeader is the canonical header carrying the logical environment.
const EnvHeader = "X-Env"

// GameScopeKey is the gin context key for the resolved (gameID, env) pair.
const GameScopeKey = "gameScope"

// gameScopeCtxKey is the standard context.Context key type.
type gameScopeCtxKey struct{}

// GameScope captures the resolved game/environment scope of a request.
type GameScope struct {
	GameID string
	Env    string
}

// GameDBMiddleware resolves the per-game *gorm.DB from the request's
// X-Game-ID / X-Env headers (when the database-per-game router is enabled)
// and stores it in the request context so game-scoped models pick it up via
// dbctx.Resolve. When the router is nil (legacy single-DB mode) the middleware
// is a pass-through.
func GameDBMiddleware(svcCtx *ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		gameID := strings.TrimSpace(c.GetHeader(GameDBHeader))
		env := strings.TrimSpace(c.GetHeader(EnvHeader))

		scope := GameScope{GameID: gameID, Env: env}
		if gameID != "" {
			c.Set(GameScopeKey, scope)
		}

		// Always propagate scope via standard context so non-gin callers can
		// read it.
		ctx := context.WithValue(c.Request.Context(), gameScopeCtxKey{}, scope)

		if svcCtx != nil && svcCtx.Router != nil && gameID != "" {
			gameDB, err := svcCtx.Router.GameDB(ctx, gameID, env)
			if err != nil {
				slog.ErrorContext(ctx, "failed to resolve game database",
					"gameId", gameID, "env", env, "error", err)
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error":   "game_database_unavailable",
					"message": "无法连接到游戏数据库",
				})
				return
			}
			ctx = dbctx.WithDB(ctx, gameDB)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GameScopeFromContext extracts the (gameID, env) pair stored by
// GameDBMiddleware from a standard context. Returns empty values when absent.
func GameScopeFromContext(ctx context.Context) (gameID, env string) {
	if ctx == nil {
		return "", ""
	}
	if v, ok := ctx.Value(gameScopeCtxKey{}).(GameScope); ok {
		return v.GameID, v.Env
	}
	return "", ""
}

// WithGameScope stores a GameScope in ctx. Useful for background jobs and
// tests that need to mimic the middleware.
func WithGameScope(ctx context.Context, scope GameScope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, gameScopeCtxKey{}, scope)
}
