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

// setMFAPolicy builds a real Layered singleton and writes
// security.mfaRequired (the production read path: settings.Current()), then
// resets it afterwards to avoid polluting other cases in this package.
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

// TestMFASetupRequired_Matrix is the decision matrix for mfaSetupRequired:
// binding is enforced only when local + TOTP not bound + policy on all hold at
// the same time; LDAP/OIDC accounts (MFA is the IdP's responsibility) and
// already-bound accounts are never forced.
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
