package auth

// 注册邮箱策略测试（OPEN-ISSUES #51c）：域后缀白名单（子域放行/域外拒绝）、
// 别名限制（+tag 拒、local 去点归一查重、归一后不冲突放行）、基础形态
// 校验、email 留空跳过策略。L3 键经 settings 测试库播种。

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedEmailPolicy(t *testing.T, pairs map[string]string) {
	t.Helper()
	settings.ResetForTest()
	db := setupTestDB(t)
	store := model.NewPlatformSettingModel(db)
	require.NoError(t, model.AutoMigrate(db))
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	for k, v := range pairs {
		require.NoError(t, store.Set(context.Background(), k, json.RawMessage(v), "tester"))
	}
	settings.Current().Reload(context.Background(), store)
	t.Cleanup(settings.ResetForTest)
}

func TestRegister_EmailDomainWhitelist(t *testing.T) {
	seedEmailPolicy(t, map[string]string{settings.KeyAuthEmailDomainWhitelist: `"example.com, foo.io"`})
	svc := registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})

	// 白名单域与子域放行
	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "wluser1", Password: "Str0ngPass!x", Email: "wluser1@example.com",
	})
	require.NoError(t, err)
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "wluser2", Password: "Str0ngPass!x", Email: "wluser2@api.example.com",
	})
	require.NoError(t, err)
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "wluser3", Password: "Str0ngPass!x", Email: "wluser3@foo.io",
	})
	require.NoError(t, err)

	// 域外拒绝
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "wluser4", Password: "Str0ngPass!x", Email: "wluser4@gmail.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不在允许清单")

	// evil-example.com 后缀撞车不匹配
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "wluser5", Password: "Str0ngPass!x", Email: "x@evil-example.com",
	})
	require.Error(t, err)
}

func TestRegister_EmailAliasRestriction(t *testing.T) {
	seedEmailPolicy(t, map[string]string{settings.KeyAuthEmailAliasRestriction: `true`})
	svc := registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "baseuser", Password: "Str0ngPass!x", Email: "us.dot@example.com",
	})
	require.NoError(t, err)

	// + 别名形态拒绝
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "alias1", Password: "Str0ngPass!x", Email: "someone+tag@example.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "+tag")

	// local 去点归一撞既有账号（us.dot@example.com 归一 usdot）
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "alias2", Password: "Str0ngPass!x", Email: "us.dot@example.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已被其他账号使用")
	// 大小写归一同样拦
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "alias3", Password: "Str0ngPass!x", Email: "US.DOT@EXAMPLE.COM",
	})
	require.Error(t, err)

	// 归一后不冲突放行
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "alias4", Password: "Str0ngPass!x", Email: "other.user@example.com",
	})
	require.NoError(t, err)
}

func TestRegister_EmailFormatAndEmptySkips(t *testing.T) {
	seedEmailPolicy(t, map[string]string{
		settings.KeyAuthEmailDomainWhitelist: `"example.com"`,
	})
	svc := registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})

	// 形态无效：无 @ / 域无点
	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "fmtuser1", Password: "Str0ngPass!x", Email: "not-an-email",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "格式无效")

	// email 留空跳过策略（邮箱选填）
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "fmtuser2", Password: "Str0ngPass!x",
	})
	require.NoError(t, err)
}
