package auth

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setMFAPolicy 构造真实 Layered 单例并写入 security.mfaRequired（生产读取
// 路径：settings.Current()），用后复位避免污染同包其他用例。
func setMFAPolicy(t *testing.T, required bool) {
	t.Helper()
	dsn := "file:" + t.TempDir() + "/authpolicy.db?_pragma=synchronous(OFF)"
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	store := model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	raw := `false`
	if required {
		raw = `true`
	}
	require.NoError(t, store.Set(context.Background(), settings.KeySecurityMFARequired,
		json.RawMessage(raw), "tester"))
	settings.Current().Reload(context.Background(), store)
	t.Cleanup(settings.ResetForTest)
}

// TestMFASetupRequired_Matrix mfaSetupRequired 判定矩阵：仅 local + 未绑定
// TOTP + 策略开启三者同时成立才强制绑定；LDAP/OIDC 账号（MFA 由 IdP 负责）
// 与已绑定账号永不强制。
func TestMFASetupRequired_Matrix(t *testing.T) {
	setMFAPolicy(t, false)
	assert.False(t, mfaSetupRequired("local", false), "策略关闭 = 自助模式")
	assert.False(t, mfaSetupRequired("local", true))
	assert.False(t, mfaSetupRequired("ldap", false))
	assert.False(t, mfaSetupRequired("oidc", false))

	setMFAPolicy(t, true)
	assert.True(t, mfaSetupRequired("local", false), "策略开启 + 未绑定 = 强制")
	assert.False(t, mfaSetupRequired("local", true), "已绑定不重复强制")
	assert.False(t, mfaSetupRequired("ldap", false), "外部身份源由 IdP 负责")
	assert.False(t, mfaSetupRequired("oidc", false))
}
