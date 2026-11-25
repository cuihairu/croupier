package sitesettings

// Outbound security and restriction keys (OPEN-ISSUES #56): list format validation
// (ports 1-65535, IP/CIDR, domain suffixes forbidding protocol/path), empty string
// = no limits; ssrfProtection goes through bool validation (the validateValue
// general path).

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutKey_SecOutboundKeys(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	// allowPorts: reject invalid fragments (out of range / non-numeric / empty fragment)
	for _, bad := range []string{"70000", "443,bad", "0", "443,,8080"} {
		rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowPorts", `{"value":"`+bad+`"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code, bad)
		assert.Contains(t, rec.Body.String(), "1-65535")
	}

	// allowPorts: valid lists and the empty string (clearing = no limits)
	for _, ok := range []string{"443", "443, 8080", ""} {
		rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowPorts", `{"value":"`+ok+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, ok)
	}

	// allowIPs: reject an invalid IP / invalid CIDR
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs", `{"value":"10.0.0.0/8"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs", `{"value":"notanip"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "非法")
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs", `{"value":"10.0.0.0/99"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// domainFilter: reject protocol/path; a domain suffix and the empty string pass
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.domainFilter", `{"value":"http://example.com"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "不带协议")
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.domainFilter", `{"value":"example.com/x"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	for _, ok := range []string{"example.com", "example.com, foo.io", ""} {
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.domainFilter", `{"value":"`+ok+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, ok)
	}

	// ssrfProtection: a bool key (invalid types are rejected, a valid boolean passes)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.ssrfProtection", `{"value":"on"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.ssrfProtection", `{"value":true}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Undeclared sec.* keys are still rejected
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.unknown", `{"value":"x"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// net.* outbound call policy (OPEN-ISSUES #57): general int validation
	// (negatives/out-of-range are rejected, 0 = keep the caller's default)
	for key := range map[string]struct{}{
		"net.requestTimeoutMs": {},
		"net.maxRetries":       {},
		"net.retryBackoffMs":   {},
	} {
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/"+key, `{"value":-1}`)
		require.Equal(t, http.StatusBadRequest, rec.Code, key)
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/"+key, `{"value":70000}`)
		require.Equal(t, http.StatusBadRequest, rec.Code, key)
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/"+key, `{"value":3}`)
		assert.Equal(t, http.StatusOK, rec.Code, key)
	}
}
