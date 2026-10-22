package auth

// 覆盖率巡检补测（auth 包 handler.go 53 块 / service.go 18 块残余中的大
// 头）：此前 auth 包的服务层 OAuth 用例只覆盖 OIDC 与微信/自定义 OAuth 的
// **回调**，GitHub 一路（WithGitHubProvider 装配 + AuthCodeURL + SuccessURL
// + LoginCallback）完全没被调用，三个「登录入口」handler（GitHubLogin/
// WeChatLogin/GenericOAuthLogin）与三个「回调」handler 也从无直调用例；
// 注册/验证邮箱/重发验证三个 handler 同样只有服务层覆盖。
//
// 本文件只补测试、不改源码，逐 handler 锁四态：绑定失败 / 未启用 /
// 业务拒绝 / 成功。夹具复用包内既有 newOIDCService、registerFixture、
// verificationFixture、newAuthTestContext、hookVerificationSender。

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

// ---- 注册 handler：403 关闭态与 400 业务校验的分野 ----

func TestHandler_Register_FullContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	post := func(h *Handler, body string) *httptest.ResponseRecorder {
		c, rec := newAuthTestContext(http.MethodPost, "/api/v1/auth/register", body)
		h.Register(c)
		return rec
	}

	// 开关关闭（默认）：合法负载仍被业务拒 → 403 + registration_disabled
	disabled := NewHandler(registerFixture(t, config.AuthProvidersConfig{}))
	rec := post(disabled, `{"username":"newuser1","password":"Str0ngPass!x"}`)
	assertHTTPStatus(t, rec, 403)
	assertErrorCode(t, rec, "registration_disabled")

	// 开关开启：弱密码 → 400 业务校验（不是 403）
	enabled := NewHandler(registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	}))
	rec = post(enabled, `{"username":"newuser2","password":"123"}`)
	assertHTTPStatus(t, rec, 400)
	assert.Contains(t, rec.Body.String(), "密码长度至少为8个字符", "开关开启时是业务校验失败")

	// 绑定失败（非法 JSON）→ 400 参数错误（先于开关/业务判定）
	rec = post(disabled, `{"username":`)
	assertHTTPStatus(t, rec, 400)
	assert.Contains(t, rec.Body.String(), "参数错误")

	// 成功：回 username + nickname（昵称已 trim）
	rec = post(enabled, `{"username":"newuser3","password":"Str0ngPass!x","nickname":"  新号  "}`)
	assertHTTPStatus(t, rec, 200)
	assert.Contains(t, rec.Body.String(), `"username":"newuser3"`)
	assert.Contains(t, rec.Body.String(), "新号")
}

// ---- 验证邮箱 handler：空/无效/有效三态 ----

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

	assertHTTPStatus(t, verify(""), 400)                // token 缺省
	assertHTTPStatus(t, verify("?token=deadbeef"), 400) // 未知令牌（不区分原因）

	rec := verify("?token=" + url.QueryEscape(token))
	assertHTTPStatus(t, rec, 200)
	assert.Contains(t, rec.Body.String(), `"verified":true`)
}

// ---- 重发验证 handler：绑定失败 / 参数缺失 / 真实发信 / 防枚举静默 ----

func TestHandler_ResendVerification_FullContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _, _ := verificationFixture(t)
	sent, restore := hookVerificationSender(t, nil)
	defer restore()

	// 直接建号（绕开注册自动签发），保证重发不在频控窗口内
	require.NoError(t, svc.adminModel.Create(context.Background(),
		&model.Admin{Username: "resend1", Email: "resend1@example.com", Status: 1}, "Str0ngPass!x"))
	before := len(*sent)

	resend := func(body string) *httptest.ResponseRecorder {
		c, rec := newAuthTestContext(http.MethodPost, "/api/v1/auth/resend-verification", body)
		NewHandler(svc).ResendVerification(c)
		return rec
	}

	assertHTTPStatus(t, resend(`{"username":`), 400)                                     // 绑定失败
	assertHTTPStatus(t, resend(`{"username":"resend1"}`), 400)                           // 缺 email → 400
	assertHTTPStatus(t, resend(`{"username":"ghost","email":"ghost@example.com"}`), 200) // 防枚举：静默 200

	rec := resend(`{"username":"resend1","email":"resend1@example.com"}`)
	assertHTTPStatus(t, rec, 200)
	assert.Contains(t, rec.Body.String(), `"resent":true`)
	assert.Len(t, *sent, before+1, "真实匹配且未验证时确实发信")

	// 频控窗口内再发：静默成功但不重复发信
	rec = resend(`{"username":"resend1","email":"resend1@example.com"}`)
	assertHTTPStatus(t, rec, 200)
	assert.Len(t, *sent, before+1, "频控内静默：不重复发信")
}

// ---- GitHub 一路服务层装配（此前完全无调用）----

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

	// Exchange 失败：kind label 进错误文案
	failing := &fakeOAuthProvider{err: errors.New("bad_verification_code")}
	s2 := newOIDCService(t, nil, "").WithGitHubProvider(failing, nil, "")
	_, err = s2.GitHubLoginCallback(context.Background(), "code", s2.newOIDCState(), &LoginRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub 登录失败")

	// 未启用：授权 URL 与回调都报「未启用」
	s3 := newOIDCService(t, nil, "")
	_, err = s3.GitHubAuthCodeURL()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub 登录未启用")
	_, err = s3.GitHubLoginCallback(context.Background(), "code", s3.newOIDCState(), &LoginRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub 登录未启用")
	assert.Empty(t, s3.GitHubSuccessURL())
}

// ---- 三个「登录入口」handler：未启用 400 / 已启用 302 ----

func TestHandler_OAuthLoginEndpoints_FullContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 未启用 → 400 且不重定向
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

	// 已启用 → 302 且 Location 指向授权地址
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

// ---- 三个「回调」handler：JSON 200 / 配了 loginSuccessUrl 则 302 携 token /
// 无效 state 401 ----

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

	// 未配 loginSuccessUrl → 返回 JSON 响应体
	for name, call := range callbacks {
		c, rec := newAuthTestContext(http.MethodGet,
			"/"+name+"/callback?code=abc&state="+s.newOIDCState(), "")
		call(c)
		assertHTTPStatus(t, rec, 200)
		assert.Contains(t, rec.Body.String(), `"token"`)
	}

	// 配了 loginSuccessUrl → 302 且 token 进 query
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

	// state 非法 → 401
	for name, call := range callbacks {
		c, rec := newAuthTestContext(http.MethodGet, "/"+name+"/callback?code=abc&state=bogus", "")
		call(c)
		assertHTTPStatus(t, rec, 401)
	}
}
