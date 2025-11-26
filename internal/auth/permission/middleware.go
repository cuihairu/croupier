package permission

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// PermissionConfig defines configuration for permission middleware
type PermissionConfig struct {
	Resource string
	Action   string
	// Optional game scope check
	CheckGameScope bool
	// Optional env scope check
	CheckEnvScope bool
}

// PermissionMiddleware creates a permission checking middleware
func PermissionMiddleware(permissionService *PermissionService, config PermissionConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get admin ID from JWT token or session
			adminID, err := extractAdminID(r)
			if err != nil {
				logx.Error("Failed to extract admin ID", logx.Field("error", err))
				httpx.ErrorCtx(ctx, w, errors.New("unauthorized"))
				return
			}

			// Check basic permission
			hasPermission, err := permissionService.CheckPermission(ctx, adminID, config.Resource, config.Action)
			if err != nil {
				logx.Error("Permission check failed", logx.Field("error", err))
				httpx.ErrorCtx(ctx, w, err)
				return
			}

			if !hasPermission {
				logx.Warn("Permission denied",
					logx.Field("adminID", adminID),
					logx.Field("resource", config.Resource),
					logx.Field("action", config.Action))
				httpx.ErrorCtx(ctx, w, ErrPermissionDenied)
				return
			}

			// Check game scope if required
			if config.CheckGameScope {
				gameID, err := extractGameID(r)
				if err != nil {
					logx.Error("Failed to extract game ID", logx.Field("error", err))
					httpx.ErrorCtx(ctx, w, errors.New("game ID required"))
					return
				}

				hasGameScope, err := permissionService.CheckGameScope(ctx, adminID, gameID)
				if err != nil {
					logx.Error("Game scope check failed", logx.Field("error", err))
					httpx.ErrorCtx(ctx, w, err)
					return
				}

				if !hasGameScope {
					logx.Warn("Game scope permission denied",
						logx.Field("adminID", adminID),
						logx.Field("gameID", gameID))
					httpx.ErrorCtx(ctx, w, ErrPermissionDenied)
					return
				}
			}

			// Check env scope if required
			if config.CheckEnvScope {
				gameID, err := extractGameID(r)
				if err != nil {
					httpx.ErrorCtx(ctx, w, errors.New("game ID required"))
					return
				}

				env, err := extractEnv(r)
				if err != nil {
					httpx.ErrorCtx(ctx, w, errors.New("env required"))
					return
				}

				hasEnvScope, err := permissionService.CheckGameEnvScope(ctx, adminID, gameID, env)
				if err != nil {
					logx.Error("Env scope check failed", logx.Field("error", err))
					httpx.ErrorCtx(ctx, w, err)
					return
				}

				if !hasEnvScope {
					logx.Warn("Env scope permission denied",
						logx.Field("adminID", adminID),
						logx.Field("gameID", gameID),
						logx.Field("env", env))
					httpx.ErrorCtx(ctx, w, ErrPermissionDenied)
					return
				}
			}

			// Store admin info in context for later use
			ctx = context.WithValue(ctx, "adminID", adminID)
			ctx = context.WithValue(ctx, "resource", config.Resource)
			ctx = context.WithValue(ctx, "action", config.Action)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Helper functions to extract data from request
func extractAdminID(r *http.Request) (uint, error) {
	// Try to get from Authorization header (JWT token)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// TODO: Parse JWT token and extract admin ID
		// For now, return a placeholder
		return 1, nil
	}

	// Try to get from cookie or session header
	adminIDStr := r.Header.Get("X-Admin-ID")
	if adminIDStr == "" {
		return 0, errors.New("admin ID not found in request")
	}

	adminID, err := strconv.ParseUint(adminIDStr, 10, 32)
	if err != nil {
		return 0, errors.New("invalid admin ID format")
	}

	return uint(adminID), nil
}

func extractGameID(r *http.Request) (uint, error) {
	// Try to get from path parameter
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) >= 3 {
		if pathParts[1] == "games" {
			gameIDStr := pathParts[2]
			gameID, err := strconv.ParseUint(gameIDStr, 10, 32)
			if err == nil {
				return uint(gameID), nil
			}
		}
	}

	// Try to get from query parameter
	gameIDStr := r.URL.Query().Get("gameId")
	if gameIDStr == "" {
		gameIDStr = r.URL.Query().Get("game_id")
	}

	if gameIDStr != "" {
		gameID, err := strconv.ParseUint(gameIDStr, 10, 32)
		if err == nil {
			return uint(gameID), nil
		}
	}

	// Try to get from header
	gameIDStr = r.Header.Get("X-Game-ID")
	if gameIDStr != "" {
		gameID, err := strconv.ParseUint(gameIDStr, 10, 32)
		if err == nil {
			return uint(gameID), nil
		}
	}

	return 0, errors.New("game ID not found in request")
}

func extractEnv(r *http.Request) (string, error) {
	// Try to get from query parameter
	env := r.URL.Query().Get("env")
	if env != "" {
		return env, nil
	}

	// Try to get from header
	env = r.Header.Get("X-Env")
	if env != "" {
		return env, nil
	}

	return "", errors.New("env not found in request")
}

// Predefined permission configurations for common use cases
func AdminReadPermission() PermissionConfig {
	return PermissionConfig{
		Resource: "admin",
		Action:   "read",
	}
}

func AdminCreatePermission() PermissionConfig {
	return PermissionConfig{
		Resource: "admin",
		Action:   "create",
	}
}

func AdminUpdatePermission() PermissionConfig {
	return PermissionConfig{
		Resource: "admin",
		Action:   "update",
	}
}

func AdminDeletePermission() PermissionConfig {
	return PermissionConfig{
		Resource: "admin",
		Action:   "delete",
	}
}

func RoleManagePermission() PermissionConfig {
	return PermissionConfig{
		Resource: "role",
		Action:   "create", // Covers all role operations
	}
}

func GameManagePermission() PermissionConfig {
	return PermissionConfig{
		Resource: "game",
		Action:   "create",
	}
}

func GameReadPermission() PermissionConfig {
	return PermissionConfig{
		Resource: "game",
		Action:   "read",
	}
}

func PlayerManagePermission() PermissionConfig {
	return PermissionConfig{
		Resource:      "player",
		Action:        "create",
		CheckGameScope: true,
	}
}

func FunctionExecutePermission() PermissionConfig {
	return PermissionConfig{
		Resource:      "function",
		Action:        "execute",
		CheckGameScope: true,
		CheckEnvScope: true,
	}
}