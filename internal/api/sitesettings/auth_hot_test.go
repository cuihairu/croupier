package sitesettings

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
)

// newAuthTestHandler initializes the global Layered with a test store. Every call
// goes through InitLayered rather than a once-guard: other test files in this
// package (putkey/testemail/handler_extra) call ResetForTest and clear the global
// singleton, so a once-style helper would get a nil Layered from then on -- the
// put lands in the per-test store while Reload is silently swallowed by the nil
// guard, and every auth.* key falls back to defaults (under shuffle this goes red
// depending on order; proven by the OIDC Discovery series). InitLayered itself has
// sync.Once semantics, so repeated calls without a reset are no-ops and the
// production "wire it up exactly once" constraint is unchanged.
func newAuthTestHandler(t *testing.T) *Handler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/site.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.PlatformSetting{}); err != nil {
		t.Fatal(err)
	}
	store := model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	return NewHandler(settings.Current(), store)
}

func putSiteKey(t *testing.T, h *Handler, key, jsonValue string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: key}}
	c.Request = httptest.NewRequest("PUT", "/site/"+key, strings.NewReader(`{"value":`+jsonValue+`}`))
	h.PutKey(c)
	return w
}

// Re-saving the masked secret placeholder keeps the old value (a read->save
// round-trip must not destroy the real value).
func TestPutSecretPlaceholderPreserved(t *testing.T) {
	h := newAuthTestHandler(t)
	if w := putSiteKey(t, h, "auth.oidc.clientSecret", `"real-secret-123"`); w.Code != 200 {
		t.Fatalf("first put = %d %s", w.Code, w.Body.String())
	}
	if w := putSiteKey(t, h, "auth.oidc.clientSecret", `"****t123"`); w.Code != 200 {
		t.Fatalf("mask put = %d %s", w.Code, w.Body.String())
	}
	snap := settings.Current().AuthSnapshot()
	if !snap.OIDC.SecretSet {
		t.Fatal("secret should remain set after mask re-save")
	}
	// The real value was not overwritten by the mask: the masked echo is still based
	// on the original value's tail
	if !strings.HasSuffix(snap.OIDC.SecretMasked, "-123") {
		t.Fatalf("masked = %q, want suffix t123", snap.OIDC.SecretMasked)
	}
}

// Saving any auth.* key triggers a hot refresh; invalid config is rejected and
// rolled back (the login entry point must not be broken by a bad config).
func TestPutAuthKeyInvalidConfigRejectedAndRolledBack(t *testing.T) {
	h := newAuthTestHandler(t)
	h.SetAuthChangeCallback(func(cfg config.AuthProvidersConfig) error {
		// Simulate buildIdentityProviders rejecting: enabled but no addr
		if cfg.LDAP.Enabled && cfg.LDAP.Addr == "" {
			return context.Canceled
		}
		return nil
	})
	// enabled=true with an empty addr -> the callback errors -> 400 + key rollback
	w := putSiteKey(t, h, "auth.ldap.enabled", `true`)
	if w.Code == 200 {
		t.Fatalf("invalid config should be rejected, got 200: %s", w.Body.String())
	}
	// The key should be rolled back (no database-layer override is readable)
	if _, src, _ := settings.Current().GetString(context.Background(), settings.KeyAuthLdapEnabled); src == "database" {
		t.Fatal("key should be rolled back")
	}
}
