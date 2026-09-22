package cmd

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	mq "github.com/cuihairu/croupier/internal/analytics/mq"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// nowUnixStr 当前 Unix 秒（签名契约的时间戳格式）。
func nowUnixStr() string { return strconv.FormatInt(time.Now().Unix(), 10) }

// newTestRedisClient 用 miniredis 地址构造已连接的 redis 客户端。
func newTestRedisClient(t *testing.T, addr string) *redis.Client {
	t.Helper()
	opt, err := redis.ParseURL("redis://" + addr)
	require.NoError(t, err)
	return redis.NewClient(opt)
}

// waitTimeout 等待 WaitGroup 完成，超时判失败（避免 goroutine 泄漏静默通过）。
func waitTimeout(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("goroutine did not stop after cancel")
	}
}

// signPayload 按认证契约生成签名：base64(HMAC_SHA256(secret, ts + "\n" + nonce + "\n" + sha256hex(body)))。
func signPayload(secret string, ts int64, nonce string, body []byte) string {
	sum := sha256.Sum256(body)
	msg := strconv.FormatInt(ts, 10) + "\n" + nonce + "\n" + hex.EncodeToString(sum[:])
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(msg))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// fakeQueue 可注入失败的 mq.Queue 替身，并支持积压查询接口。
type fakeQueue struct {
	pubEventErr error
	pubPayErr   error
	events      []map[string]any
	payments    []map[string]any

	pendingEvents   int64
	pendingPayments int64
	pendingErr      error
	pendingCalls    int64
}

func (f *fakeQueue) PublishEvent(evt map[string]any) error {
	if f.pubEventErr != nil {
		return f.pubEventErr
	}
	f.events = append(f.events, evt)
	return nil
}

func (f *fakeQueue) PublishPayment(pay map[string]any) error {
	if f.pubPayErr != nil {
		return f.pubPayErr
	}
	f.payments = append(f.payments, pay)
	return nil
}

func (f *fakeQueue) Close() error { return nil }

func (f *fakeQueue) PendingEvents() (int64, error) {
	atomic.AddInt64(&f.pendingCalls, 1)
	if f.pendingErr != nil {
		return 0, f.pendingErr
	}
	return f.pendingEvents, nil
}

func (f *fakeQueue) PendingPayments() (int64, error) {
	if f.pendingErr != nil {
		return 0, f.pendingErr
	}
	return f.pendingPayments, nil
}

func TestParseListenAddr(t *testing.T) {
	cases := []struct {
		in         string
		wantHost   string
		wantPort   int
		wantErr    bool
		defaultPrt int
	}{
		{in: "", wantHost: "0.0.0.0", wantPort: 8088, defaultPrt: 8088},
		{in: "  :9000  ", wantHost: "0.0.0.0", wantPort: 9000},
		{in: "8088", wantHost: "0.0.0.0", wantPort: 8088},
		{in: "127.0.0.1:9001", wantHost: "127.0.0.1", wantPort: 9001},
		{in: "[::1]:9002", wantHost: "::1", wantPort: 9002},
		{in: "host.example:0", wantErr: true},
		{in: "host.example:abc", wantErr: true},
		{in: "no-port-here", wantErr: true},
		{in: "host:-5", wantErr: true},
	}
	for _, tc := range cases {
		host, port, err := parseListenAddr(tc.in, tc.defaultPrt)
		if tc.wantErr {
			assert.Error(t, err, "addr %q", tc.in)
			continue
		}
		require.NoError(t, err, "addr %q", tc.in)
		assert.Equal(t, tc.wantHost, host, "addr %q", tc.in)
		assert.Equal(t, tc.wantPort, port, "addr %q", tc.in)
	}
}

func TestAbs64(t *testing.T) {
	assert.Equal(t, int64(7), abs64(7))
	assert.Equal(t, int64(7), abs64(-7))
	assert.Equal(t, int64(0), abs64(0))
}

func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()
	respondJSON(w, http.StatusTeapot, map[string]string{"err": "x"})
	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `"err":"x"`)
}

func TestSecretForGame(t *testing.T) {
	s := &server{secret: "global", perSecret: map[string]string{"game-a": "secret-a"}}
	assert.Equal(t, "secret-a", s.secretForGame("game-a"))
	// perSecret 未命中时回落全局密钥
	assert.Equal(t, "global", s.secretForGame("game-b"))
	// 空 gameID 直接走全局
	assert.Equal(t, "global", s.secretForGame(""))
}

func TestNonceKey(t *testing.T) {
	s := &server{}
	assert.Equal(t, "ingest:nonce:default:n1", s.nonceKey("", "n1"))
	assert.Equal(t, "ingest:nonce:game-a:n1", s.nonceKey("game-a", "n1"))
}

func TestValidateEventPayload(t *testing.T) {
	valid := map[string]any{"game_id": "g", "env": "prod", "ts": "1", "event": "login"}
	require.NoError(t, validateEventPayload(valid, "g"))
	require.NoError(t, validateEventPayload(valid, ""))

	// 缺每个必填字段都要报对应错误
	for _, key := range []string{"game_id", "env", "ts", "event"} {
		broken := map[string]any{"game_id": "g", "env": "prod", "ts": "1", "event": "login"}
		delete(broken, key)
		err := validateEventPayload(broken, "")
		require.Error(t, err, "key %s", key)
		assert.Contains(t, err.Error(), "missing "+key)
	}

	// header game 与 payload game 不一致
	err := validateEventPayload(valid, "other")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

func TestValidatePaymentPayload(t *testing.T) {
	valid := map[string]any{"game_id": "g", "env": "prod", "ts": "1", "order_id": "o1", "status": "paid", "amount_cents": 100}
	require.NoError(t, validatePaymentPayload(valid, "g"))

	for _, key := range []string{"game_id", "env", "ts", "order_id", "status"} {
		broken := map[string]any{"game_id": "g", "env": "prod", "ts": "1", "order_id": "o1", "status": "paid", "amount_cents": 100}
		delete(broken, key)
		err := validatePaymentPayload(broken, "")
		require.Error(t, err, "key %s", key)
		assert.Contains(t, err.Error(), "missing "+key)
	}

	require.ErrorContains(t, validatePaymentPayload(valid, "other"), "mismatch")

	// amount_cents 缺失 / 非正数 / 非数值类型
	noAmount := map[string]any{"game_id": "g", "env": "prod", "ts": "1", "order_id": "o1", "status": "paid"}
	require.ErrorContains(t, validatePaymentPayload(noAmount, ""), "amount_cents")
	zero := map[string]any{"game_id": "g", "env": "prod", "ts": "1", "order_id": "o1", "status": "paid", "amount_cents": 0}
	require.ErrorContains(t, validatePaymentPayload(zero, ""), "amount_cents")
	negative := map[string]any{"game_id": "g", "env": "prod", "ts": "1", "order_id": "o1", "status": "paid", "amount_cents": -1}
	require.ErrorContains(t, validatePaymentPayload(negative, ""), "amount_cents")
	// mapFloat 支持 int / int64 / float64
	for _, v := range []any{1, int64(1), 1.5} {
		ok := map[string]any{"game_id": "g", "env": "prod", "ts": "1", "order_id": "o1", "status": "paid", "amount_cents": v}
		require.NoError(t, validatePaymentPayload(ok, ""))
	}
	// 字符串金额被 mapFloat 视为 0
	strAmount := map[string]any{"game_id": "g", "env": "prod", "ts": "1", "order_id": "o1", "status": "paid", "amount_cents": "1"}
	require.ErrorContains(t, validatePaymentPayload(strAmount, ""), "amount_cents")
}

func TestMapStringAndFloat(t *testing.T) {
	assert.Equal(t, "s", mapString(map[string]any{"k": "s"}, "k"))
	assert.Equal(t, "b", mapString(map[string]any{"k": []byte("b")}, "k"))
	assert.Equal(t, "", mapString(map[string]any{"k": 1}, "k"))
	assert.Equal(t, "", mapString(map[string]any{}, "k"))

	assert.Equal(t, 1.5, mapFloat(map[string]any{"k": 1.5}, "k"))
	assert.Equal(t, float64(2), mapFloat(map[string]any{"k": 2}, "k"))
	assert.Equal(t, float64(3), mapFloat(map[string]any{"k": int64(3)}, "k"))
	assert.Equal(t, float64(0), mapFloat(map[string]any{"k": "x"}, "k"))
	assert.Equal(t, float64(0), mapFloat(map[string]any{}, "k"))
}

func TestLoadPerGameSecrets(t *testing.T) {
	t.Setenv("ANALYTICS_INGEST_SECRETS", "")
	assert.Empty(t, loadPerGameSecrets())

	t.Setenv("ANALYTICS_INGEST_SECRETS", `{"game-a":"s1"}`)
	assert.Equal(t, map[string]string{"game-a": "s1"}, loadPerGameSecrets())

	t.Setenv("ANALYTICS_INGEST_SECRETS", `{not-json`)
	assert.Empty(t, loadPerGameSecrets())
}

func TestNewDedupeClient(t *testing.T) {
	t.Setenv("INGEST_REDIS_URL", "")
	t.Setenv("REDIS_URL", "")
	assert.Nil(t, newDedupeClient())

	t.Setenv("INGEST_REDIS_URL", "://bad url")
	assert.Nil(t, newDedupeClient())

	mr := miniredis.RunT(t)
	t.Setenv("INGEST_REDIS_URL", "redis://"+mr.Addr())
	client, ok := newDedupeClient().(*redis.Client)
	require.True(t, ok)
	require.NotNil(t, client)
	require.NoError(t, client.Close())

	// INGEST_REDIS_URL 未设置时回落 REDIS_URL
	t.Setenv("INGEST_REDIS_URL", "")
	t.Setenv("REDIS_URL", "redis://"+mr.Addr())
	client2, ok2 := newDedupeClient().(*redis.Client)
	require.True(t, ok2)
	require.NotNil(t, client2)
	require.NoError(t, client2.Close())
}

func TestEnvDuration(t *testing.T) {
	t.Setenv("TEST_INGEST_TTL", "")
	assert.Equal(t, 15*time.Minute, envDuration("TEST_INGEST_TTL", 15*time.Minute))

	t.Setenv("TEST_INGEST_TTL", "1m30s")
	assert.Equal(t, 90*time.Second, envDuration("TEST_INGEST_TTL", time.Minute))

	t.Setenv("TEST_INGEST_TTL", "not-a-duration")
	assert.Equal(t, time.Minute, envDuration("TEST_INGEST_TTL", time.Minute))
}

func TestEnvOrDefaultHelpers(t *testing.T) {
	t.Setenv("TEST_INGEST_STR", "  val  ")
	assert.Equal(t, "val", envOrDefault("TEST_INGEST_STR", "def"))
	t.Setenv("TEST_INGEST_STR", "   ")
	assert.Equal(t, "def", envOrDefault("TEST_INGEST_STR", "def"))

	t.Setenv("TEST_INGEST_INT", " 7 ")
	assert.Equal(t, 7, envOrDefaultInt("TEST_INGEST_INT", 1))
	t.Setenv("TEST_INGEST_INT", "abc")
	assert.Equal(t, 1, envOrDefaultInt("TEST_INGEST_INT", 1))
	t.Setenv("TEST_INGEST_INT", "-3")
	assert.Equal(t, 1, envOrDefaultInt("TEST_INGEST_INT", 1))
	t.Setenv("TEST_INGEST_INT", "")
	assert.Equal(t, 1, envOrDefaultInt("TEST_INGEST_INT", 1))
}

func TestShortVersion(t *testing.T) {
	origVer, origCommit, origBuild := Version, GitCommit, BuildTime
	t.Cleanup(func() { Version, GitCommit, BuildTime = origVer, origCommit, origBuild })

	Version, GitCommit, BuildTime = "v1", "unknown", ""
	assert.Equal(t, "v1", shortVersion())

	Version, GitCommit, BuildTime = "v1", "abc123", ""
	assert.Equal(t, "v1 (abc123)", shortVersion())

	Version, GitCommit, BuildTime = "v1", "abc123", "2026-01-01"
	assert.Equal(t, "v1 (abc123, built 2026-01-01)", shortVersion())
}

func TestPrintVersionInfo(t *testing.T) {
	origVer, origCommit, origBuild := Version, GitCommit, BuildTime
	t.Cleanup(func() { Version, GitCommit, BuildTime = origVer, origCommit, origBuild })

	out := captureStdout(t, func() { PrintVersionInfo() })
	assert.Contains(t, out, "Croupier Ingest")
	// dev 默认值下不输出 commit / build time
	assert.NotContains(t, out, "Git commit")

	Version, GitCommit, BuildTime = "v9", "abc", "built-at"
	out = captureStdout(t, func() { versionCmd.Run(versionCmd, nil) })
	assert.Contains(t, out, "Croupier Ingest v9")
	assert.Contains(t, out, "Git commit: abc")
	assert.Contains(t, out, "Build time: built-at")
}

// captureStdout 捕获函数期间的 stdout 输出。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	require.NoError(t, w.Close())
	return <-done
}

func newTestServer(q mq.Queue) *server {
	return &server{
		q:           q,
		secret:      "top-secret",
		allowSkew:   time.Minute,
		maxBodySize: 1 << 20,
		limiter:     rate.NewLimiter(rate.Inf, 1),
	}
}

func TestHealthzHandler(t *testing.T) {
	s := newTestServer(&fakeQueue{})
	s.latencySum = 3000 // 3ms 总量
	s.latencyCount = 2
	s.eventsProcessed = 5

	w := httptest.NewRecorder()
	s.healthzHandler(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
	metrics := body["metrics"].(map[string]any)
	assert.Equal(t, float64(5), metrics["events_processed"])
	// 平均延迟 = 3000µs / 2 / 1000 = 1.5ms
	assert.InDelta(t, 1.5, metrics["avg_latency_ms"], 1e-9)
	assert.Contains(t, body, "system")
	assert.Contains(t, body, "config")

	// latencyCount=0 的零除保护路径
	w2 := httptest.NewRecorder()
	s.latencyCount = 0
	s.healthzHandler(w2, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, w2.Code)
	var body2 map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &body2))
	assert.Equal(t, float64(0), body2["metrics"].(map[string]any)["avg_latency_ms"])
}

func TestAuthMiddlewareDisabled(t *testing.T) {
	s := &server{} // 无全局密钥也无 per-game 密钥
	w := httptest.NewRecorder()
	s.authMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(
		w, httptest.NewRequest(http.MethodPost, "/api/ingest/events", strings.NewReader("{}")))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ingest_disabled")
}

func TestAuthMiddleware(t *testing.T) {
	nextCalled := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled++
		w.WriteHeader(http.StatusAccepted)
	})

	newSrv := func() *server {
		s := &server{secret: "top-secret", allowSkew: time.Minute}
		return s
	}

	t.Run("unknown_game", func(t *testing.T) {
		s := &server{perSecret: map[string]string{"known": "k-secret"}}
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("{}"))
		req.Header.Set("X-Game-Id", "unknown-game")
		req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
		req.Header.Set("X-Nonce", "n")
		req.Header.Set("X-Signature", "sig")
		w := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "unknown_game")
	})

	t.Run("missing_auth_headers", func(t *testing.T) {
		s := newSrv()
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("{}"))
		w := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "missing_auth_headers")
	})

	t.Run("bad_timestamp", func(t *testing.T) {
		s := newSrv()
		req := signedRequest(t, s.secret, "not-a-number", "n1", "sig", "{}")
		w := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "bad_timestamp")
	})

	t.Run("timestamp_skew", func(t *testing.T) {
		s := newSrv()
		old := strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)
		req := signedRequest(t, s.secret, old, "n1", "sig", "{}")
		w := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "timestamp_skew")
	})

	t.Run("read_body_failed", func(t *testing.T) {
		s := newSrv()
		req := httptest.NewRequest(http.MethodPost, "/x", errReader{})
		req.Header.Set("X-Timestamp", nowUnixStr())
		req.Header.Set("X-Nonce", "n1")
		req.Header.Set("X-Signature", "sig")
		w := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "read_body_failed")
	})

	t.Run("bad_signature", func(t *testing.T) {
		s := newSrv()
		req := signedRequest(t, s.secret, nowUnixStr(), "n1", "bogus-signature", "{}")
		w := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "bad_signature")
	})

	t.Run("valid_no_dedupe", func(t *testing.T) {
		s := newSrv()
		before := nextCalled
		body := `{"a":1}`
		req := signedRequest(t, s.secret, nowUnixStr(), "n-valid", "", body)
		w := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Equal(t, before+1, nextCalled)
		// gameID 透传给下游 header
		assert.Equal(t, "", req.Header.Get("X-Game-Id"))
	})

	t.Run("valid_with_per_game_secret", func(t *testing.T) {
		s := &server{
			perSecret: map[string]string{"game-a": "a-secret"},
			allowSkew: time.Minute,
		}
		body := `{"a":2}`
		req := signedRequest(t, "a-secret", nowUnixStr(), "n-a", "", body)
		req.Header.Set("X-Game-Id", "game-a")
		w := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Equal(t, "game-a", req.Header.Get("X-Game-Id"))
	})

	t.Run("dedupe_replay_rejected", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := newTestRedisClient(t, mr.Addr())
		s := newSrv()
		s.dedupe = client
		s.dedupTTL = time.Minute

		body := `{"a":3}`
		ts := nowUnixStr()
		req := signedRequest(t, s.secret, ts, "nonce-replay", "", body)
		w := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)

		// 同 nonce 重放 → 429 duplicate_nonce
		req2 := signedRequest(t, s.secret, ts, "nonce-replay", "", body)
		w2 := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		assert.Contains(t, w2.Body.String(), "duplicate_nonce")
	})

	t.Run("dedupe_errors_still_pass_through", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := newTestRedisClient(t, mr.Addr())
		require.NoError(t, client.Close()) // 关闭后 Exists/SetEx 均报错
		s := newSrv()
		s.dedupe = client

		before := nextCalled
		req := signedRequest(t, s.secret, nowUnixStr(), "n-err", "", "{}")
		w := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(w, req)
		// dedupe 故障不阻断业务（warn 后放行）
		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Equal(t, before+1, nextCalled)
	})
}

// signedRequest 构造带认证 header 的请求；sig 为空则填占位值。
func signedRequest(t *testing.T, secret, ts, nonce, sig, body string) *http.Request {
	t.Helper()
	if sig == "" {
		tsInt, err := strconv.ParseInt(ts, 10, 64)
		require.NoError(t, err)
		sig = signPayload(secret, tsInt, nonce, []byte(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", sig)
	return req
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestRateLimitMiddleware(t *testing.T) {
	called := 0
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called++ })

	s := &server{limiter: rate.NewLimiter(rate.Inf, 1), rateLimit: 100}
	s.rateLimitMiddleware(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/x", nil))
	assert.Equal(t, 1, called)
	assert.Equal(t, int64(0), atomic.LoadInt64(&s.requestsDropped))

	// burst=0 → 永不允许 → 429
	s2 := &server{limiter: rate.NewLimiter(rate.Limit(1), 0), rateLimit: 1}
	w := httptest.NewRecorder()
	s2.rateLimitMiddleware(next).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", nil))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "rate_limit_exceeded")
	assert.Equal(t, int64(1), atomic.LoadInt64(&s2.requestsDropped))
	assert.Equal(t, 1, called)
}

func TestMetricsMiddleware(t *testing.T) {
	t.Run("body_too_large", func(t *testing.T) {
		s := &server{maxBodySize: 10}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.ContentLength = 1024
		s.metricsMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(w, req)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.Contains(t, w.Body.String(), "body_too_large")
		assert.Equal(t, int64(1), atomic.LoadInt64(&s.requestsError))
	})

	t.Run("status_classification", func(t *testing.T) {
		for _, status := range []int{200, 302, 400, 401, 403, 500} {
			s := &server{maxBodySize: 1024}
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			})
			s.metricsMiddleware(handler).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/x", nil))
			assert.Equal(t, int64(1), atomic.LoadInt64(&s.requestsTotal))
			if status >= 200 && status < 400 {
				assert.Equal(t, int64(1), atomic.LoadInt64(&s.requestsSuccess), "status %d", status)
				assert.Equal(t, int64(0), atomic.LoadInt64(&s.requestsError), "status %d", status)
				continue
			}
			assert.Equal(t, int64(1), atomic.LoadInt64(&s.requestsError), "status %d", status)
			switch status {
			case 400:
				assert.Equal(t, int64(1), atomic.LoadInt64(&s.validationErrors))
			case 401, 403:
				assert.Equal(t, int64(1), atomic.LoadInt64(&s.authErrors))
			case 500:
				assert.Equal(t, int64(1), atomic.LoadInt64(&s.queueErrors))
			}
		}
	})

	t.Run("slow_request_logged", func(t *testing.T) {
		s := &server{maxBodySize: 1024}
		handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			time.Sleep(1100 * time.Millisecond)
		})
		s.metricsMiddleware(handler).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/x", nil))
		assert.Greater(t, atomic.LoadInt64(&s.latencyCount), int64(0))
		assert.Greater(t, atomic.LoadInt64(&s.latencySum), int64(0))
	})
}

func TestResponseWriterCapturesStatus(t *testing.T) {
	w := &responseWriter{ResponseWriter: httptest.NewRecorder()}
	w.WriteHeader(http.StatusAccepted)
	assert.Equal(t, http.StatusAccepted, w.status)
}

func TestIngestEvents(t *testing.T) {
	t.Run("method_not_allowed", func(t *testing.T) {
		s := newTestServer(&fakeQueue{})
		w := httptest.NewRecorder()
		s.ingestEvents(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("invalid_payload", func(t *testing.T) {
		s := newTestServer(&fakeQueue{})
		w := httptest.NewRecorder()
		s.ingestEvents(w, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{not-json`)))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid_payload")
	})

	t.Run("body_too_large", func(t *testing.T) {
		s := newTestServer(&fakeQueue{})
		s.maxBodySize = 8
		w := httptest.NewRecorder()
		s.ingestEvents(w, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`["aaaaaaaaaaaaaaaaaaaa"]`)))
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.Contains(t, w.Body.String(), "body_too_large")
	})

	t.Run("validation_error", func(t *testing.T) {
		s := newTestServer(&fakeQueue{})
		w := httptest.NewRecorder()
		s.ingestEvents(w, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`[{"game_id":"g"}]`)))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "missing env")
	})

	t.Run("game_mismatch", func(t *testing.T) {
		s := newTestServer(&fakeQueue{})
		req := httptest.NewRequest(http.MethodPost, "/x",
			strings.NewReader(`[{"game_id":"g","env":"p","ts":"1","event":"e"}]`))
		req.Header.Set("X-Game-Id", "other")
		w := httptest.NewRecorder()
		s.ingestEvents(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "mismatch")
	})

	t.Run("queue_write_failed", func(t *testing.T) {
		s := newTestServer(&fakeQueue{pubEventErr: errors.New("down")})
		w := httptest.NewRecorder()
		s.ingestEvents(w, httptest.NewRequest(http.MethodPost, "/x",
			strings.NewReader(`[{"game_id":"g","env":"p","ts":"1","event":"e"}]`)))
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "queue_write_failed")
	})

	t.Run("accepted", func(t *testing.T) {
		fq := &fakeQueue{}
		s := newTestServer(fq)
		w := httptest.NewRecorder()
		s.ingestEvents(w, httptest.NewRequest(http.MethodPost, "/x",
			strings.NewReader(`[{"game_id":"g","env":"p","ts":"1","event":"e"},{"game_id":"g","env":"p","ts":"2","event":"e"}]`)))
		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Len(t, fq.events, 2)
		assert.Equal(t, int64(2), atomic.LoadInt64(&s.eventsProcessed))
	})
}

func TestIngestPayments(t *testing.T) {
	t.Run("method_not_allowed", func(t *testing.T) {
		s := newTestServer(&fakeQueue{})
		w := httptest.NewRecorder()
		s.ingestPayments(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("invalid_payload", func(t *testing.T) {
		s := newTestServer(&fakeQueue{})
		w := httptest.NewRecorder()
		s.ingestPayments(w, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`[1,2`)))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid_payload")
	})

	t.Run("body_too_large", func(t *testing.T) {
		s := newTestServer(&fakeQueue{})
		s.maxBodySize = 4
		w := httptest.NewRecorder()
		s.ingestPayments(w, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`[{"aaaa":"aaaaaaaaaaaaaaaaaa"}]`)))
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.Contains(t, w.Body.String(), "body_too_large")
	})

	t.Run("validation_error", func(t *testing.T) {
		s := newTestServer(&fakeQueue{})
		w := httptest.NewRecorder()
		s.ingestPayments(w, httptest.NewRequest(http.MethodPost, "/x",
			strings.NewReader(`[{"game_id":"g","env":"p","ts":"1","order_id":"o"}]`)))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "missing status")
	})

	t.Run("amount_required", func(t *testing.T) {
		s := newTestServer(&fakeQueue{})
		w := httptest.NewRecorder()
		s.ingestPayments(w, httptest.NewRequest(http.MethodPost, "/x",
			strings.NewReader(`[{"game_id":"g","env":"p","ts":"1","order_id":"o","status":"paid"}]`)))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "amount_cents required")
	})

	t.Run("queue_write_failed", func(t *testing.T) {
		s := newTestServer(&fakeQueue{pubPayErr: errors.New("down")})
		w := httptest.NewRecorder()
		s.ingestPayments(w, httptest.NewRequest(http.MethodPost, "/x",
			strings.NewReader(`[{"game_id":"g","env":"p","ts":"1","order_id":"o","status":"paid","amount_cents":5}]`)))
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "queue_write_failed")
	})

	t.Run("accepted", func(t *testing.T) {
		fq := &fakeQueue{}
		s := newTestServer(fq)
		w := httptest.NewRecorder()
		s.ingestPayments(w, httptest.NewRequest(http.MethodPost, "/x",
			strings.NewReader(`[{"game_id":"g","env":"p","ts":"1","order_id":"o","status":"paid","amount_cents":5}]`)))
		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Len(t, fq.payments, 1)
		assert.Equal(t, int64(1), atomic.LoadInt64(&s.paymentsProcessed))
	})
}

func TestMonitorQueueBacklog(t *testing.T) {
	fq := &fakeQueue{pendingEvents: 12, pendingPayments: 34}
	s := &server{q: fq, queueCheckInterval: time.Millisecond}
	s.eventsPending, s.paymentsPending = -1, -1

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.monitorQueueBacklog(ctx)
	}()

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&s.eventsPending) == 12 && atomic.LoadInt64(&s.paymentsPending) == 34
	}, 2*time.Second, 5*time.Millisecond)
	assert.False(t, s.queueLastCheck.IsZero())

	// 查询报错时保留上次读数
	fq.pendingErr = errors.New("x")
	before := atomic.LoadInt64(&s.eventsPending)
	require.Eventually(t, func() bool { return atomic.LoadInt64(&fq.pendingCalls) >= 3 }, 2*time.Second, 5*time.Millisecond)
	assert.Equal(t, before, atomic.LoadInt64(&s.eventsPending))

	cancel()
	waitTimeout(t, &wg)
}

func TestReportMetrics(t *testing.T) {
	// 告警分支（错误率/积压/延迟）用单轮直测，确定性覆盖
	s := &server{}
	atomic.StoreInt64(&s.requestsTotal, 10)
	atomic.StoreInt64(&s.requestsError, 2)
	atomic.StoreInt64(&s.eventsPending, 20000)
	atomic.StoreInt64(&s.paymentsPending, 20000)
	atomic.StoreInt64(&s.latencySum, 3_000_000) // avg = 1500ms > 1000ms
	atomic.StoreInt64(&s.latencyCount, 2)
	s.reportMetricsOnce()

	// 循环体 + 复位分支：延迟计数超阈值时被周期复位
	s2 := &server{metricsInterval: time.Millisecond}
	atomic.StoreInt64(&s2.latencyCount, 1_000_001)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s2.reportMetrics(ctx)
	}()

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&s2.latencyCount) == 0
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
	waitTimeout(t, &wg)
}

func TestWaitForShutdown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	sig := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() { done <- waitForShutdown(srv.Config, sig) }()

	sig <- syscall.SIGTERM
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("waitForShutdown did not return")
	}
}

func TestNotifyShutdownSignals(t *testing.T) {
	ch := notifyShutdownSignals()
	require.NotNil(t, ch)
	assert.Equal(t, 0, len(ch))
}

func TestRunIngestMQFailure(t *testing.T) {
	t.Setenv("ANALYTICS_MQ_TYPE", "bogus-type")
	err := runIngest()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init analytics MQ")
}

// TestRunIngest_ConfigFallbacks 覆盖 runIngest 的配置边界分支：
// 空密钥告警 + 非法监听地址报错。
func TestRunIngest_ConfigFallbacks(t *testing.T) {
	t.Setenv("ANALYTICS_MQ_TYPE", "noop")
	t.Setenv("ANALYTICS_INGEST_SECRETS", "")

	orig := waitForShutdownFn
	t.Cleanup(func() { waitForShutdownFn = orig })
	waitForShutdownFn = func(*http.Server, <-chan os.Signal) error { return nil }

	origListen, origSecret := listenAddr, sharedSecret
	t.Cleanup(func() { listenAddr, sharedSecret = origListen, origSecret })

	// 空密钥 → 启动时告警（仍能起服务，因 perSecret 也为空则端点 403）
	listenAddr, sharedSecret = "127.0.0.1:18923", "  "
	require.NoError(t, runIngest())

	// 非法监听地址 → 报错返回
	listenAddr = "bogus listen addr"
	err := runIngest()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid listen addr")
	waitForShutdownFn = orig
}

// TestRunIngest_Full 驱动完整装配链：noop MQ + 真实 mux + 签名请求走通
// auth → rate limit → metrics → handler 全链路。waitForShutdownFn 接缝
// 让 runIngest 在探查完 mux 后立即收尾。
func TestRunIngest_Full(t *testing.T) {
	t.Setenv("ANALYTICS_MQ_TYPE", "noop")
	t.Setenv("ANALYTICS_INGEST_SECRETS", "")

	orig := waitForShutdownFn
	restored := false
	t.Cleanup(func() {
		if !restored {
			waitForShutdownFn = orig
		}
	})

	// 保存并恢复全局 flag 值
	origListen, origSecret, origSkew := listenAddr, sharedSecret, skewSeconds
	origRPS, origBurst, origMB := rateLimitRPS, rateBurst, maxBodySizeMB
	t.Cleanup(func() {
		listenAddr, sharedSecret, skewSeconds = origListen, origSecret, origSkew
		rateLimitRPS, rateBurst, maxBodySizeMB = origRPS, origBurst, origMB
	})

	listenAddr = "127.0.0.1:18924"
	sharedSecret = "e2e-secret"
	skewSeconds = 300
	rateLimitRPS, rateBurst, maxBodySizeMB = 1000, 100, 10

	// 预占端口 → 后台 ListenAndServe 立即失败，确定性覆盖报错日志分支；
	// 探查走 svr.Handler 直调，不依赖监听成功。
	occupied, err := net.Listen("tcp", "127.0.0.1:18924")
	require.NoError(t, err)
	t.Cleanup(func() { _ = occupied.Close() })

	var captured *http.Server
	waitForShutdownFn = func(svr *http.Server, sig <-chan os.Signal) error {
		captured = svr
		// 经装配好的 mux 走一遍健康检查
		w := httptest.NewRecorder()
		svr.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if w.Code != http.StatusOK {
			return fmt.Errorf("healthz status = %d", w.Code)
		}
		// 签名事件请求走完整中间件链（auth → rate limit → metrics → handler）
		body := `[{"game_id":"e2e","env":"prod","ts":"1","event":"login"}]`
		ts := time.Now().Unix()
		req := httptest.NewRequest(http.MethodPost, "/api/ingest/events", strings.NewReader(body))
		req.Header.Set("X-Game-Id", "e2e")
		req.Header.Set("X-Timestamp", strconv.FormatInt(ts, 10))
		req.Header.Set("X-Nonce", "e2e-nonce")
		req.Header.Set("X-Signature", signPayload("e2e-secret", ts, "e2e-nonce", []byte(body)))
		w2 := httptest.NewRecorder()
		svr.Handler.ServeHTTP(w2, req)
		if w2.Code != http.StatusAccepted {
			return fmt.Errorf("events status = %d body = %s", w2.Code, w2.Body.String())
		}
		// 未签名请求应被拒（401）
		w3 := httptest.NewRecorder()
		svr.Handler.ServeHTTP(w3, httptest.NewRequest(http.MethodPost, "/api/ingest/events", strings.NewReader(body)))
		if w3.Code != http.StatusUnauthorized {
			return fmt.Errorf("unsigned status = %d", w3.Code)
		}
		return nil
	}

	require.NoError(t, rootCmd.RunE(rootCmd, nil))
	require.NotNil(t, captured)
	restored = true
	waitForShutdownFn = orig
	require.NoError(t, captured.Shutdown(context.Background()))
	// 等后台 ListenAndServe goroutine 跑完（端口被占应立即报错退出），
	// 避免它与测试进程退出竞态丢覆盖
	time.Sleep(200 * time.Millisecond)
}
