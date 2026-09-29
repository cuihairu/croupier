package secguard

// 出站安全守卫测试（OPEN-ISSUES #56）：清单解析、CheckURL 矩阵（scheme/
// 端口/域名/SSRF 解析拦截）、HTTPClient 拨号 Control 级拦截（TOCTOU 消除）、
// 放行清单命中（单 IP + CIDR）。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseList(t *testing.T) {
	assert.Nil(t, parseList(""))
	assert.Nil(t, parseList("  , ,"))
	assert.Equal(t, []string{"example.com", "foo.io", "bar"}, parseList("example.com, foo.io\nbar ,"))
}

func TestParsePortList(t *testing.T) {
	assert.Nil(t, parsePortList(""))
	assert.Equal(t, []int{443, 8080}, parsePortList("443, 8080"))
	assert.Equal(t, []int{443}, parsePortList("443,bad,70000,0"))
}

func TestCheckURL_AllOffPassthrough(t *testing.T) {
	s := Settings{}
	assert.NoError(t, CheckURL(context.Background(), s, "ftp://anything"))
	assert.NoError(t, CheckURL(context.Background(), s, "not-a-url"))
}

func TestCheckURL_Matrix(t *testing.T) {
	schemeOnly := Settings{SSRFProtection: true}
	// 非 http(s) 拒绝
	require.Error(t, CheckURL(context.Background(), schemeOnly, "ftp://example.com"))
	require.Error(t, CheckURL(context.Background(), schemeOnly, "example.com"))

	// 端口白名单
	ports := Settings{AllowPorts: []int{443}}
	assert.NoError(t, CheckURL(context.Background(), ports, "https://example.com"))
	assert.ErrorContains(t, CheckURL(context.Background(), ports, "http://example.com:8080/x"), "端口")
	assert.NoError(t, CheckURL(context.Background(), Settings{}, "http://example.com:8080"))

	// 域名后缀白名单（子域匹配、后缀撞车拒绝）
	domains := Settings{AllowDomains: []string{"example.com"}}
	assert.NoError(t, CheckURL(context.Background(), domains, "https://api.example.com/v1"))
	assert.NoError(t, CheckURL(context.Background(), domains, "https://example.com"))
	assert.ErrorContains(t, CheckURL(context.Background(), domains, "https://evil-example.com"), "域名")
	assert.ErrorContains(t, CheckURL(context.Background(), domains, "https://github.com"), "域名")
}

func TestCheckURL_SSRFResolvesPrivate(t *testing.T) {
	s := Settings{SSRFProtection: true}
	// 公网域名（解析结果可能含 v6；只要有公网 IP 即可过——本用例域名稳定公网）
	assert.NoError(t, CheckURL(context.Background(), s, "https://example.com"))
	// localhost 静态解析即拒
	assert.ErrorContains(t, CheckURL(context.Background(), s, "http://localhost:8080"), "受限")
}

func TestCheckURL_SSRFAllowListCIDR(t *testing.T) {
	// 双栈 allow-list：runner 上 localhost 可能先解析到 ::1（IPv6 环回同样受限），
	// 只配 127.0.0.0/8 会随解析顺序偶发挂（CI 实证）。
	s := Settings{SSRFProtection: true, AllowIPs: []string{"127.0.0.0/8", "::1/128"}}
	assert.NoError(t, CheckURL(context.Background(), s, "http://localhost:8080"))
}

func TestHTTPClient_DialControlInterception(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	off := Settings{}
	resp, err := HTTPClient(off, &http.Client{}).Get(srv.URL)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	// 保护开：拨号 127.0.0.1 被 Control 钩子拦截
	on := Settings{SSRFProtection: true}
	client := HTTPClient(on, &http.Client{})
	_, err = client.Get(srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "受限")

	// 放行清单（单 IP）命中后放行
	allowed := Settings{SSRFProtection: true, AllowIPs: []string{"127.0.0.1"}}
	resp, err = HTTPClient(allowed, &http.Client{}).Get(srv.URL)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	// 保护关：HTTPClient 原样返回 base
	assert.Same(t, off2Base, HTTPClient(off, off2Base))
}

var off2Base = &http.Client{}
