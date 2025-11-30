package permission

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	tokenmgr "github.com/cuihairu/croupier/internal/security/token"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"gorm.io/gorm"
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
func PermissionMiddleware(permissionService *PermissionService, jwtSecret string, config PermissionConfig) func(http.Handler) http.Handler {
	var tokenManager *tokenmgr.Manager
	if strings.TrimSpace(jwtSecret) != "" {
		tokenManager = tokenmgr.NewManager(jwtSecret)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get admin ID from context (set by auth middleware) or JWT token
			adminID, err := extractAdminID(ctx, r, tokenManager, permissionService)
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
				logx.Infof("Permission denied admin=%d resource=%s action=%s",
					adminID, config.Resource, config.Action)
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
					logx.Infof("Game scope permission denied admin=%d game=%d", adminID, gameID)
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
					logx.Infof("Env scope permission denied admin=%d game=%d env=%s", adminID, gameID, env)
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
func extractAdminID(ctx context.Context, r *http.Request, tokenManager *tokenmgr.Manager, permSvc *PermissionService) (uint, error) {
	if ctx != nil {
		if v := ctx.Value("adminID"); v != nil {
			if id, ok := v.(uint); ok && id > 0 {
				return id, nil
			}
			if id64, ok := v.(int64); ok && id64 > 0 {
				return uint(id64), nil
			}
		}
	}

	// Try to get from Authorization header (JWT token)
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" && tokenManager != nil {
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) == 2 && strings.EqualFold(tokenParts[0], "Bearer") {
			subject, _, err := tokenManager.Verify(tokenParts[1])
			if err != nil {
				return 0, fmt.Errorf("invalid token: %w", err)
			}
			if subject == "" {
				return 0, errors.New("token subject missing")
			}

			admin, err := permSvc.lookupAdminByUsername(subject)
			if err != nil {
				return 0, err
			}
			return admin.ID, nil
		}
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

func (s *PermissionService) lookupAdminByUsername(username string) (*model.Admin, error) {
	if strings.TrimSpace(username) == "" {
		return nil, errors.New("username missing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var admin model.Admin
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to lookup admin: %w", err)
	}
	return &admin, nil
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
		Resource:       "player",
		Action:         "create",
		CheckGameScope: true,
	}
}

func FunctionExecutePermission() PermissionConfig {
	return PermissionConfig{
		Resource:       "function",
		Action:         "execute",
		CheckGameScope: true,
		CheckEnvScope:  true,
	}
}
