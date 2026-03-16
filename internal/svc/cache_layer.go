package svc

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
)

func (s *ServiceContext) cachedFetch(ctx context.Context, key string, dest interface{}, loader func() (interface{}, error)) error {
	if s != nil && s.CacheHelper != nil && key != "" {
		return s.CacheHelper.Remember(ctx, key, 0, dest, loader)
	}

	value, err := loader()
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, dest)
}

// -------- Admin helpers --------

func (s *ServiceContext) GetAdminCached(ctx context.Context, adminID uint) (*model.Admin, error) {
	var admin model.Admin
	if err := s.cachedFetch(ctx, cache.AdminIDCacheKey(adminID), &admin, func() (interface{}, error) {
		return s.AdminModel.FindOne(ctx, adminID)
	}); err != nil {
		return nil, err
	}
	s.cacheAdminAliases(ctx, &admin)
	return &admin, nil
}

func (s *ServiceContext) GetAdminByUsernameCached(ctx context.Context, username string) (*model.Admin, error) {
	normalized := strings.ToLower(strings.TrimSpace(username))
	if normalized == "" {
		return nil, nil
	}

	var admin model.Admin
	if err := s.cachedFetch(ctx, cache.AdminCacheKey(normalized), &admin, func() (interface{}, error) {
		return s.AdminModel.FindByUsername(ctx, normalized)
	}); err != nil {
		return nil, err
	}
	s.cacheAdminAliases(ctx, &admin)
	return &admin, nil
}

func (s *ServiceContext) GetAdminRolesCached(ctx context.Context, adminID uint) ([]model.Role, error) {
	var roles []model.Role
	if err := s.cachedFetch(ctx, cache.AdminRolesCacheKey(adminID), &roles, func() (interface{}, error) {
		list, err := s.AdminModel.GetAdminRoles(ctx, adminID)
		if err != nil {
			return nil, err
		}
		return list, nil
	}); err != nil {
		return nil, err
	}
	return roles, nil
}

func (s *ServiceContext) InvalidateAdminCache(ctx context.Context, adminID uint, username string) {
	s.deleteCacheKey(ctx, cache.AdminIDCacheKey(adminID))
	if trimmed := strings.ToLower(strings.TrimSpace(username)); trimmed != "" {
		s.deleteCacheKey(ctx, cache.AdminCacheKey(trimmed))
	}
	s.InvalidateAdminRolesCache(ctx, adminID)
}

func (s *ServiceContext) InvalidateAdminRolesCache(ctx context.Context, adminID uint) {
	s.deleteCacheKey(ctx, cache.AdminRolesCacheKey(adminID))
}

func (s *ServiceContext) cacheAdminAliases(ctx context.Context, admin *model.Admin) {
	if admin == nil || s == nil || s.CacheHelper == nil {
		return
	}

	if admin.ID != 0 {
		if err := s.CacheHelper.SetJSON(ctx, cache.AdminIDCacheKey(admin.ID), admin, 0); err != nil {
			slog.ErrorContext(ctx, "failed to cache admin by id", "error", err)
		}
	}

	if username := strings.ToLower(strings.TrimSpace(admin.Username)); username != "" {
		if err := s.CacheHelper.SetJSON(ctx, cache.AdminCacheKey(username), admin, 0); err != nil {
			slog.ErrorContext(ctx, "failed to cache admin by username", "error", err)
		}
	}
}

// -------- Role helpers --------

func (s *ServiceContext) GetRoleCached(ctx context.Context, roleID uint) (*model.Role, error) {
	var role model.Role
	if err := s.cachedFetch(ctx, cache.RoleCacheKey(roleID), &role, func() (interface{}, error) {
		return s.RoleModel.FindOne(ctx, roleID)
	}); err != nil {
		return nil, err
	}
	return &role, nil
}

func (s *ServiceContext) GetRolePermissionIDsCached(ctx context.Context, roleID uint) ([]string, error) {
	var ids []string
	if err := s.cachedFetch(ctx, cache.RolePermissionsCacheKey(roleID), &ids, func() (interface{}, error) {
		values, err := s.RoleModel.GetRolePermissionIDs(ctx, roleID)
		if err != nil {
			return nil, err
		}
		return values, nil
	}); err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *ServiceContext) InvalidateRoleCache(ctx context.Context, roleID uint) {
	s.deleteCacheKey(ctx, cache.RoleCacheKey(roleID))
	s.deleteCacheKey(ctx, cache.RolePermissionsCacheKey(roleID))
}

// -------- Permission helpers --------

func (s *ServiceContext) GetPermissionCached(ctx context.Context, permissionID string) (*model.Permission, error) {
	normalized := strings.ToLower(strings.TrimSpace(permissionID))
	if normalized == "" {
		return nil, nil
	}

	var perm model.Permission
	if err := s.cachedFetch(ctx, cache.PermissionCacheKey(normalized), &perm, func() (interface{}, error) {
		return s.PermissionModel.FindOne(ctx, permissionID)
	}); err != nil {
		return nil, err
	}
	return &perm, nil
}

func (s *ServiceContext) InvalidatePermissionCache(ctx context.Context, permissionID string) {
	normalized := strings.ToLower(strings.TrimSpace(permissionID))
	if normalized == "" {
		return
	}
	s.deleteCacheKey(ctx, cache.PermissionCacheKey(normalized))
}

// -------- Game helpers --------

func (s *ServiceContext) GetGameCached(ctx context.Context, gameID uint) (*model.Game, error) {
	var game model.Game
	if err := s.cachedFetch(ctx, cache.GameCacheKey(gameID), &game, func() (interface{}, error) {
		return s.GameModel.FindOne(ctx, gameID)
	}); err != nil {
		return nil, err
	}
	return &game, nil
}

func (s *ServiceContext) ListAllGamesCached(ctx context.Context) ([]model.Game, error) {
	var games []model.Game
	if err := s.cachedFetch(ctx, cache.GamesCacheKey(), &games, func() (interface{}, error) {
		items, err := s.GameModel.ListAll(ctx)
		if err != nil {
			return nil, err
		}
		return items, nil
	}); err != nil {
		return nil, err
	}
	return games, nil
}

func (s *ServiceContext) InvalidateGameCache(ctx context.Context, gameID uint) {
	s.deleteCacheKey(ctx, cache.GameCacheKey(gameID))
	s.deleteCacheKey(ctx, cache.GamesCacheKey())
}

// -------- shared helpers --------

func (s *ServiceContext) deleteCacheKey(ctx context.Context, key string) {
	if s == nil || s.Cache == nil || key == "" {
		return
	}
	if err := s.Cache.Delete(ctx, key); err != nil {
		slog.ErrorContext(ctx, "failed to delete cache key", "key", key, "error", err)
	}
}
