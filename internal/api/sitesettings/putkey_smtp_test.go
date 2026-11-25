package sitesettings

// SMTP transport detail keys (OPEN-ISSUES #55): encryption/authType enum
// validation, empty string = restore auto/default; insecureSkipVerify goes through
// bool validation (the validateValue general path).

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutKey_SmtpTransportKeys(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	// encryption: reject an invalid value
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpEncryption", `{"value":"tls-wrong"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "none / ssl / starttls")

	// encryption: valid values and the empty string
	for _, v := range []string{"none", "ssl", "starttls", ""} {
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpEncryption", `{"value":"`+v+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, v)
	}

	// authType: invalid value rejected + valid values
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpAuthType", `{"value":"ntlm"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "plain / login")

	for _, v := range []string{"plain", "login", ""} {
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpAuthType", `{"value":"`+v+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, v)
	}

	// insecureSkipVerify: a bool key (invalid types are rejected, a valid boolean passes)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpInsecureSkipVerify", `{"value":"yes"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpInsecureSkipVerify", `{"value":true}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}
