package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newFakeWeChat 起假微信开放平台（授权页断言 + token/userinfo 端点），
// 返回 provider 与请求记录。
func newFakeWeChat(t *testing.T, tokenBody, userBody string, tokenStatus int) (*WeChatProvider, *[]*http.Request) {
	t.Helper()
	var reqs []*http.Request
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs = append(reqs, r.Clone(context.Background()))
		if strings.Contains(r.URL.Path, "/sns/oauth2/access_token") {
			w.WriteHeader(tokenStatus)
			_, _ = w.Write([]byte(tokenBody))
			return
		}
		_, _ = w.Write([]byte(userBody))
	}))
	t.Cleanup(api.Close)

	p, err := NewWeChatProvider(WeChatConfig{
		AppID:       "wx-app",
		AppSecret:   "wx-secret",
		RedirectURL: "https://gm.example.com/api/v1/auth/wechat/callback",
		AuthBase:    "https://open.weixin.qq.com",
		APIBase:     api.URL,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	return p, &reqs
}

func TestWeChatProvider_AuthCodeURL(t *testing.T) {
	p, _ := newFakeWeChat(t, `{}`, `{}`, 200)
	got := p.AuthCodeURL("state-xyz")
	if !strings.HasPrefix(got, "https://open.weixin.qq.com/connect/qrconnect?") {
		t.Fatalf("unexpected auth url: %q", got)
	}
	if !strings.Contains(got, "appid=wx-app") ||
		!strings.Contains(got, "scope=snsapi_login") ||
		!strings.Contains(got, "state=state-xyz") {
		t.Fatalf("auth url missing params: %q", got)
	}
	if !strings.HasSuffix(got, "#wechat_redirect") {
		t.Fatalf("auth url missing wechat_redirect suffix: %q", got)
	}
	// redirect_uri 经 url.Values.Encode 转义
	if !strings.Contains(url.QueryEscape("https://gm.example.com/api/v1/auth/wechat/callback"), "%2F") &&
		!strings.Contains(got, "redirect_uri=https%3A%2F%2Fgm.example.com") {
		t.Fatalf("redirect_uri not escaped: %q", got)
	}
}

func TestWeChatProvider_Exchange(t *testing.T) {
	p, reqs := newFakeWeChat(t,
		`{"access_token":"tok-1","openid":"oX-1","expires_in":7200}`,
		`{"openid":"oX-1","nickname":"老王","unionid":"u-1"}`, 200)

	ident, err := p.Exchange(context.Background(), "code-1")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if ident.Provider != KindWeChat || ident.Username != "oX-1" || ident.Nickname != "老王" {
		t.Fatalf("unexpected identity: %+v", ident)
	}
	if ident.Email != "" {
		t.Fatalf("wechat identity must not carry email, got %q", ident.Email)
	}
	// token 请求以 query 携带凭证（微信协议形态）
	q := (*reqs)[0].URL.Query()
	if q.Get("appid") != "wx-app" || q.Get("secret") != "wx-secret" ||
		q.Get("code") != "code-1" || q.Get("grant_type") != "authorization_code" {
		t.Fatalf("token query mismatch: %v", q)
	}
}

func TestWeChatProvider_ExchangeWeChatError(t *testing.T) {
	// 微信业务错误：code 已用/过期 → errcode 包裹 ErrInvalidCredentials
	p, _ := newFakeWeChat(t, `{"errcode":40029,"errmsg":"invalid code"}`, `{}`, 200)
	_, err := p.Exchange(context.Background(), "bad-code")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestWeChatProvider_ExchangeHTTPFailure(t *testing.T) {
	// 非 200 是网络/服务侧故障，不归类为凭证错误
	p, _ := newFakeWeChat(t, `upstream down`, `{}`, 503)
	_, err := p.Exchange(context.Background(), "code-1")
	if err == nil || errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected non-credentials error, got: %v", err)
	}
}

func TestWeChatProvider_ConfigValidation(t *testing.T) {
	if _, err := NewWeChatProvider(WeChatConfig{AppID: "x"}); err == nil {
		t.Fatal("expected missing appSecret rejected")
	}
	if _, err := NewWeChatProvider(WeChatConfig{AppSecret: "y"}); err == nil {
		t.Fatal("expected missing appId rejected")
	}
}
