package auth

// Registration email policy tests (OPEN-ISSUES #51c): domain suffix
// whitelisting (subdomain allowed / out-of-list rejected), alias restrictions
// (+tag rejected, dot-stripped local-part normalization duplicate check,
// allowed when normalization finds no conflict), basic shape validation, and
// skipping the policy when email is left blank. L3 keys are seeded through the
// settings test database.

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

	// whitelisted domains and subdomains are allowed
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

	// out-of-list domains are rejected
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "wluser4", Password: "Str0ngPass!x", Email: "wluser4@gmail.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不在允许清单")

	// the evil-example.com suffix collision does not match
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

	// the + alias shape is rejected
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "alias1", Password: "Str0ngPass!x", Email: "someone+tag@example.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "+tag")

	// the dot-stripped local-part normalization collides with the existing
	// account (us.dot@example.com normalizes to usdot)
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "alias2", Password: "Str0ngPass!x", Email: "us.dot@example.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已被其他账号使用")
	// case normalization is blocked the same way
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "alias3", Password: "Str0ngPass!x", Email: "US.DOT@EXAMPLE.COM",
	})
	require.Error(t, err)

	// after normalization there is no conflict, so it is allowed through
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

	// invalid shape: no @ / dotless domain
	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "fmtuser1", Password: "Str0ngPass!x", Email: "not-an-email",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "格式无效")

	// leaving email blank skips the policy (email is optional)
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "fmtuser2", Password: "Str0ngPass!x",
	})
	require.NoError(t, err)
}
