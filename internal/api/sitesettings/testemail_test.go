package sitesettings

// Tests for the send-test-email endpoint (OPEN-ISSUES #55 debt / #51c): to is
// required, an unconfigured SMTP answers 400, the injection seam records the
// recipient without actually sending, and the domain whitelist key write is
// validated.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendTestEmail(t *testing.T) {
	r, db := setupRouterWithDB(t)
	settings.ResetForTest()
	t.Cleanup(settings.ResetForTest)

	// SMTP not configured -> 400 (avoids EmailSender's silent false success on an empty host)
	rec := doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email", `{"to":"a@example.com"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "未配置")

	// to is required
	rec = doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email", `{}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// Inject the seam: record the recipient and verify the real call and the response shape
	var sent []string
	orig := sendTestEmailFn
	sendTestEmailFn = func(_ context.Context, to string) error {
		sent = append(sent, to)
		return nil
	}
	t.Cleanup(func() { sendTestEmailFn = orig })

	// Even with the seam injected, host must still be non-empty -- seed the SMTP
	// host through L3 (the store handle is unreachable: rather than swap in a
	// Router with settings initialization -- this file's harness has no settings
	// database -- use an AuthSnapshot-independent path: NotifySMTP reads
	// settings.Current(), and before initialization Current() returns nil ->
	// empty NotifySMTP config -> 400). Under the injected seam, to pass the
	// host check settings must be initialized and the host seeded.
	settings.ResetForTest()
	store := model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	require.NoError(t, store.Set(context.Background(), "notification.smtpHost", json.RawMessage(`"smtp.example.com"`), "tester"))
	settings.Current().Reload(context.Background(), store)

	rec = doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email", `{"to":"ops@example.com"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"sent":true`)
	require.Len(t, sent, 1)
	assert.Equal(t, "ops@example.com", sent[0])

	// A send failure surfaces the backend error
	sendTestEmailFn = func(_ context.Context, _ string) error { return assert.AnError }
	rec = doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email", `{"to":"ops@example.com"}`)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestPutKey_EmailDomainWhitelist(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	// Reject protocol/path (same rules as sec.domainFilter)
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/auth.email.domainWhitelist", `{"value":"http://example.com"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// Valid lists and the empty string
	for _, ok := range []string{"example.com", "example.com, foo.io", ""} {
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/auth.email.domainWhitelist", `{"value":"`+ok+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, ok)
	}

	// aliasRestriction: a bool key
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/auth.email.aliasRestriction", `{"value":"yes"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/auth.email.aliasRestriction", `{"value":true}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}
