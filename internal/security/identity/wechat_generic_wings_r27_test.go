package identity

// 覆盖率巡检第二十七轮（wt-api）：wechat.go / genericoauth.go（964e40b
// 落地）残余错误翼——两 provider 的 Kind 标识、WeChat 构造缺省 base 回落、
// Exchange / apiGet / fetchUserInfo 的全错误域（token 换取失败、userinfo
// 非 200 / 坏 JSON、NewRequest 非法 URL、传输失败、声明 Content-Length 后
// 中断连接的体读取错误、openid 缺失回退与双缺失兜底）。风格沿用本包
// stdlib testing；错误注入全部走 httptest / 原生 listener，零真实外联。

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// startTruncatingServer 起一个「声明 Content-Length 远大于实发即断连」的
// 原生服务：客户端 io.ReadAll 必得 unexpected EOF——ReadAll 错误分支的
// 唯一确定性注入形态。
func startTruncatingServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				buf := make([]byte, 4096)
				_, _ = c.Read(buf) // 丢弃请求头，立即应答
				_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2048\r\nContent-Type: application/json\r\n\r\n{\"partial\":"))
			}(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return "http://" + ln.Addr().String()
}

// TestProviderKindIdentifiers 两 provider 的类型标识（此前 0%）+ WeChat
// 构造缺省 base 回落（AuthBase/APIBase 空串 → 官方域名）。
func TestProviderKindIdentifiers(t *testing.T) {
	gen, err := NewGenericOAuthProvider(GenericOAuthConfig{
		ClientID: "cid", ClientSecret: "cs",
		AuthURL:     "https://sso.example.com/authorize",
		TokenURL:    "https://sso.example.com/token",
		UserInfoURL: "https://sso.example.com/user",
	})
	if err != nil {
		t.Fatalf("new generic provider: %v", err)
	}
	if gen.Kind() != KindGenericOAuth {
		t.Fatalf("generic kind = %q", gen.Kind())
	}

	wx, err := NewWeChatProvider(WeChatConfig{AppID: "wx-app", AppSecret: "wx-secret"})
	if err != nil {
		t.Fatalf("new wechat provider: %v", err)
	}
	if wx.Kind() != KindWeChat {
		t.Fatalf("wechat kind = %q", wx.Kind())
	}
	// 缺省 base 回落：认证页指向官方开放平台域
	if wx.authBase != "https://open.weixin.qq.com" || wx.apiBase != "https://api.weixin.qq.com" {
		t.Fatalf("default bases = %q / %q", wx.authBase, wx.apiBase)
	}
	if !strings.HasPrefix(wx.AuthCodeURL("s"), "https://open.weixin.qq.com/connect/qrconnect?") {
		t.Fatalf("auth url not on default base: %q", wx.AuthCodeURL("s"))
	}
}

// TestGenericOAuth_ExchangeWings Exchange 两层失败：token 端点换取失败、
// token 成功后 userinfo 拉取失败（非 200 透传）。
func TestGenericOAuth_ExchangeWings(t *testing.T) {
	// token 端点即失败（closed server → 传输层错误）
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	dead.Close()
	p, err := NewGenericOAuthProvider(GenericOAuthConfig{
		ClientID: "cid", ClientSecret: "cs",
		AuthURL:     "https://sso.example.com/authorize",
		TokenURL:    dead.URL,
		UserInfoURL: "https://sso.example.com/user",
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if _, err := p.Exchange(context.Background(), "code"); err == nil || !strings.Contains(err.Error(), "generic-oauth code exchange") {
		t.Fatalf("token exchange failure not surfaced: %v", err)
	}

	// token 成功、userinfo 500 → fetchUserInfo 错误经 Exchange 透传
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","token_type":"bearer"}`))
	}))
	defer tokenSrv.Close()
	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer userSrv.Close()
	p2, err := NewGenericOAuthProvider(GenericOAuthConfig{
		ClientID: "cid", ClientSecret: "cs",
		AuthURL:     "https://sso.example.com/authorize",
		TokenURL:    tokenSrv.URL,
		UserInfoURL: userSrv.URL,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if _, err := p2.Exchange(context.Background(), "code"); err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("userinfo failure not surfaced: %v", err)
	}
}

// TestGenericOAuth_FetchUserInfoWings fetchUserInfo 直测四翼：非法 URL 的
// NewRequest 失败、传输失败、体读取中断、坏 JSON 解码失败。
func TestGenericOAuth_FetchUserInfoWings(t *testing.T) {
	p := &GenericOAuthProvider{userInfoURL: "http://exa mple.com/user", client: http.DefaultClient}
	if _, err := p.fetchUserInfo(context.Background(), "at"); err == nil {
		t.Fatal("invalid userinfo url must fail NewRequest")
	}

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	dead.Close()
	p2 := &GenericOAuthProvider{userInfoURL: dead.URL + "/user", client: http.DefaultClient}
	if _, err := p2.fetchUserInfo(context.Background(), "at"); err == nil || !strings.Contains(err.Error(), "generic-oauth userinfo") {
		t.Fatalf("transport failure not surfaced: %v", err)
	}

	trunc := &GenericOAuthProvider{userInfoURL: startTruncatingServer(t), client: http.DefaultClient}
	if _, err := trunc.fetchUserInfo(context.Background(), "at"); err == nil {
		t.Fatal("truncated body must fail ReadAll")
	}

	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer badJSON.Close()
	p3 := &GenericOAuthProvider{userInfoURL: badJSON.URL, client: http.DefaultClient}
	if _, err := p3.fetchUserInfo(context.Background(), "at"); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("bad json not surfaced: %v", err)
	}
}

// TestWeChat_ExchangeWings Exchange 深层翼：token 响应缺 access_token/
// openid、token 成功但 userinfo 拉取失败。
//
// 登记不可达（不造假用例、不删防御分支）：Exchange 尾部的 openid 回退
// （134）与双缺失兜底（137）——上方 122 守卫已按 TrimSpace 拒绝空
// access_token/openid，通过后 TrimSpace(token.OpenID) 必非空，134 的
// `if openID == ""` 恒假、137 恒不可达（自证性双保险，同 openapi
// unboundFunctionID "fn-" 前缀豁免的构造）。openid 语义上由 token 端点
// 提供，userinfo 的 openid 仅是冗余字段。
func TestWeChat_ExchangeWings(t *testing.T) {
	// errcode=0 但缺 access_token/openid
	p, _ := newFakeWeChat(t, `{"errcode":0}`, `{}`, http.StatusOK)
	if _, err := p.Exchange(context.Background(), "c"); err == nil || !strings.Contains(err.Error(), "response missing access_token/openid") {
		t.Fatalf("missing token fields not surfaced: %v", err)
	}

	// token 成功、userinfo 500
	userFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "access_token") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"at","openid":"oid"}`))
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer userFail.Close()
	wx, err := NewWeChatProvider(WeChatConfig{
		AppID: "wx-app", AppSecret: "s", APIBase: userFail.URL,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if _, err := wx.Exchange(context.Background(), "c"); err == nil || !strings.Contains(err.Error(), "wechat userinfo") {
		t.Fatalf("userinfo failure not surfaced: %v", err)
	}
}

// TestWeChat_APIGetWings apiGet 直测四翼：非法 URL 的 NewRequest 失败、
// 传输失败、体读取中断、坏 JSON 解码失败。
func TestWeChat_APIGetWings(t *testing.T) {
	var out map[string]any

	p := &WeChatProvider{apiBase: "http://exa mple.com", client: http.DefaultClient}
	if err := p.apiGet(context.Background(), "/x", &out); err == nil {
		t.Fatal("invalid apiBase must fail NewRequest")
	}

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	dead.Close()
	p2 := &WeChatProvider{apiBase: dead.URL, client: http.DefaultClient}
	if err := p2.apiGet(context.Background(), "/x", &out); err == nil {
		t.Fatal("transport failure must surface")
	}

	trunc := &WeChatProvider{apiBase: startTruncatingServer(t), client: http.DefaultClient}
	if err := trunc.apiGet(context.Background(), "/x", &out); err == nil {
		t.Fatal("truncated body must fail ReadAll")
	}

	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer badJSON.Close()
	p3 := &WeChatProvider{apiBase: badJSON.URL, client: http.DefaultClient}
	if err := p3.apiGet(context.Background(), "/x", &out); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("bad json not surfaced: %v", err)
	}
}
