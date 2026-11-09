package serverstatus

// 状态源补测：Atlas Name、注册表 atlas 闭包、NewGate 缺省缓存、CacheKey 的
// ip 分支、listServers 的未配置/传输失败臂、fetch 的请求构造失败臂与响应体
// 读失败臂、matchValue 的 ip 分支、数组/包裹解码失败臂、inMaintenance 的
// 字段缺失与非常量类型臂，以及 parseTimeOrZero 的不可解析回落。
//
// 诚实跳过：
//   - fetch 的 http 非 200 臂（92-94）与超时臂（87-89）已由既有
//     TestHTTPProvider_FailsOpen 覆盖（500/10ms timeout 用例）。
//   - mapResponse/mapList 的其余解码失败形态在既有测试已覆盖。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Atlas Name 上报注册表键（atlas.go 73）。
func TestAtlasProvider_Name(t *testing.T) {
	p := NewAtlasProvider(Options{BaseURL: "http://x"})
	assert.Equal(t, ProviderAtlas, p.Name())
}

// 注册表：atlas 闭包实际构造（registry.go 42）。
func TestRegistry_AtlasClosure(t *testing.T) {
	factory := Lookup(ProviderAtlas)
	require.NotNil(t, factory, "atlas factory must exist")
	assert.Equal(t, ProviderAtlas, factory(Options{BaseURL: "http://x"}).Name())
}

// NewGate：cache 为 nil 时新建默认 TTL 缓存（gate.go 38-39）。
func TestNewGate_DefaultCache(t *testing.T) {
	g := NewGate(NewNoopProvider(), nil, nil)
	require.NotNil(t, g.cache, "nil cache 应回退默认")
}

// CacheKey：仅 IP 可用时走 ip 分支（provider.go 53）。
func TestCacheKey_IPBranch(t *testing.T) {
	assert.Equal(t, "ip:1.2.3.4", (ServerRef{IP: "1.2.3.4"}).CacheKey())
}

// listServers：baseURL 未配置（68-69）与传输失败（72-73）。
func TestListServers_UnconfiguredAndTransportFail(t *testing.T) {
	p := NewHTTPProvider(Options{})
	_, err := p.ListServers(context.Background())
	require.ErrorIs(t, err, ErrProviderUnavailable)

	boom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer boom.Close()
	_, err = NewHTTPProvider(Options{BaseURL: boom.URL}).ListServers(context.Background())
	require.ErrorIs(t, err, ErrProviderUnavailable)
}

// fetch：请求构造失败（非法 URL 转义，rest.go 82-83）。
func TestFetch_NewRequestError(t *testing.T) {
	p := NewHTTPProvider(Options{BaseURL: "http://host/%zz"})
	_, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "a"})
	require.ErrorIs(t, err, ErrProviderUnavailable)
}

// fetch：响应体读失败（Content-Length 超出实际字节，rest.go 97-98）。
func TestFetch_BodyReadError(t *testing.T) {
	truncated := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("short"))
	}))
	defer truncated.Close()
	p := NewHTTPProvider(Options{BaseURL: truncated.URL})
	_, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "a"})
	require.ErrorIs(t, err, ErrProviderUnavailable)
}

// matchValue：MatchKey=ip 分支（rest.go 136）。
func TestMatchValue_IP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ip") != "1.2.3.4" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"inMaintenance": false}`))
	}))
	defer srv.Close()
	p := NewHTTPProvider(Options{BaseURL: srv.URL, Atlas: AtlasOptions{MatchKey: "ip"}})
	st, err := p.GetServerStatus(context.Background(), ServerRef{IP: "1.2.3.4"})
	require.NoError(t, err)
	assert.False(t, st.InMaintenance)
}

// mapResponse：数组解码失败（rest.go 152-153）。
func TestMapResponse_ArrayDecodeError(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[not-json`))
	}))
	defer bad.Close()
	_, err := NewHTTPProvider(Options{BaseURL: bad.URL}).GetServerStatus(context.Background(), ServerRef{AgentID: "a"})
	require.ErrorIs(t, err, ErrProviderUnavailable)
}

// mapList：数组解码失败（183-184）与包裹解码失败（191-192）。
func TestMapList_DecodeErrors(t *testing.T) {
	arrayBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[not-json`))
	}))
	defer arrayBad.Close()
	_, err := NewHTTPProvider(Options{BaseURL: arrayBad.URL}).ListServers(context.Background())
	require.ErrorIs(t, err, ErrProviderUnavailable)

	wrappedBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"servers": bad}`))
	}))
	defer wrappedBad.Close()
	_, err = NewHTTPProvider(Options{BaseURL: wrappedBad.URL}).ListServers(context.Background())
	require.ErrorIs(t, err, ErrProviderUnavailable)
}

// inMaintenance：字段缺失（242-243）与非常量类型（250）。
func TestInMaintenance_MissingAndOtherType(t *testing.T) {
	missing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"agentId":"a1"}]`))
	}))
	defer missing.Close()
	rows, err := NewHTTPProvider(Options{BaseURL: missing.URL}).ListServers(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.False(t, rows[0].InMaintenance, "字段缺失视为不在维护")

	num := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"inMaintenance": 5}`))
	}))
	defer num.Close()
	st, err := NewHTTPProvider(Options{BaseURL: num.URL}).GetServerStatus(context.Background(), ServerRef{AgentID: "a"})
	require.NoError(t, err)
	assert.False(t, st.InMaintenance, "数字形态按零值处理")
}

// parseTimeOrZero：三布局均不匹配回落零值（rest.go 285）。
func TestParseTimeOrZero_Fallback(t *testing.T) {
	assert.True(t, parseTimeOrZero("not-a-time").IsZero())
	assert.False(t, parseTimeOrZero("2026-10-12T00:00:00Z").IsZero())
}

// toStatus：window 部分字段缺失与不可解析时间（224-233 分支细化）。
func TestToStatus_WindowPartial(t *testing.T) {
	s := &restSource{opts: AtlasOptions{}}
	st := s.toStatus(map[string]interface{}{
		"windowStart": "garbage",
		"windowNote":  "planned",
	})
	require.NotNil(t, st.Window)
	assert.True(t, st.Window.Start.IsZero(), "不可解析时间回零")
	assert.Equal(t, "planned", st.Window.Note)

	onlyEnd := s.toStatus(map[string]interface{}{"windowEnd": "2026-10-12T00:00:00Z"})
	require.NotNil(t, onlyEnd.Window)
	assert.True(t, onlyEnd.Window.Start.IsZero())
}
