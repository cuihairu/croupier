package auth

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerFixture 构造带角色模型的最小 Service（走 Attach 生产装配路径）。
func registerFixture(t *testing.T, cfg config.AuthProvidersConfig) *Service {
	t.Helper()
	db := setupTestDB(t)
	require.NoError(t, model.NewRoleModel(db).Create(context.Background(), &model.Role{Name: "viewer"}))
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), jwtutil.DevSecret()).
		WithRecoveryDB(db).
		WithRoleModel(model.NewRoleModel(db))
	ip, err := BuildIdentityProviders(cfg)
	require.NoError(t, err)
	return ip.Attach(svc)
}

// TestRegister_DisabledByDefault 未开启时注册被拒且不建号。
func TestRegister_DisabledByDefault(t *testing.T) {
	svc := registerFixture(t, config.AuthProvidersConfig{})
	assert.False(t, svc.RegisterEnabled(), "注册默认关闭")
	_, err := svc.Register(context.Background(), &RegisterRequest{Username: "newuser", Password: "Str0ngPass!x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未开启")
}

// TestRegister_HappyPath 开启后注册成功：建号 + 默认角色 + 本地登录可用。
func TestRegister_HappyPath(t *testing.T) {
	svc := registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true, DefaultRoles: []string{"viewer"}},
	})
	assert.True(t, svc.RegisterEnabled())

	admin, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "newuser",
		Password: "Str0ngPass!x",
		Nickname: "  新用户  ",
		Email:    "new@example.com",
		ClientIP: "10.0.0.9",
	})
	require.NoError(t, err)
	assert.Equal(t, "newuser", admin.Username)
	assert.Equal(t, "新用户", admin.Nickname, "昵称 trim 落库")
	assert.Equal(t, "new@example.com", admin.Email)
	assert.Equal(t, 1, admin.Status, "注册账号直接可用")

	roles, err := svc.adminModel.GetAdminRoles(context.Background(), admin.ID)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.Equal(t, "viewer", roles[0].Name, "注册默认角色已绑定")

	// 注册账号可用密码正常登录（无 mustChangePassword 标记）
	resp, loginErr := svc.Login(context.Background(), &LoginRequest{Username: "newuser", Password: "Str0ngPass!x"})
	require.NoError(t, loginErr)
	assert.NotEmpty(t, resp.Token)
	assert.False(t, resp.MustChangePassword)
}

// TestRegister_NicknameDefaultsToUsername 昵称缺省回落用户名。
func TestRegister_NicknameDefaultsToUsername(t *testing.T) {
	svc := registerFixture(t, config.AuthProvidersConfig{Register: config.RegisterConfig{Enabled: true}})
	admin, err := svc.Register(context.Background(), &RegisterRequest{Username: "nickless", Password: "Str0ngPass!x"})
	require.NoError(t, err)
	assert.Equal(t, "nickless", admin.Nickname)
}

// TestRegister_Rejections 拒绝面：非法用户名/弱密码/重名。
func TestRegister_Rejections(t *testing.T) {
	svc := registerFixture(t, config.AuthProvidersConfig{Register: config.RegisterConfig{Enabled: true}})

	for name, req := range map[string]RegisterRequest{
		"用户名过短":    {Username: "ab", Password: "Str0ngPass!x"},
		"用户名含非法字符": {Username: "bad name!", Password: "Str0ngPass!x"},
		"弱密码":      {Username: "weakpwd", Password: "123456"},
	} {
		_, err := svc.Register(context.Background(), &req)
		require.Error(t, err, name)
	}

	// 重名
	_, err := svc.Register(context.Background(), &RegisterRequest{Username: "dup", Password: "Str0ngPass!x"})
	require.NoError(t, err)
	_, err = svc.Register(context.Background(), &RegisterRequest{Username: "dup", Password: "An0therPass!x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

// TestRegister_UnknownRoleSkipped 默认角色名不存在时告警跳过、建号不受影响。
func TestRegister_UnknownRoleSkipped(t *testing.T) {
	svc := registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true, DefaultRoles: []string{"ghost-role"}},
	})
	admin, err := svc.Register(context.Background(), &RegisterRequest{Username: "noroles", Password: "Str0ngPass!x"})
	require.NoError(t, err)
	roles, roleErr := svc.adminModel.GetAdminRoles(context.Background(), admin.ID)
	require.NoError(t, roleErr)
	assert.Empty(t, roles, "未知角色跳过不阻塞注册")
}

// TestRefreshIdentityProviders_RegisterToggle 注册开关经热刷新级联更新。
func TestRefreshIdentityProviders_RegisterToggle(t *testing.T) {
	svc := registerFixture(t, config.AuthProvidersConfig{})
	assert.False(t, svc.RegisterEnabled())

	require.NoError(t, svc.RefreshIdentityProviders(config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true, DefaultRoles: []string{"viewer"}},
	}))
	assert.True(t, svc.RegisterEnabled())
}
