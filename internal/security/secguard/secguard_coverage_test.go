package secguard

// 覆盖率巡检补测（secguard.go 74.3% / retry_probe.go 87.3% 残余翼）：
// #56/#57 落地时只测了 CheckURL 矩阵与拨号拦截，守卫的**策略侧**（七键解析、
// 端口缺省、超时/重试/退避钳制、派生客户端不污染共享实例）与**出站策略侧**
// （退避缺省、坏请求、重定向跳数、SMTP 探活三段）整块未覆盖。
//
// 注入手法：本地 httptest/net.Listener 假服务（HTTP 重定向链 + 脚本化 SMTP
// 会话），settings.InitLayered 铺 L3 真值走 Resolve 的非 nil 路径。

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// secL3Store 初始化 settings 单例并返回 L3 store 句柄（只迁 platform_settings）。
func secL3Store(t *testing.T) *model.PlatformSettingModel {
	t.Helper()
	settings.ResetForTest()
	t.Cleanup(settings.ResetForTest)

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/secguard.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PlatformSetting{}))

	store := model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	return store
}

func secSetL3(t *testing.T, store *model.PlatformSettingModel, key, raw string) {
	t.Helper()
	require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	settings.Current().Reload(context.Background(), store)
}

// Resolve(nil) 是 settings 未初始化的直通路径：必须全关零值而不是 panic。
func TestResolve_NilLayeredIsAllOff(t *testing.T) {
	assert.Equal(t, Settings{}, Resolve(nil))
}

// 七键解析：逗号/空白分隔、非法片段跳过、缺省值回落。
func TestResolve_ParsesSevenKeys(t *testing.T) {
	store := secL3Store(t)
	secSetL3(t, store, settings.KeySecAllowPorts, `" 443, 8443 ,notaport,0,70000 "`)
	secSetL3(t, store, settings.KeySecAllowIPs, `"10.0.0.0/8, 1.2.3.4"`)
	secSetL3(t, store, settings.KeySecDomainFilter, `"example.com, .Internal.Corp"`)
	secSetL3(t, store, settings.KeySecSSRFProtection, `true`)
	secSetL3(t, store, settings.KeyNetRequestTimeoutMs, `1500`)
	secSetL3(t, store, settings.KeyNetMaxRetries, `3`)
	secSetL3(t, store, settings.KeyNetRetryBackoffMs, `250`)

	got := Resolve(settings.Current())
	assert.Equal(t, []int{443, 8443}, got.AllowPorts, "非法/越界端口片段跳过")
	assert.Equal(t, []string{"10.0.0.0/8", "1.2.3.4"}, got.AllowIPs)
	assert.Equal(t, []string{"example.com", ".Internal.Corp"}, got.AllowDomains)
	assert.True(t, got.SSRFProtection)
	assert.Equal(t, 1500, got.RequestTimeoutMs)
	assert.Equal(t, 3, got.MaxRetries)
	assert.Equal(t, 250, got.RetryBackoffMs)

	// 同一实例再解一次（缓存路径）须稳定一致。
	assert.Equal(t, got, Resolve(settings.Current()))
}

// 未配置任何键 → 零值（零行为变更基线）。
func TestResolve_UnsetKeysAreZero(t *testing.T) {
	_ = secL3Store(t)
	got := Resolve(settings.Current())
	assert.Empty(t, got.AllowPorts)
	assert.Empty(t, got.AllowIPs)
	assert.Empty(t, got.AllowDomains)
	assert.False(t, got.SSRFProtection)
	assert.Zero(t, got.RequestTimeoutMs)
	assert.Zero(t, got.MaxRetries)
	assert.Zero(t, got.RetryBackoffMs)
}

// 端口缺省：显式端口 → scheme 缺省（https 443 / http 80）；显式端口非法回落缺省。
func TestPortOf_SchemeDefaults(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"https://h.example.com/x", 443},
		{"http://h.example.com/x", 80},
		{"https://h.example.com:8443/x", 8443},
		{"http://h.example.com:8080/x", 8080},
		// 端口数字溢出：url.Parse 放行（全数字），strconv.Atoi 失败 → 回落 scheme 缺省
		{"http://h.example.com:99999999999999999999/x", 80},
	}
	for _, tc := range cases {
		u, err := parseHTTPLike(tc.raw)
		require.NoError(t, err, tc.raw)
		assert.Equal(t, tc.want, portOf(u), tc.raw)
	}
}

// CheckURL：SSRF 开启时域名解析失败必须显式报错（区别于"解析到私有地址"）。
// 注：本机解析器会把任意短名劫持到 198.18.0.0/15（基准段），故用超长
// 主机名（>253 字符）触发本地解析器直接失败，不依赖外网 DNS 行为。
func TestCheckURL_DNSResolveFailure(t *testing.T) {
	s := Settings{SSRFProtection: true}
	long := strings.Repeat("a", 300) + ".invalid"
	err := CheckURL(context.Background(), s, "http://"+long+"/x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "域名解析失败")
}

// HTTPClient：仅配超时时派生新实例，不得改写包级共享 base（http.DefaultClient）。
func TestHTTPClient_TimeoutDerivesCopyWithoutMutatingBase(t *testing.T) {
	base := http.DefaultClient
	derived := HTTPClient(Settings{RequestTimeoutMs: 2500}, base)
	require.NotSame(t, base, derived, "共享实例必须派生副本")
	assert.Equal(t, 2500*time.Millisecond, derived.Timeout)
	assert.Equal(t, time.Duration(0), base.Timeout, "base 超时不得被原地改写")

	// SSRF 开启 + 自定义 base：Timeout/CheckRedirect 需从 base 继承。
	cb := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	custom := &http.Client{Timeout: 7 * time.Second, CheckRedirect: cb}
	withSSRF := HTTPClient(Settings{SSRFProtection: true}, custom)
	assert.Equal(t, 7*time.Second, withSSRF.Timeout)
	assert.NotNil(t, withSSRF.CheckRedirect)
	assert.Equal(t, 7*time.Second, custom.Timeout, "base 超时不得被改写")

	// 全关 → 原样返回 base。
	assert.Same(t, base, HTTPClient(Settings{}, base))
	assert.Same(t, base, HTTPClient(Settings{RequestTimeoutMs: 0}, base))
}

// 超时/重试/退避三件套的取值与钳制。
func TestSettings_TimeoutRetriesBackoff(t *testing.T) {
	assert.Equal(t, 5*time.Second, Settings{}.TimeoutOrDefault(5*time.Second), "未配置用调用方缺省")
	assert.Equal(t, 1200*time.Millisecond,
		Settings{RequestTimeoutMs: 1200}.TimeoutOrDefault(5*time.Second))

	assert.Equal(t, 0, Settings{MaxRetries: -3}.Retries(), "负值收敛为不重试")
	assert.Equal(t, 0, Settings{}.Retries())
	assert.Equal(t, 4, Settings{MaxRetries: 4}.Retries())
	assert.Equal(t, maxRetries, Settings{MaxRetries: 999}.Retries(), "超上限钳到 10")

	assert.Equal(t, DefaultRetryBackoff, Settings{}.Backoff())
	assert.Equal(t, 300*time.Millisecond, Settings{RetryBackoffMs: 300}.Backoff())
}

// 拨号钩子：地址不可拆端口 / 主机非 IP 两类拒绝。
func TestDialControl_RejectsMalformedAddress(t *testing.T) {
	hook := dialControl(Settings{})
	assert.ErrorContains(t, hook("tcp", "no-port-here", nil), "非法地址")
	assert.ErrorContains(t, hook("tcp", "not-an-ip:80", nil), "非法 IP")
	assert.NoError(t, hook("tcp", "1.2.3.4:80", nil), "公网地址放行")
}

// ipAllowed：非法 CIDR 片段跳过且不中断后续条目。
func TestIPAllowed_SkipsInvalidCIDR(t *testing.T) {
	loop := net.ParseIP("127.0.0.1")
	s := Settings{AllowIPs: []string{"not-a-cidr/xx", "10.1.2.3"}}
	assert.False(t, ipAllowed(loop, s), "非法 CIDR 不得放行")
	assert.True(t, ipAllowed(net.ParseIP("10.1.2.3"), s), "非法片段不应中断后续条目")
	assert.True(t, ipAllowed(net.ParseIP("10.9.9.9"), Settings{AllowIPs: []string{"10.0.0.0/8"}}))
}

// DoWithRetry：退避缺省（backoff<=0）与请求构造失败两翼。
func TestDoWithRetry_BackoffDefaultAndBadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// backoff<=0 → 回落 DefaultRetryBackoff（本例不重试，不实际等待）。
	resp, err := DoWithRetry(context.Background(), srv.Client(), http.MethodGet, srv.URL, nil, nil, 0, 0)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 方法名含空格 → NewRequestWithContext 直接失败。
	resp, err = DoWithRetry(context.Background(), srv.Client(), "BAD METHOD", srv.URL, nil, nil, 2, time.Millisecond)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

// DoWithRetry：5xx 重试到成功；body/headers 逐轮重建。
func TestDoWithRetry_RetriesOn5xxThenSucceeds(t *testing.T) {
	var hits int
	var gotBody string
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		gotHeader = r.Header.Get("X-Token")
		if hits == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := DoWithRetry(context.Background(), srv.Client(), http.MethodPost, srv.URL,
		[]byte(`{"a":1}`), map[string]string{"X-Token": "t"}, 2, time.Millisecond)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 2, hits, "首轮 5xx 应触发一次重试")
	assert.Equal(t, `{"a":1}`, gotBody, "重试轮次须重建请求体")
	assert.Equal(t, "t", gotHeader)
	resp.Body.Close()

	// 4xx 不重试：命中即返回。
	hits = 0
	resp, err = DoWithRetry(context.Background(), srv.Client(), http.MethodPost, srv.URL,
		[]byte(`{}`), nil, 3, time.Millisecond)
	require.NoError(t, err)
	resp.Body.Close()
}

// Probe：重定向跳数——3 跳内放行（OK），超 3 跳报错。
func TestProbe_RedirectHopLimit(t *testing.T) {
	// 每级都跳到下一级；hops=2 → via 长度最大 2，放行到终点 200。
	run := func(t *testing.T, hops int) ProbeResult {
		t.Helper()
		mux := http.NewServeMux()
		for i := 0; i < hops; i++ {
			next := fmt.Sprintf("/r%d", i+1)
			mux.HandleFunc(fmt.Sprintf("/r%d", i), func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", next)
				w.WriteHeader(http.StatusFound)
			})
		}
		mux.HandleFunc("/r"+fmt.Sprint(hops), func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()
		return Probe(context.Background(), Settings{}, srv.URL+"/r0")
	}

	ok := run(t, 2)
	assert.True(t, ok.OK, "3 跳以内应放行: %+v", ok)
	assert.Equal(t, http.StatusOK, ok.Status)

	blocked := run(t, 5)
	assert.False(t, blocked.OK)
	assert.Contains(t, blocked.Error, "重定向超过 3 跳")
	assert.Equal(t, 0, blocked.Status)
}

// ProbeSMTP 假服务：脚本化应答，覆盖 greeting/EHLO/NOOP 三段失败与成功。
func TestProbeSMTP_FakeServerMatrix(t *testing.T) {
	// startFakeSMTP 起一个按脚本应答的 SMTP 会话服务，返回 host/port。
	startFakeSMTP := func(t *testing.T, greeting string, ehloReply, noopReply string) (string, int) {
		t.Helper()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { _ = ln.Close() })

		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
			_, _ = conn.Write([]byte(greeting))
			r := bufio.NewReader(conn)
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					return
				}
				cmd := line[:min(4, len(line))]
				switch cmd {
				case "EHLO", "HELO":
					if ehloReply != "" {
						_, _ = conn.Write([]byte(ehloReply))
					}
				case "NOOP":
					if noopReply != "" {
						_, _ = conn.Write([]byte(noopReply))
					}
				case "QUIT":
					_, _ = conn.Write([]byte("221 bye\r\n"))
					return
				default:
					return
				}
			}
		}()

		addr := ln.Addr().(*net.TCPAddr)
		return "127.0.0.1", addr.Port
	}

	t.Run("成功", func(t *testing.T) {
		host, port := startFakeSMTP(t, "220 ready\r\n", "250 hello\r\n", "250 pong\r\n")
		res := ProbeSMTP(context.Background(), Settings{}, host, port)
		assert.True(t, res.OK, "完整 EHLO+NOOP 应探活成功: %+v", res)
		assert.Zero(t, res.Status)
	})

	t.Run("SSRF 保护下拨号钩子生效并探活成功", func(t *testing.T) {
		host, port := startFakeSMTP(t, "220 ready\r\n", "250 hello\r\n", "250 pong\r\n")
		s := Settings{SSRFProtection: true, AllowIPs: []string{"127.0.0.1"}}
		res := ProbeSMTP(context.Background(), s, host, port)
		assert.True(t, res.OK, "回环在 allowIPs 内应放行: %+v", res)
	})

	t.Run("SSRF 保护下未放行回环被拨号钩子拦截", func(t *testing.T) {
		host, port := startFakeSMTP(t, "220 ready\r\n", "250 hello\r\n", "250 pong\r\n")
		res := ProbeSMTP(context.Background(), Settings{SSRFProtection: true}, host, port)
		assert.False(t, res.OK)
		assert.Contains(t, res.Error, "拒绝连接受限地址")
	})

	t.Run("问候语缺失", func(t *testing.T) {
		host, port := startFakeSMTP(t, "", "", "") // 建连即关闭 → NewClient 读不到 220
		res := ProbeSMTP(context.Background(), Settings{}, host, port)
		assert.False(t, res.OK)
		assert.NotEmpty(t, res.Error)
	})

	t.Run("EHLO 被拒", func(t *testing.T) {
		host, port := startFakeSMTP(t, "220 ready\r\n", "500 no ehlo\r\n", "250 pong\r\n")
		res := ProbeSMTP(context.Background(), Settings{}, host, port)
		assert.False(t, res.OK)
		assert.NotEmpty(t, res.Error)
	})

	t.Run("NOOP 被拒", func(t *testing.T) {
		host, port := startFakeSMTP(t, "220 ready\r\n", "250 hello\r\n", "500 no noop\r\n")
		res := ProbeSMTP(context.Background(), Settings{}, host, port)
		assert.False(t, res.OK)
		assert.NotEmpty(t, res.Error)
	})

	t.Run("拨号失败", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := ln.Addr().(*net.TCPAddr).Port
		require.NoError(t, ln.Close())
		res := ProbeSMTP(context.Background(), Settings{}, "127.0.0.1", port)
		assert.False(t, res.OK)
		assert.NotEmpty(t, res.Error)
	})
}
