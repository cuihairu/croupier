package auth

// Coverage-audit follow-up tests (the bulk of the auth package's remaining 53
// blocks in handler.go / 18 blocks in service.go): previously the auth
// package's service-layer OAuth cases covered only OIDC and the WeChat/custom
// OAuth **callbacks**; the whole GitHub path (WithGitHubProvider wiring +
// AuthCodeURL + SuccessURL + LoginCallback) was never invoked, and the three
// "login entry" handlers (GitHubLogin/WeChatLogin/GenericOAuthLogin) plus the
// three "callback" handlers had no direct-call cases either; the register /
// verify-email / resend-verification handlers likewise had only service-layer
// coverage.
//
// This file adds tests only and changes no source. Each handler is pinned
// across four states: bind failure / not enabled / business rejection /
// success. Fixtures reuse the existing in-package newOIDCService,
// registerFixture, verificationFixture, newAuthTestContext, and
// hookVerificationSender.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- register handler: where the 403 disabled state ends and 400 business
// validation begins ----

func TestHandler_Register_FullContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	post := func(h *Handler, body string) *httptest.ResponseRecorder {
		c, rec := newAuthTestContext(http.MethodPost, "/api/v1/auth/register", body)
		h.Register(c)
		return rec
	}

	// switch off (default): a valid payload is still rejected by business
	// rules → 403 + registration_disabled
	disabled := NewHandler(registerFixture(t, config.AuthProvidersConfig{}))
	rec := post(disabled, `{"username":"newuser1","password":"Str0ngPass!x"}`)
	assertHTTPStatus(t, rec, 403)
	assertErrorCode(t, rec, "registration_disabled")

	// switch on: weak password → 400 business validation (not 403)
	enabled := NewHandler(registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	}))
	rec = post(enabled, `{"username":"newuser2","password":"123"}`)
	assertHTTPStatus(t, rec, 400)
	assert.Contains(t, rec.Body.String(), "密码长度至少为8个字符", "开关开启时是业务校验失败")

	// bind failure (invalid JSON) → 400 bad parameters (ahead of the switch
	// and business checks)
	rec = post(disabled, `{"username":`)
	assertHTTPStatus(t, rec, 400)
	assert.Contains(t, rec.Body.String(), "参数错误")

	// success: echoes username + nickname (nickname already trimmed)
	rec = post(enabled, `{"username":"newuser3","password":"Str0ngPass!x","nickname":"  新号  "}`)
	assertHTTPStatus(t, rec, 200)
	assert.Contains(t, rec.Body.String(), `"username":"newuser3"`)
	assert.Contains(t, rec.Body.String(), "新号")
}

// ---- verify-email handler: the empty / invalid / valid three states ----

func TestHandler_VerifyEmail_FullContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _, _ := verificationFixture(t)
	sent, restore := hookVerificationSender(t, nil)
	defer restore()

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "verify1", Password: "Str0ngPass!x", Email: "verify1@example.com",
	})
	require.NoError(t, err)
	require.NotEmpty(t, *sent, "注册即签发验证邮件")
	token := (*sent)[0].token

	verify := func(q string) *httptest.ResponseRecorder {
		c, rec := newAuthTestContext(http.MethodGet, "/api/v1/auth/verify-email"+q, "")
		NewHandler(svc).VerifyEmail(c)
		return rec
	}

	assertHTTPStatus(t, verify(""), 400)                // token absent
	assertHTTPStatus(t, verify("?token=deadbeef"), 400) // unknown token (reason not distinguished)

	rec := verify("?token=" + url.QueryEscape(token))
	assertHTTPStatus(t, rec, 200)
	assert.Contains(t, rec.Body.String(), `"verified":true`)
}

// ---- resend-verification handler: bind failure / missing parameters / real
// mail-out / anti-enumeration silence ----

func TestHandler_ResendVerification_FullContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _, _ := verificationFixture(t)
	sent, restore := hookVerificationSender(t, nil)
	defer restore()

	// create the account directly (bypassing registration's automatic issue)
	// so the resend is not inside the rate-limit window
	require.NoError(t, svc.adminModel.Create(context.Background(),
		&model.Admin{Username: "resend1", Email: "resend1@example.com", Status: 1}, "Str0ngPass!x"))
	before := len(*sent)

	resend := func(body string) *httptest.ResponseRecorder {
		c, rec := newAuthTestContext(http.MethodPost, "/api/v1/auth/resend-verification", body)
		NewHandler(svc).ResendVerification(c)
		return rec
	}

	assertHTTPStatus(t, resend(`{"username":`), 400)                                     // bind failure
	assertHTTPStatus(t, resend(`{"username":"resend1"}`), 400)                           // missing email → 400
	assertHTTPStatus(t, resend(`{"username":"ghost","email":"ghost@example.com"}`), 200) // anti-enumeration: silent 200

	rec := resend(`{"username":"resend1","email":"resend1@example.com"}`)
	assertHTTPStatus(t, rec, 200)
	assert.Contains(t, rec.Body.String(), `"resent":true`)
	assert.Len(t, *sent, before+1, "真实匹配且未验证时确实发信")

	// resending inside the rate-limit window: silent success, no duplicate
	// mail
	rec = resend(`{"username":"resend1","email":"resend1@example.com"}`)
	assertHTTPStatus(t, rec, 200)
	assert.Len(t, *sent, before+1, "频控内静默：不重复发信")
}

// ---- the GitHub path's service-layer wiring (previously never invoked) ----

func TestService_GitHubProviderWiring(t *testing.T) {
	fake := &fakeOAuthProvider{
		authURL: "https://github.example.com/login/oauth/authorize",
		ident:   &identity.Identity{Provider: identity.KindGitHub, Username: "octocat"},
	}
	s := newOIDCService(t, nil, "").WithGitHubProvider(fake, []string{"viewer"}, "https://gm.example.com/done")

	assert.True(t, s.GitHubEnabled())
	assert.Equal(t, "https://gm.example.com/done", s.GitHubSuccessURL())

	url, err := s.GitHubAuthCodeURL()
	require.NoError(t, err)
	assert.Contains(t, url, "https://github.example.com/login/oauth/authorize?state=")

	resp, err := s.GitHubLoginCallback(context.Background(), "code", s.newOIDCState(), &LoginRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "octocat", resp.User.Username)

	// Exchange failure: the kind label lands in the error text
	failing := &fakeOAuthProvider{err: errors.New("bad_verification_code")}
	s2 := newOIDCService(t, nil, "").WithGitHubProvider(failing, nil, "")
	_, err = s2.GitHubLoginCallback(context.Background(), "code", s2.newOIDCState(), &LoginRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub 登录失败")

	// not enabled: both the authorization URL and the callback report "not
	// enabled"
	s3 := newOIDCService(t, nil, "")
	_, err = s3.GitHubAuthCodeURL()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub 登录未启用")
	_, err = s3.GitHubLoginCallback(context.Background(), "code", s3.newOIDCState(), &LoginRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub 登录未启用")
	assert.Empty(t, s3.GitHubSuccessURL())
}

// ---- the three "login entry" handlers: 400 when disabled / 302 when enabled ----

func TestHandler_OAuthLoginEndpoints_FullContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// disabled → 400 and no redirect
	off := NewHandler(newOIDCService(t, nil, ""))
	for name, call := range map[string]func(*gin.Context){
		"github":       off.GitHubLogin,
		"wechat":       off.WeChatLogin,
		"genericoauth": off.GenericOAuthLogin,
	} {
		c, rec := newAuthTestContext(http.MethodGet, "/"+name+"/login", "")
		call(c)
		assertHTTPStatus(t, rec, 400)
		assert.Contains(t, rec.Body.String(), "未启用")
	}

	// enabled → 302 with Location pointing at the authorization URL
	fake := &fakeOAuthProvider{
		authURL: "https://idp.example.com/authorize",
		ident:   &identity.Identity{Provider: identity.KindOIDC, Username: "alice"},
	}
	s := newOIDCService(t, nil, "").
		WithGitHubProvider(fake, nil, "").
		WithWeChatProvider(fake, nil, "").
		WithGenericOAuthProvider(fake, nil, "")
	on := NewHandler(s)

	for name, call := range map[string]func(*gin.Context){
		"github":       on.GitHubLogin,
		"wechat":       on.WeChatLogin,
		"genericoauth": on.GenericOAuthLogin,
	} {
		c, rec := newAuthTestContext(http.MethodGet, "/"+name+"/login", "")
		call(c)
		assertHTTPStatus(t, rec, http.StatusFound)
		assert.Contains(t, rec.Header().Get("Location"), "https://idp.example.com/authorize?state=")
	}
}

// ---- the three "callback" handlers: JSON 200 / 302 carrying the token when
// loginSuccessUrl is configured / 401 on an invalid state ----

func TestHandler_OAuthCallbackEndpoints_FullContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &fakeOAuthProvider{
		authURL: "https://idp.example.com/authorize",
		ident:   &identity.Identity{Provider: identity.KindOIDC, Username: "alice", Email: "alice@example.com"},
	}
	s := newOIDCService(t, nil, "").
		WithGitHubProvider(fake, nil, "").
		WithWeChatProvider(fake, nil, "").
		WithGenericOAuthProvider(fake, nil, "")
	h := NewHandler(s)

	callbacks := map[string]func(*gin.Context){
		"github":       h.GitHubCallback,
		"wechat":       h.WeChatCallback,
		"genericoauth": h.GenericOAuthCallback,
	}

	// no loginSuccessUrl configured → returns a JSON response body
	for name, call := range callbacks {
		c, rec := newAuthTestContext(http.MethodGet,
			"/"+name+"/callback?code=abc&state="+s.newOIDCState(), "")
		call(c)
		assertHTTPStatus(t, rec, 200)
		assert.Contains(t, rec.Body.String(), `"token"`)
	}

	// loginSuccessUrl configured → 302 with the token in the query
	withJump := newOIDCService(t, nil, "")
	withJump.WithGitHubProvider(fake, nil, "https://gm.example.com/done?from=github")
	c, rec := newAuthTestContext(http.MethodGet,
		"/github/callback?code=abc&state="+withJump.newOIDCState(), "")
	NewHandler(withJump).GitHubCallback(c)
	assertHTTPStatus(t, rec, http.StatusFound)
	loc, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "https://gm.example.com/done", loc.Scheme+"://"+loc.Host+loc.Path)
	assert.NotEmpty(t, loc.Query().Get("token"), "跳转须携带签发的 token")
	assert.Equal(t, "github", loc.Query().Get("from"), "原有 query 保留")

	// an invalid state → 401
	for name, call := range callbacks {
		c, rec := newAuthTestContext(http.MethodGet, "/"+name+"/callback?code=abc&state=bogus", "")
		call(c)
		assertHTTPStatus(t, rec, 401)
	}
}
