package utils

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// initPolicySettings 构造真实 Layered 单例（settings.Current() 生产读取路径），
// 经 L3 store 写入策略键——策略校验与设置页走同一数据面。
func initPolicySettings(t *testing.T, overrides map[string]string) {
	t.Helper()
	dsn := "file:" + t.TempDir() + "/policy.db?_pragma=synchronous(OFF)"
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	store := model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	for key, raw := range overrides {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	settings.Current().Reload(context.Background(), store)
	t.Cleanup(settings.ResetForTest)
}

func TestValidatePassword_PolicyOffKeepsBuiltInBaseline(t *testing.T) {
	initPolicySettings(t, nil)

	// 内置基线：8 位 + 2 类即可通过（纯小写+数字）
	_, err := ValidatePassword("abcd1234")
	assert.NoError(t, err)
	// 弱密码表/长度仍生效
	_, err = ValidatePassword("password")
	assert.Error(t, err)
	_, err = ValidatePassword("ab1")
	assert.Error(t, err)
}

func TestValidatePassword_PolicyTightens(t *testing.T) {
	initPolicySettings(t, map[string]string{
		"security.passwordMinLength":        "12",
		"security.passwordRequireUppercase": "true",
		"security.passwordRequireSpecial":   "true",
	})

	// 通过内置但未达策略：10 位无大写无特殊字符
	_, err := ValidatePassword("abcd123456")
	require.Error(t, err)
	forbidden, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Contains(t, forbidden.Message, "账号安全策略")

	// 长度达标但缺大写
	_, err = ValidatePassword("abcd1234efg!@#")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "大写字母")

	// 缺特殊字符
	_, err = ValidatePassword("Abcd1234efgh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "特殊字符")

	// 全部达标
	_, err = ValidatePassword("Abcd1234!efg")
	assert.NoError(t, err)
}

func TestValidatePassword_PolicyMinLengthOnlyTightens(t *testing.T) {
	// 策略值小于内置 8：不放宽（仍按内置基线拒绝 3 位）
	initPolicySettings(t, map[string]string{"security.passwordMinLength": "4"})
	_, err := ValidatePassword("ab1")
	assert.Error(t, err)
	_, err = ValidatePassword("abcd1234")
	assert.NoError(t, err)
}

func TestPasswordExpiresAtFromPolicy(t *testing.T) {
	initPolicySettings(t, nil)
	assert.Nil(t, PasswordExpiresAtFromPolicy(), "策略关闭 = 永不过期")

	initPolicySettings(t, map[string]string{"security.passwordMaxAgeDays": "30"})
	expiresAt := PasswordExpiresAtFromPolicy()
	require.NotNil(t, expiresAt)
	// 到期时刻 ≈ 30 天后（±1 分钟容差）
	assert.WithinDuration(t, time.Now().UTC().Add(30*24*time.Hour), *expiresAt, time.Minute)
}

func TestSecurityPolicyMFARequiredReadsThroughCurrent(t *testing.T) {
	// mfaSetupRequired/mfaGate 的 settings 读取面：Current() 经 L3 热生效
	initPolicySettings(t, map[string]string{"security.mfaRequired": "true"})
	assert.True(t, settings.Current().SecurityPolicy().MFARequired)
}
