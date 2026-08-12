package svc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AdminUser 管理员用户结构
//
// AdminManager 用途说明：
// - 主要用于初始化和引导数据管理（bootstrap data）
// - 支持从配置文件导入管理员账号
// - 运行时会自动将明文密码转换为 bcrypt 哈希
//
// 密码字段说明：
// - 支持 bcrypt 哈希密码（推荐）：以 "$2a$" 或 "$2b$" 开头的哈希值
// - 支持明文密码（仅用于配置文件导入）：运行时自动转换为 bcrypt 哈希
// - ValidateUser 会自动检测并使用正确的验证方式
//
// 安全建议：
// - 生产环境配置文件应使用预哈希的 bcrypt 密码
// - 可使用 `htpasswd -nbB username password` 生成 bcrypt 哈希
// - 或使用在线工具：https://bcrypt-generator.com/
// - 运行时不再进行明文密码比较（除非是旧配置文件）
type AdminUser struct {
	Username string   `json:"username"`
	Password string   `json:"password"` // bcrypt 哈希或明文（见上方说明）
	Roles    []string `json:"roles"`
	Nickname string   `json:"nickname,omitempty"`
	Email    string   `json:"email,omitempty"`
	Phone    string   `json:"phone,omitempty"`
	Status   int      `json:"status"` // 1:active 0:disabled
	CreateAt string   `json:"createAt,omitempty"`
	UpdateAt string   `json:"updateAt,omitempty"`
}

// IsHashedPassword 检查密码是否为 bcrypt 哈希格式
func (u *AdminUser) IsHashedPassword() bool {
	return strings.HasPrefix(u.Password, "$2a$") || strings.HasPrefix(u.Password, "$2b$")
}

// Role 角色定义
type Role struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Level       int      `json:"level"`
	Permissions []string `json:"permissions"`
}

// Permission 权限定义
type Permission struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Module      string `json:"module"`
}

// AdminManager 管理员管理器
type AdminManager struct {
	admins      map[string]*AdminUser
	roles       map[string]*Role
	permissions map[string]*Permission
	mu          sync.RWMutex
	configDir   string
}

// NewAdminManager 创建管理员管理器
func NewAdminManager(configDir string) *AdminManager {
	return &AdminManager{
		admins:      make(map[string]*AdminUser),
		roles:       make(map[string]*Role),
		permissions: make(map[string]*Permission),
		configDir:   configDir,
	}
}

// Initialize 初始化管理器，加载默认数据
func (am *AdminManager) Initialize() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 加载默认数据
	if err := am.loadDefaultAdmins(); err != nil {
		slog.Default().Error("Failed to load default admins", "error", err)
		return err
	}

	if err := am.loadDefaultRoles(); err != nil {
		slog.Default().Error("Failed to load default roles", "error", err)
		return err
	}

	if err := am.loadDefaultPermissions(); err != nil {
		slog.Default().Error("Failed to load default permissions", "error", err)
		return err
	}

	slog.Default().Info("Admin manager initialized successfully")
	return nil
}

// loadDefaultAdmins 加载默认管理员账号
func (am *AdminManager) loadDefaultAdmins() error {
	slog.Default().Info("Loading default admins", "configDir", am.configDir)
	// 尝试加载多个可能的配置文件
	configFiles := []string{"admins.json", "users.json"}

	for _, configFile := range configFiles {
		adminsPath := filepath.Join(am.configDir, configFile)
		slog.Default().Debug("Trying to load admin config", "path", adminsPath)

		// 检查文件是否存在
		if _, err := os.Stat(adminsPath); os.IsNotExist(err) {
			slog.Default().Debug("Admin config file not found, trying next", "file", adminsPath)
			continue
		}

		data, err := os.ReadFile(adminsPath)
		if err != nil {
			slog.Default().Error("Failed to read config file", "file", configFile, "error", err)
			continue
		}

		var defaultAdmins []AdminUser
		if err := json.Unmarshal(data, &defaultAdmins); err != nil {
			slog.Default().Error("Failed to parse config file", "file", configFile, "error", err)
			continue
		}

		now := time.Now().Format("2006-01-02 15:04:05")
		loadedCount := 0

		for i, admin := range defaultAdmins {
			// 设置默认值
			if admin.Status == 0 {
				defaultAdmins[i].Status = 1 // 默认激活
			}
			if admin.CreateAt == "" {
				defaultAdmins[i].CreateAt = now
			}
			if admin.UpdateAt == "" {
				defaultAdmins[i].UpdateAt = now
			}

			// 检查是否已存在，不存在则添加
			if _, exists := am.admins[admin.Username]; !exists {
				am.admins[admin.Username] = &defaultAdmins[i]
				slog.Default().Info("Loaded default admin", "file", configFile, "username", admin.Username, "roles", admin.Roles)
				loadedCount++
			} else {
				slog.Default().Debug("Admin already loaded, skipping", "username", admin.Username)
			}
		}

		if loadedCount > 0 {
			slog.Default().Info("Successfully loaded admins", "count", loadedCount, "file", configFile)
			return nil
		}
	}

	slog.Default().Warn("No valid admin config files found", "files", configFiles, "configDir", am.configDir)
	return nil
}

// loadDefaultRoles 加载默认角色
func (am *AdminManager) loadDefaultRoles() error {
	rolesPath := filepath.Join(am.configDir, "roles.json")

	// 检查文件是否存在
	if _, err := os.Stat(rolesPath); os.IsNotExist(err) {
		slog.Default().Warn("Roles config file not found", "path", rolesPath)
		return nil
	}

	data, err := os.ReadFile(rolesPath)
	if err != nil {
		return fmt.Errorf("failed to read roles config: %v", err)
	}

	var defaultRoles []Role
	if err := json.Unmarshal(data, &defaultRoles); err != nil {
		return fmt.Errorf("failed to parse roles config: %v", err)
	}

	for _, role := range defaultRoles {
		am.roles[role.Code] = &role
		slog.Default().Info("Loaded default role", "code", role.Code, "name", role.Name)
	}

	return nil
}

// loadDefaultPermissions 加载默认权限
func (am *AdminManager) loadDefaultPermissions() error {
	permissionsPath := filepath.Join(am.configDir, "permissions.json")

	// 检查文件是否存在
	if _, err := os.Stat(permissionsPath); os.IsNotExist(err) {
		slog.Default().Warn("Permissions config file not found", "path", permissionsPath)
		return nil
	}

	data, err := os.ReadFile(permissionsPath)
	if err != nil {
		return fmt.Errorf("failed to read permissions config: %v", err)
	}

	var defaultPermissions []Permission
	if err := json.Unmarshal(data, &defaultPermissions); err != nil {
		return fmt.Errorf("failed to parse permissions config: %v", err)
	}

	for _, permission := range defaultPermissions {
		am.permissions[permission.Code] = &permission
	}

	return nil
}

// ValidateUser 验证用户登录。
// 新配置应只使用 bcrypt 哈希；明文仅为历史兼容导入保留。
func (am *AdminManager) ValidateUser(username, password string) (*AdminUser, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	admin, exists := am.admins[username]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if admin.Status != 1 {
		return nil, fmt.Errorf("user is disabled")
	}

	// 如果存储的密码是 bcrypt 哈希，使用 bcrypt 比较
	if admin.IsHashedPassword() {
		if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password)); err != nil {
			return nil, fmt.Errorf("invalid password")
		}
	} else {
		// 明文密码仅用于历史兼容，建议通过 seedBootstrapAdmins 迁移到数据库哈希存储。
		slog.Default().Warn("AdminManager using legacy plaintext password comparison", "username", username)
		if admin.Password != password {
			return nil, fmt.Errorf("invalid password")
		}
	}

	return admin, nil
}

// CreateAdmin 创建管理员账号
// 如果密码不是 bcrypt 哈希格式，将自动进行哈希处理
func (am *AdminManager) CreateAdmin(admin *AdminUser) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.admins[admin.Username]; exists {
		return fmt.Errorf("admin already exists")
	}

	// 如果密码不是 bcrypt 哈希，自动进行哈希处理
	if !admin.IsHashedPassword() {
		slog.Default().Info("Hashing plaintext password for new admin", "username", admin.Username)
		hashedBytes, err := bcrypt.GenerateFromPassword([]byte(admin.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		admin.Password = string(hashedBytes)
	}

	admin.Status = 1
	admin.CreateAt = time.Now().Format("2006-01-02 15:04:05")
	admin.UpdateAt = admin.CreateAt

	am.admins[admin.Username] = admin
	slog.Default().Info("Created admin", "username", admin.Username)
	return nil
}

// GetAdmin 获取管理员信息
func (am *AdminManager) GetAdmin(username string) (*AdminUser, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	admin, exists := am.admins[username]
	if !exists {
		return nil, fmt.Errorf("admin not found")
	}

	return admin, nil
}

// ListAdmins 获取所有管理员列表
func (am *AdminManager) ListAdmins() []*AdminUser {
	am.mu.RLock()
	defer am.mu.RUnlock()

	admins := make([]*AdminUser, 0, len(am.admins))
	for _, admin := range am.admins {
		admins = append(admins, admin)
	}

	return admins
}

// UpdateAdmin 更新管理员信息
func (am *AdminManager) UpdateAdmin(username string, updates map[string]interface{}) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	admin, exists := am.admins[username]
	if !exists {
		return fmt.Errorf("admin not found")
	}

	if nickname, ok := updates["nickname"].(string); ok {
		admin.Nickname = nickname
	}
	if email, ok := updates["email"].(string); ok {
		admin.Email = email
	}
	if phone, ok := updates["phone"].(string); ok {
		admin.Phone = phone
	}
	if roles, ok := updates["roles"].([]string); ok {
		admin.Roles = roles
	}
	if status, ok := updates["status"].(int); ok {
		admin.Status = status
	}

	admin.UpdateAt = time.Now().Format("2006-01-02 15:04:05")

	slog.Default().Info("Updated admin", "username", username)
	return nil
}

// DeleteAdmin 删除管理员账号
func (am *AdminManager) DeleteAdmin(username string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.admins[username]; !exists {
		return fmt.Errorf("admin not found")
	}

	delete(am.admins, username)
	slog.Default().Info("Deleted admin", "username", username)
	return nil
}

// ResetPassword 重置管理员密码
// 如果新密码不是 bcrypt 哈希格式，将自动进行哈希处理
func (am *AdminManager) ResetPassword(username, newPassword string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	admin, exists := am.admins[username]
	if !exists {
		return fmt.Errorf("admin not found")
	}

	// 如果密码不是 bcrypt 哈希，自动进行哈希处理
	if !strings.HasPrefix(newPassword, "$2a$") && !strings.HasPrefix(newPassword, "$2b$") {
		slog.Default().Info("Hashing new password for admin", "username", username)
		hashedBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		admin.Password = string(hashedBytes)
	} else {
		admin.Password = newPassword
	}

	admin.UpdateAt = time.Now().Format("2006-01-02 15:04:05")

	slog.Default().Info("Reset password for admin", "username", username)
	return nil
}

// GetRole 获取角色信息
func (am *AdminManager) GetRole(code string) (*Role, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	role, exists := am.roles[code]
	if !exists {
		return nil, fmt.Errorf("role not found")
	}

	return role, nil
}

// ListRoles 获取所有角色列表
func (am *AdminManager) ListRoles() []*Role {
	am.mu.RLock()
	defer am.mu.RUnlock()

	roles := make([]*Role, 0, len(am.roles))
	for _, role := range am.roles {
		roles = append(roles, role)
	}

	return roles
}

// ListPermissions 获取所有默认权限
func (am *AdminManager) ListPermissions() []*Permission {
	am.mu.RLock()
	defer am.mu.RUnlock()

	perms := make([]*Permission, 0, len(am.permissions))
	for _, perm := range am.permissions {
		perms = append(perms, perm)
	}
	return perms
}

// CheckPermission 检查用户权限
func (am *AdminManager) CheckPermission(username, permission string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	admin, exists := am.admins[username]
	if !exists {
		return false
	}

	// 检查是否有通配符权限
	for _, role := range admin.Roles {
		if role == "admin" || role == "super_admin" {
			return true
		}

		roleInfo, roleExists := am.roles[role]
		if roleExists {
			for _, perm := range roleInfo.Permissions {
				if perm == "*" || perm == "admin:all" || perm == permission {
					return true
				}
			}
		}
	}

	return false
}

// GetAdminPermissions 获取管理员所有权限
func (am *AdminManager) GetAdminPermissions(username string) []string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	admin, exists := am.admins[username]
	if !exists {
		return nil
	}

	permissions := make(map[string]bool)

	for _, roleCode := range admin.Roles {
		role, roleExists := am.roles[roleCode]
		if roleExists {
			for _, perm := range role.Permissions {
				permissions[perm] = true
			}
		}
	}

	// 转换为切片
	result := make([]string, 0, len(permissions))
	for perm := range permissions {
		result = append(result, perm)
	}

	return result
}
