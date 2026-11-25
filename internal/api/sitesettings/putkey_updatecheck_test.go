package sitesettings

// The write surface of the L3 key system.updateCheckUrl added by system
// maintenance (OPEN-ISSUES #52): the key is whitelisted, its value must be an
// http(s) URL or an empty string that clears it; non-URLs are rejected.

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutKey_SystemUpdateCheckURL(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	// A non-URL is rejected
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/system.updateCheckUrl", `{"value":"not-a-url"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "http(s) URL")

	// A valid URL lands in the database
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/system.updateCheckUrl", `{"value":"https://releases.example.com/latest.json"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	// An empty string = clear the update source (allowed, back to the unconfigured semantics)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/system.updateCheckUrl", `{"value":""}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}
