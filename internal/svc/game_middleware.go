package svc

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/gin-gonic/gin"
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
//
// SECURITY: When the database-per-game router is enabled, this middleware
// validates that the (gameID, env) pair exists in the meta database's
// game_envs table before opening or creating any database connection.
// Requests with unknown or unauthorized game/env pairs are rejected with 403.
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
			// SECURITY: Validate that the (gameID, env) pair is registered in
			// the meta database before allowing access. This prevents:
			// 1. Unauthenticated/unknown game IDs from triggering database creation
			// 2. Environment spoofing (accessing non-existent envs)
			// 3. Database name injection via crafted headers
			if err := validateGameScope(ctx, svcCtx, gameID, env); err != nil {
				slog.WarnContext(ctx, "game scope validation failed",
					"gameId", gameID, "env", env, "error", err)
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":   "invalid_game_scope",
					"message": "游戏环境不存在或无权访问",
				})
				return
			}

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

// validateGameScope checks that the (gameID, env) pair exists in the meta
// database's game_envs table. This ensures only registered game environments
// can access the database-per-game routing.
func validateGameScope(ctx context.Context, svcCtx *ServiceContext, gameID, env string) error {
	if svcCtx.GameModel == nil {
		// If GameModel is not available, skip validation (legacy mode).
		// This maintains backward compatibility for deployments without
		// the meta database model.
		return nil
	}

	// Check if the binding exists in game_envs table.
	dbName, err := svcCtx.GameModel.LookupDatabaseName(ctx, gameID, env)
	if err != nil {
		return err
	}
	if dbName == "" {
		return errGameScopeNotFound
	}
	return nil
}

// errGameScopeNotFound is returned when the requested (gameID, env) pair
// does not exist in the meta database.
type gameScopeNotFoundError struct{}

func (e *gameScopeNotFoundError) Error() string { return "game scope not found" }

var errGameScopeNotFound = &gameScopeNotFoundError{}

// GameScopeFromContext extracts the GameScope stored by GameDBMiddleware
// from a standard context. Returns a zero-value GameScope when absent.
func GameScopeFromContext(ctx context.Context) GameScope {
	if ctx == nil {
		return GameScope{}
	}
	if v, ok := ctx.Value(gameScopeCtxKey{}).(GameScope); ok {
		return v
	}
	return GameScope{}
}

// WithGameScope stores a GameScope in ctx. Useful for background jobs and
// tests that need to mimic the middleware.
func WithGameScope(ctx context.Context, scope GameScope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, gameScopeCtxKey{}, scope)
}
