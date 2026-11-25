package sitesettings

// Closes out the remaining coverage gaps (coverage sweep): the GetSecurity /
// GetOutbound read-snapshot endpoints are entirely 0% (routes mounted but never
// exercised), the "empty segment" continue branch in the sec.allowIPs and domain
// suffix lists, and SendTestEmail's binding failure plus its **real send path**
// (previously everything went through the sendTestEmailFn injection seam, so the
// default implementation never ran).

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

// TestGetSecurityAndOutboundSnapshots covers both read endpoints in their default
// shape and their post-L3-override shape: the five security.* keys are all off by
// default, the sec.* lists are empty strings (no limits), ssrfProtection is false
// (no blocking), and net.* defaults to 0.
func TestGetSecurityAndOutboundSnapshots(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	rec := doSiteReq(t, r, http.MethodGet, "/api/v1/site/security", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var sec map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sec))
	assert.Equal(t, false, sec["mfaRequired"], "默认全关")
	assert.Equal(t, false, sec["passwordRequireUppercase"])
	assert.Equal(t, float64(0), sec["passwordMinLength"])

	rec = doSiteReq(t, r, http.MethodGet, "/api/v1/site/outbound", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "", out["allowPorts"], "空清单 = 不限")
	assert.Equal(t, "", out["allowIPs"])
	assert.Equal(t, "", out["domainFilter"])
	assert.Equal(t, false, out["ssrfProtection"], "默认不拦截")
	assert.Equal(t, float64(0), out["requestTimeoutMs"], "0 = 沿用调用方缺省")
	assert.Equal(t, float64(0), out["maxRetries"])
	assert.Equal(t, float64(0), out["retryBackoffMs"])
	assert.Equal(t, "default", out["sources"].(map[string]any)["sec.allowPorts"])

	// After an L3 write the snapshot should echo the real value and mark the source as database
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.ssrfProtection", `{"value":true}`)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = doSiteReq(t, r, http.MethodGet, "/api/v1/site/outbound", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, true, out["ssrfProtection"])
	assert.Equal(t, "database", out["sources"].(map[string]any)["sec.ssrfProtection"])

	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/security.mfaRequired", `{"value":true}`)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = doSiteReq(t, r, http.MethodGet, "/api/v1/site/security", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sec))
	assert.Equal(t, true, sec["mfaRequired"])
}

// TestValidateValueEmptySegments covers the "empty segment" continue branch of the
// list-type keys: consecutive commas / leading or trailing commas / all-whitespace
// items must be skipped, not rejected as invalid.
func TestValidateValueEmptySegments(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	// sec.allowIPs: empty segments are skipped (consecutive commas, a leading comma, a
	// trailing comma, whitespace-only items)
	for _, ok := range []string{
		"10.0.0.1,,10.0.0.2",
		",10.0.0.1",
		"10.0.0.1,",
		"10.0.0.1, ,10.0.0.2",
		",,",
	} {
		rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs",
			`{"value":"`+ok+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, "空分段应跳过: %q", ok)
	}

	// Skipping empty segments must not mask a real error in the same segment
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs", `{"value":"10.0.0.1,,bogus"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs", `{"value":"10.0.0.1,,10.0.0.0/999"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "非法 CIDR 仍应被拒")

	// Domain suffix list: empty segments behave the same (sec.domainFilter and
	// auth.email.domainWhitelist share validateDomainSuffixList; verify each key)
	for _, key := range []string{"sec.domainFilter", "auth.email.domainWhitelist"} {
		for _, ok := range []string{"a.com,,b.com", ",a.com", "a.com,", "a.com, ,b.com", ","} {
			rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/"+key,
				`{"value":"`+ok+`"}`)
			assert.Equal(t, http.StatusOK, rec.Code, "%s 空分段应跳过: %q", key, ok)
		}
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/"+key, `{"value":"a.com,,http://x.io"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s 空分段不得掩盖真错误", key)
	}
}

// TestSendTestEmailBindError: a body that is not valid JSON -> binding fails with
// 400 (only the "missing field" and "injection seam" paths were tested before).
func TestSendTestEmailBindError(t *testing.T) {
	r, _ := setupRouterWithDB(t)
	rec := doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email", `{"to":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestSendTestEmailRealSendPath goes through the **uninjected** sendTestEmailFn:
// SMTP points at a local connection-refused port, so the real send is guaranteed
// to fail -> the endpoint surfaces the backend error as 500. Covers the default
// implementation body (building EmailSender + WithTransport + Send), which was
// previously 0%.
func TestSendTestEmailRealSendPath(t *testing.T) {
	r, db := setupRouterWithDB(t)
	// Use the package-level default implementation as-is (the tracked injection cases
	// all restore themselves via t.Cleanup, and this file runs before
	// testemail_test.go in alphabetical order, so the starting state must be the
	// default; an earlier draft mistakenly set the seam variable to nil -- the
	// default only exists in the declaration's initializer, and setting it to nil
	// means calling a nil function and panicking).
	require.NotNil(t, sendTestEmailFn, "起点须为 default 实现")

	store := model.NewPlatformSettingModel(db)
	for k, v := range map[string]string{
		"notification.smtpHost": "127.0.0.1",
		"notification.smtpPort": "1", // reserved port: dial gets ECONNREFUSED immediately
		"notification.smtpFrom": "noreply@example.com",
	} {
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		require.NoError(t, store.Set(context.Background(), k, json.RawMessage(raw), "tester"))
	}
	settings.Current().Reload(context.Background(), store)

	rec := doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email",
		`{"to":"ops@example.com"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "真实发信失败应透出 500")
	assert.NotContains(t, rec.Body.String(), `"sent":true`)

	// An invalid recipient address is rejected before dialing (validateEmailAddress),
	// producing no external connection
	rec = doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email",
		`{"to":"not-an-email"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
