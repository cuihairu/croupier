package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/platform/vault"
	"gorm.io/gorm"
)

func seedBootstrapAdmins(ctx *ServiceContext) error {
	if ctx == nil || ctx.AdminManager == nil || ctx.AdminModel == nil {
		slog.Default().Warn("seedBootstrapAdmins: missing dependencies", "ctx", ctx == nil, "adminManager", ctx == nil || ctx.AdminManager == nil, "adminModel", ctx == nil || ctx.AdminModel == nil)
		return nil
	}

	admins := ctx.AdminManager.ListAdmins()
	slog.Default().Info("seedBootstrapAdmins: starting", "adminCount", len(admins))
	if len(admins) == 0 {
		slog.Default().Warn("seedBootstrapAdmins: no admins to seed from AdminManager")
		return nil
	}

	bg := context.Background()
	for _, admin := range admins {
		username := strings.TrimSpace(admin.Username)
		if username == "" || strings.TrimSpace(admin.Password) == "" {
			continue
		}
		// Warn about plaintext passwords in bootstrap config
		if !admin.IsHashedPassword() {
			slog.Default().Warn("Bootstrap admin uses plaintext password - consider using bcrypt hash", "username", username)
		}

		bootstrapStatus := 1
		if admin.Status == 1 {
			bootstrapStatus = 1
		}

		var dbAdmin model.Admin
		err := ctx.DB.WithContext(bg).
			Where("username = ?", username).
			First(&dbAdmin).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			newAdmin := &model.Admin{
				Username: username,
				Nickname: strings.TrimSpace(admin.Nickname),
				Email:    strings.TrimSpace(admin.Email),
				Phone:    strings.TrimSpace(admin.Phone),
				Status:   bootstrapStatus,
			}

			if err := ctx.AdminModel.Create(bg, newAdmin, admin.Password); err != nil {
				slog.Default().Error("创建引导管理员失败", "username", username, "error", err)
				continue
			}

			dbAdmin = *newAdmin
			slog.Default().Info("已创建引导管理员账号", "username", username)
		} else if err != nil {
			slog.Default().Error("查询管理员失败", "username", username, "error", err)
			continue
		} else {
			if dbAdmin.Status != bootstrapStatus {
				if err := ctx.AdminModel.Update(bg, dbAdmin.ID, map[string]interface{}{"status": bootstrapStatus}); err != nil {
					slog.Default().Error("同步引导管理员状态失败", "username", username, "error", err)
				} else {
					dbAdmin.Status = bootstrapStatus
				}
			}
			// 存量行档案字段回填：仅补空字段，不覆盖用户已设置的值
			//（引导配置补齐 nickname/email/phone，docs/BUGS.md BUG-002）。
			updates := map[string]interface{}{}
			if dbAdmin.Nickname == "" && strings.TrimSpace(admin.Nickname) != "" {
				updates["nickname"] = strings.TrimSpace(admin.Nickname)
			}
			if dbAdmin.Email == "" && strings.TrimSpace(admin.Email) != "" {
				updates["email"] = strings.TrimSpace(admin.Email)
			}
			if dbAdmin.Phone == "" && strings.TrimSpace(admin.Phone) != "" {
				updates["phone"] = strings.TrimSpace(admin.Phone)
			}
			if len(updates) > 0 {
				if err := ctx.AdminModel.Update(bg, dbAdmin.ID, updates); err != nil {
					slog.Default().Warn("回填引导管理员档案字段失败", "username", username, "error", err)
				} else {
					if v, ok := updates["nickname"]; ok {
						dbAdmin.Nickname, _ = v.(string)
					}
					if v, ok := updates["email"]; ok {
						dbAdmin.Email, _ = v.(string)
					}
					if v, ok := updates["phone"]; ok {
						dbAdmin.Phone, _ = v.(string)
					}
					slog.Default().Info("已回填引导管理员档案字段", "username", username, "fields", len(updates))
				}
			}
		}

		for _, roleName := range admin.Roles {
			trimmed := strings.TrimSpace(roleName)
			if trimmed == "" {
				continue
			}
			var role model.Role
			if err := ctx.DB.WithContext(bg).Where("LOWER(name) = ?", strings.ToLower(trimmed)).First(&role).Error; err != nil {
				slog.Default().Error("为管理员查询角色失败", "username", username, "role", trimmed, "error", err)
				continue
			}
			var relCount int64
			if err := ctx.DB.WithContext(bg).
				Model(&model.AdminRole{}).
				Where("admin_id = ? AND role_id = ?", dbAdmin.ID, role.ID).
				Count(&relCount).Error; err != nil {
				slog.Default().Error("检查管理员是否已分配角色失败", "username", username, "role", trimmed, "error", err)
				continue
			}
			if relCount > 0 {
				continue
			}
			if err := ctx.AdminModel.AssignRole(bg, dbAdmin.ID, role.ID); err != nil {
				slog.Default().Error("为管理员分配角色失败", "username", username, "role", trimmed, "error", err)
			}
		}
		ctx.InvalidateAdminCache(bg, dbAdmin.ID, username)
	}

	return nil
}

func seedBootstrapRoles(ctx *ServiceContext) error {
	if ctx == nil || ctx.AdminManager == nil || ctx.RoleModel == nil || ctx.DB == nil {
		return nil
	}

	roles := ctx.AdminManager.ListRoles()
	if len(roles) == 0 {
		return nil
	}

	bg := context.Background()
	for _, role := range roles {
		if role == nil {
			continue
		}
		code := strings.TrimSpace(role.Code)
		if code == "" {
			continue
		}

		var dbRole model.Role
		err := ctx.DB.WithContext(bg).
			Where("name = ?", code).
			First(&dbRole).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			dbRole = model.Role{
				Name:        code,
				Description: strings.TrimSpace(role.Description),
				Category:    strings.TrimSpace(role.Name),
			}
			if err := ctx.RoleModel.Create(bg, &dbRole); err != nil {
				slog.Default().Error("创建引导角色失败", "code", code, "error", err)
				continue
			}
			slog.Default().Info("已创建引导角色", "code", code)
		} else if err != nil {
			slog.Default().Error("检查引导角色是否存在失败", "code", code, "error", err)
			continue
		}

		if len(role.Permissions) == 0 {
			continue
		}
		normalized, err := ctx.RoleModel.ValidatePermissionIDs(bg, role.Permissions)
		if err != nil {
			slog.Default().Error("校验角色权限失败", "code", code, "error", err)
			continue
		}

		// 获取现有权限
		existingPerms, err := ctx.RoleModel.GetRolePermissionIDs(bg, dbRole.ID)
		if err != nil {
			slog.Default().Error("查询角色现有权限失败", "code", code, "error", err)
			continue
		}

		// 如果已有权限数量匹配且内容相同，跳过
		if len(existingPerms) == len(normalized) {
			existingPermMap := make(map[string]bool)
			for _, ep := range existingPerms {
				existingPermMap[ep] = true
			}
			allMatch := true
			for _, perm := range normalized {
				if !existingPermMap[perm] {
					allMatch = false
					break
				}
			}
			if allMatch {
				continue // 权限完全一致，跳过更新
			}
		}

		// 使用 ReplacePermissions 更新，它会先删除旧的再插入新的
		if err := ctx.RoleModel.ReplacePermissions(bg, dbRole.ID, normalized); err != nil {
			slog.Default().Error("更新角色权限失败", "code", code, "error", err)
		}
	}
	return nil
}

func seedBootstrapPermissions(ctx *ServiceContext) error {
	if ctx == nil || ctx.AdminManager == nil || ctx.PermissionModel == nil || ctx.DB == nil {
		return nil
	}

	perms := ctx.AdminManager.ListPermissions()
	if len(perms) == 0 {
		return nil
	}

	bg := context.Background()
	for _, perm := range perms {
		if perm == nil {
			continue
		}
		code := strings.TrimSpace(perm.Code)
		if code == "" {
			continue
		}

		var existing model.Permission
		err := ctx.DB.WithContext(bg).Where("id = ?", code).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resource, action := derivePermissionResourceAction(code, perm.Module)
			category := strings.TrimSpace(perm.Category)
			if category == "" {
				category = resource
			}
			name := strings.TrimSpace(perm.Name)
			if name == "" {
				name = code
			}
			record := &model.Permission{
				ID:          code,
				Name:        name,
				Description: strings.TrimSpace(perm.Description),
				Resource:    resource,
				Action:      action,
				Category:    category,
			}
			if err := ctx.DB.WithContext(bg).Create(record).Error; err != nil {
				slog.Default().Error("创建引导权限失败", "code", code, "error", err)
			} else {
				slog.Default().Info("已创建引导权限", "code", code)
			}
		} else if err != nil {
			slog.Default().Error("检查权限是否存在失败", "code", code, "error", err)
		}
	}
	return nil
}

func derivePermissionResourceAction(code, module string) (string, string) {
	resource := strings.TrimSpace(module)
	action := "*"

	baseResource, baseAction := splitPermissionCode(code)
	if resource == "" {
		resource = baseResource
	}
	if baseAction != "" {
		action = baseAction
	}
	if resource == "" {
		resource = "global"
	}
	// 空 action 兜底已删：action 初值为 "*"，且仅在 baseAction 非空时被
	// 覆盖（splitPermissionCode 返回的 action 恒非空），恒不可能为 ""。
	return resource, action
}

func splitPermissionCode(code string) (string, string) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ""
	}
	parts := strings.SplitN(code, ":", 2)
	resource := strings.TrimSpace(parts[0])
	action := "*"
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		action = strings.TrimSpace(parts[1])
	}
	return resource, action
}

type bootstrapExtensionCatalogFile struct {
	Items []bootstrapExtensionCatalogItem `json:"items"`
}

type bootstrapExtensionCatalogItem struct {
	ExtensionID   string                            `json:"extensionId"`
	Name          string                            `json:"name"`
	DisplayName   string                            `json:"displayName"`
	Vendor        string                            `json:"vendor"`
	Kind          string                            `json:"kind"`
	Summary       string                            `json:"summary"`
	IconURL       string                            `json:"iconUrl"`
	HomepageURL   string                            `json:"homepageUrl"`
	Status        string                            `json:"status"`
	LatestVersion string                            `json:"latestVersion"`
	Releases      []bootstrapExtensionReleaseRecord `json:"releases"`
}

type bootstrapExtensionReleaseRecord struct {
	Version         string         `json:"version"`
	ReleaseChannel  string         `json:"releaseChannel"`
	MinCoreVersion  string         `json:"minCoreVersion"`
	PackageRef      string         `json:"packageRef"`
	Checksum        string         `json:"checksum"`
	Changelog       string         `json:"changelog"`
	PublishedAt     string         `json:"publishedAt"`
	PublishedAtUnix int64          `json:"publishedAtUnix"`
	Manifest        map[string]any `json:"manifest"`
}

func seedBootstrapExtensionCatalog(ctx *ServiceContext) error {
	if ctx == nil || ctx.DB == nil {
		return nil
	}

	seedPath := filepath.Join(resolveBootstrapAuthDir(ctx.Config), "extensions", "catalog.json")
	data, err := os.ReadFile(seedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read extension catalog seed failed: %w", err)
	}

	var payload bootstrapExtensionCatalogFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse extension catalog seed failed: %w", err)
	}
	if len(payload.Items) == 0 {
		return nil
	}

	bg := context.Background()
	for _, item := range payload.Items {
		extID := strings.TrimSpace(item.ExtensionID)
		if extID == "" {
			continue
		}
		record := model.ExtensionCatalog{
			ExtensionID:   extID,
			Name:          strings.TrimSpace(item.Name),
			DisplayName:   strings.TrimSpace(item.DisplayName),
			Vendor:        strings.TrimSpace(item.Vendor),
			Kind:          strings.TrimSpace(item.Kind),
			Summary:       strings.TrimSpace(item.Summary),
			IconURL:       strings.TrimSpace(item.IconURL),
			HomepageURL:   strings.TrimSpace(item.HomepageURL),
			Status:        strings.TrimSpace(item.Status),
			LatestVersion: strings.TrimSpace(item.LatestVersion),
		}
		if record.Name == "" {
			record.Name = extID
		}
		if record.DisplayName == "" {
			record.DisplayName = record.Name
		}
		if record.Vendor == "" {
			record.Vendor = "official"
		}
		if record.Kind == "" {
			record.Kind = "official"
		}
		if record.Status == "" {
			record.Status = "active"
		}

		var existing model.ExtensionCatalog
		err := ctx.DB.WithContext(bg).Where("extension_id = ?", extID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := ctx.DB.WithContext(bg).Create(&record).Error; err != nil {
				return fmt.Errorf("create extension catalog seed failed (%s): %w", extID, err)
			}
		} else if err != nil {
			return fmt.Errorf("query extension catalog seed failed (%s): %w", extID, err)
		} else {
			updates := map[string]any{
				"name":           record.Name,
				"display_name":   record.DisplayName,
				"vendor":         record.Vendor,
				"kind":           record.Kind,
				"summary":        record.Summary,
				"icon_url":       record.IconURL,
				"homepage_url":   record.HomepageURL,
				"status":         record.Status,
				"latest_version": record.LatestVersion,
			}
			if err := ctx.DB.WithContext(bg).Model(&existing).Updates(updates).Error; err != nil {
				return fmt.Errorf("update extension catalog seed failed (%s): %w", extID, err)
			}
		}

		for _, release := range item.Releases {
			version := strings.TrimSpace(release.Version)
			if version == "" {
				continue
			}
			manifest := release.Manifest
			if len(manifest) == 0 {
				manifest = map[string]any{
					"id":      extID,
					"name":    record.Name,
					"version": version,
				}
			}
			manifestJSON, _ := json.Marshal(manifest)
			publishedAtUnix := release.PublishedAtUnix
			if publishedAtUnix == 0 {
				if ts := strings.TrimSpace(release.PublishedAt); ts != "" {
					if parsed, parseErr := time.Parse(time.RFC3339, ts); parseErr == nil {
						publishedAtUnix = parsed.Unix()
					}
				}
			}
			if publishedAtUnix == 0 {
				publishedAtUnix = time.Now().Unix()
			}

			var existingRelease model.ExtensionRelease
			err := ctx.DB.WithContext(bg).
				Where("extension_id = ? AND version = ?", extID, version).
				First(&existingRelease).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newRelease := model.ExtensionRelease{
					ExtensionID:     extID,
					Version:         version,
					ReleaseChannel:  strings.TrimSpace(release.ReleaseChannel),
					ManifestJSON:    model.JSON(manifestJSON),
					PackageRef:      strings.TrimSpace(release.PackageRef),
					Checksum:        strings.TrimSpace(release.Checksum),
					MinCoreVersion:  strings.TrimSpace(release.MinCoreVersion),
					Changelog:       strings.TrimSpace(release.Changelog),
					PublishedAtUnix: publishedAtUnix,
				}
				if newRelease.ReleaseChannel == "" {
					newRelease.ReleaseChannel = "stable"
				}
				if err := ctx.DB.WithContext(bg).Create(&newRelease).Error; err != nil {
					return fmt.Errorf("create extension release seed failed (%s@%s): %w", extID, version, err)
				}
			} else if err != nil {
				return fmt.Errorf("query extension release seed failed (%s@%s): %w", extID, version, err)
			}
		}
	}

	slog.Default().Info("extension catalog bootstrap seeded", "path", seedPath, "items", len(payload.Items))
	return nil
}

// validateStoreConfig 是 objstore.Validate 的包级接缝：生产恒为真实实现，
// 测试注入恒 nil 驱动 initObjectStore 的 default 分支（真实 Validate 以同
// 一 driver 串先拒绝未知值，default 仅在跨包白名单失配时可达，论证见
// 调用处注释）。
var validateStoreConfig = objstore.Validate

// initObjectStore 构建对象存储。secrets 为可选（variadic）：生产装配传入
// NewServiceContext 的 resolver（AK/SK 支持 vault:// 引用，todo 4251）；
// 测试面/未传时为 nil（引用原样直通，存量零行为变化）。
func initObjectStore(ctx context.Context, cfg config.StorageConfig, secrets ...vault.Resolver) (objstore.Store, error) {
	driver := strings.TrimSpace(cfg.Driver)
	if driver == "" {
		return nil, nil
	}
	var resolve vault.Resolver
	if len(secrets) > 0 {
		resolve = secrets[0]
	}

	// AK/SK 支持 vault:// 引用（todo 4251）；解析失败按「未配置」降级
	// （fail-closed，不把引用串当真凭据发往对象存储端点）。
	accessKey := strings.TrimSpace(cfg.AccessKey)
	secretKey := strings.TrimSpace(cfg.SecretKey)
	if v, rerr := resolveSecretOrPassthrough(ctx, resolve, accessKey); rerr != nil {
		slog.WarnContext(ctx, "objstore accessKey resolve failed; treat as unset", "error", rerr)
		accessKey = ""
	} else {
		accessKey = strings.TrimSpace(v)
	}
	if v, rerr := resolveSecretOrPassthrough(ctx, resolve, secretKey); rerr != nil {
		slog.WarnContext(ctx, "objstore secretKey resolve failed; treat as unset", "error", rerr)
		secretKey = ""
	} else {
		secretKey = strings.TrimSpace(v)
	}

	storeCfg := objstore.Config{
		Driver:         driver,
		Bucket:         strings.TrimSpace(cfg.Bucket),
		Region:         strings.TrimSpace(cfg.Region),
		Endpoint:       strings.TrimSpace(cfg.Endpoint),
		AccessKey:      accessKey,
		SecretKey:      secretKey,
		ForcePathStyle: cfg.ForcePathStyle,
		BaseDir:        toAbs(strings.TrimSpace(cfg.BaseDir)),
	}
	if ttl := strings.TrimSpace(cfg.SignedURLTTL); ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			storeCfg.SignedURLTTL = d
		}
	}

	if err := validateStoreConfig(storeCfg); err != nil {
		return nil, err
	}

	switch strings.ToLower(driver) {
	case "s3":
		return objstore.OpenS3(ctx, storeCfg)
	case "oss":
		return objstore.OpenOSS(ctx, storeCfg)
	case "cos":
		return objstore.OpenCOS(ctx, storeCfg)
	case "file":
		return objstore.OpenFile(ctx, storeCfg)
	default:
		// 不可达论证（C 类）：上方 objstore.Validate 以同一 driver 串
		// （同样 ToLower）先做 default 拒绝（"unknown storage driver"），
		// 能到达本 switch 的 driver 必属四个已知值。保留 default 是因为
		// Validate 是跨包隐式契约：objstore 新增驱动而本 switch 未同步时，
		// 显式报错优于静默返回 (nil, nil) 的空 Store。经
		// validateStoreConfig 接缝测试驱动。
		return nil, fmt.Errorf("unsupported storage driver: %s", driver)
	}
}

// ============================================================================
// 认证中间件
// ============================================================================
