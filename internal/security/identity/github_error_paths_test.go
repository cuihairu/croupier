package identity

// 覆盖率巡检补测（github.go 残余 14 块）：GitHub OAuth2 提供方的失败路径与
// 两处兜底默认值。此前用例只覆盖到成功换用户与「profile 无 login」，
// 以下按错误发生的层次逐层锁死：
//  1. 构造层：Kind() 与 APIBase 缺省回落 api.github.com；
//  2. 换码层：令牌端点拒绝（code exchange 包装）；
//  3. REST 层——apiGet 的四翼依次为：URL 构造失败（畸形 APIBase）、
//     传输失败（无监听端口）、响应体中途断开（读失败）、非 200（含
//     truncateForLog 的超长/未超长两翼）、200 但非法 JSON；
//  4. 邮箱兜底：/user/emails 失败与「无 primary 标记」均静默回空。
// 令牌端点恒由 httptest 承担，APIBase 指向畸形/死端口时也不触网。

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// githubTokenServer 起一个只提供令牌端点的假 GitHub（换码恒成功），
// 返回其地址与指向该地址的 Provider 构造参数。
func githubTokenServer(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok-err","token_type":"bearer"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// githubProviderWith 构造「令牌端点成功 + API 指向 apiBase」的 Provider。
func githubProviderWith(t *testing.T, authBase, apiBase string) *GitHubProvider {
	t.Helper()
	p, err := NewGitHubProvider(GitHubConfig{
		ClientID:     "cid",
		ClientSecret: "secret",
		RedirectURL:  "https://gm.example.com/api/v1/auth/github/callback",
		AuthBase:     authBase,
		APIBase:      apiBase,
	})
	require.NoError(t, err)
	return p
}

// 构造层兜底：Kind() 是身份分派依据（此前无任何用例调用）；APIBase 缺省
// 回落官方 api.github.com（自建实例才覆盖）。
func TestGitHubProvider_KindAndDefaultAPIBase(t *testing.T) {
	p, err := NewGitHubProvider(GitHubConfig{ClientID: "cid", ClientSecret: "secret"})
	require.NoError(t, err)
	assert.Equal(t, KindGitHub, p.Kind())
	assert.Equal(t, "https://api.github.com", p.apiBase, "APIBase 缺省回落官方端点")
	assert.True(t, strings.HasPrefix(p.oauth2Config.Endpoint.TokenURL, "https://github.com"),
		"未给 AuthBase 时端点取 x/oauth2 github 常量，实际 %q", p.oauth2Config.Endpoint.TokenURL)

	// 自建实例：AuthBase/APIBase 覆盖（尾斜杠被 TrimRight 归一）。
	custom := githubProviderWith(t, "https://ghe.example.com/", "https://ghe.example.com/api/v3/")
	assert.Equal(t, "https://ghe.example.com", custom.authBase)
	assert.Equal(t, "https://ghe.example.com/api/v3", custom.apiBase)
	assert.Equal(t, "https://ghe.example.com/login/oauth/authorize", custom.oauth2Config.Endpoint.AuthURL)
}

// 换码层：令牌端点 400 → code exchange 错误包装（不触达 REST 层）。
func TestGitHubProvider_Exchange_CodeExchangeFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad_verification_code"}`))
	})
	auth := httptest.NewServer(mux)
	t.Cleanup(auth.Close)

	p := githubProviderWith(t, auth.URL, auth.URL)
	id, err := p.Exchange(context.Background(), "stale-code")
	require.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "github code exchange")
}

// REST 层第 1 翼：APIBase 畸形 → NewRequest 构造失败。
func TestGitHubProvider_Exchange_MalformedAPIBaseURL(t *testing.T) {
	auth := githubTokenServer(t)
	p := githubProviderWith(t, auth, "://bad-base")

	id, err := p.Exchange(context.Background(), "code")
	require.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "github fetch user")
}

// REST 层第 2 翼：APIBase 指向无监听端口 → client.Do 传输失败。
func TestGitHubProvider_Exchange_TransportFailure(t *testing.T) {
	auth := githubTokenServer(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	dead := ln.Addr().String()
	require.NoError(t, ln.Close())

	p := githubProviderWith(t, auth, "http://"+dead)
	id, err := p.Exchange(context.Background(), "code")
	require.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "github fetch user")
}

// REST 层第 3 翼：声明 Content-Length 后断连 → 读体失败（200 也一样）。
func TestGitHubProvider_Exchange_BodyReadFailure(t *testing.T) {
	auth := githubTokenServer(t)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, buf, hijackErr := w.(http.Hijacker).Hijack()
		if hijackErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 4096\r\n\r\n")
		_, _ = buf.WriteString(`{"login":"alice"}`)
		_ = buf.Flush()
	}))
	t.Cleanup(api.Close)

	p := githubProviderWith(t, auth, api.URL)
	id, err := p.Exchange(context.Background(), "code")
	require.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "github fetch user")
	assert.Contains(t, err.Error(), "unexpected EOF")
}

// REST 层第 4 翼：非 200 且响应体未超长 → 原文进错误信息（truncateForLog
// 的未截断翼）。
func TestGitHubProvider_Exchange_HTTPErrorShortBody(t *testing.T) {
	auth := githubTokenServer(t)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("token scope lacks read:user"))
	}))
	t.Cleanup(api.Close)

	p := githubProviderWith(t, auth, api.URL)
	id, err := p.Exchange(context.Background(), "code")
	require.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "github api /user: HTTP 403")
	assert.Contains(t, err.Error(), "token scope lacks read:user")
}

// REST 层第 5 翼：200 但响应体非 JSON → decode 错误；另锁 truncateForLog
// 的截断翼（超长响应体只保留前 200 字节，防日志注入与撑爆日志行）。
func TestGitHubProvider_Exchange_DecodeFailureAndBodyTruncation(t *testing.T) {
	auth := githubTokenServer(t)
	t.Run("decode", func(t *testing.T) {
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("<html>not json</html>"))
		}))
		defer api.Close()

		p := githubProviderWith(t, auth, api.URL)
		_, err := p.Exchange(context.Background(), "code")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "github api /user: decode")
	})

	t.Run("truncate", func(t *testing.T) {
		marker := strings.Repeat("x", 200) + "SECRET-TAIL"
		require.Greater(t, len(marker), 200, "响应体须超长才触发截断")
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(marker))
		}))
		defer api.Close()

		p := githubProviderWith(t, auth, api.URL)
		_, err := p.Exchange(context.Background(), "code")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "SECRET-TAIL", "超过 200 字节的响应体应被截断")
		assert.Contains(t, err.Error(), strings.Repeat("x", 200))
	})
}

// 邮箱兜底两翼：/user/emails 请求失败（静默回空，email 非登录必需要素）
// 与「返回列表但无 primary 标记」都不得阻断登录。
func TestGitHubProvider_Exchange_PrimaryEmailSilentFallbacks(t *testing.T) {
	profileNoEmail := `{"login":"alice","name":"Alice"}`

	t.Run("api failure", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"bearer"}`))
		})
		mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(profileNoEmail))
		})
		mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()

		p := githubProviderWith(t, srv.URL, srv.URL)
		id, err := p.Exchange(context.Background(), "code")
		require.NoError(t, err, "私有邮箱 API 被拒不应阻断登录")
		assert.Equal(t, "alice", id.Username)
		assert.Empty(t, id.Email, "拉取失败静默回空")
	})

	t.Run("no primary entry", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"bearer"}`))
		})
		mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"login":"alice"}`))
		})
		mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[{"email":"a@example.com","primary":false},{"email":"b@example.com","primary":false}]`))
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()

		p := githubProviderWith(t, srv.URL, srv.URL)
		id, err := p.Exchange(context.Background(), "code")
		require.NoError(t, err)
		assert.Equal(t, "alice", id.Username)
		assert.Empty(t, id.Email, "无 primary 标记时取不到邮箱")
		assert.Equal(t, "alice", id.Nickname, "昵称缺省回退 login")
	})
}
