package ops

// Unit tests for the system maintenance section (#52): runtime snapshot (ldflags field
// passthrough / zero-value fallback / start time and uptime calculation) + check update
// (source not configured / newer remote version / already up to date / tag_name shape /
// fetch failure / non-JSON manifest) + the version comparison pure function + handler route
// smoke tests. The update-source reader and remote fetch are injected through package-level
// function variables (same injectable-seam style as the Service header).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubUpdateHooks(t *testing.T, url string, fetch func(ctx context.Context, url string) (string, error)) {
	t.Helper()
	origURL, origFetch := systemUpdateCheckURLFn, systemUpdateFetchFn
	t.Cleanup(func() { systemUpdateCheckURLFn, systemUpdateFetchFn = origURL, origFetch })
	systemUpdateCheckURLFn = func() string { return url }
	if fetch != nil {
		systemUpdateFetchFn = fetch
	}
}

func TestSystemRuntime_SnapshotPassthrough(t *testing.T) {
	start := time.Now().Add(-90 * time.Second)
	resp := systemRuntime(&svc.ServiceContext{
		ServerVersion:   "v1.2.3",
		ServerGitCommit: "abc1234",
		ServerBuildTime: "2026-09-28T00:00:00Z",
		StartTime:       start,
	})

	assert.Equal(t, "v1.2.3", resp.Version)
	assert.Equal(t, "abc1234", resp.GitCommit)
	assert.Equal(t, "2026-09-28T00:00:00Z", resp.BuildTime)
	assert.Equal(t, start.Format(time.RFC3339), resp.StartedAt)
	assert.GreaterOrEqual(t, resp.UptimeSeconds, int64(89))
	assert.LessOrEqual(t, resp.UptimeSeconds, int64(92))
}

// Zero svcCtx (test / unwired scenarios): versions fall back to the ldflags package defaults,
// and when the start time is unknown startedAt is empty and uptime is 0.
func TestSystemRuntime_ZeroContextFallsBackToLdflags(t *testing.T) {
	resp := systemRuntime(&svc.ServiceContext{})

	assert.Equal(t, svc.ServerVersion, resp.Version)
	assert.Equal(t, svc.ServerGitCommit, resp.GitCommit)
	assert.Equal(t, svc.ServerBuildTime, resp.BuildTime)
	assert.Empty(t, resp.StartedAt)
	assert.Zero(t, resp.UptimeSeconds)

	nilResp := systemRuntime(nil)
	assert.Equal(t, svc.ServerVersion, nilResp.Version)
}

func TestSystemCheckUpdate_SourceNotConfigured(t *testing.T) {
	stubUpdateHooks(t, "", nil)

	resp := systemCheckUpdate(context.Background(), &svc.ServiceContext{ServerVersion: "v1.0.0"})
	require.False(t, resp.Checked)
	assert.False(t, resp.HasUpdate)
	assert.Equal(t, "v1.0.0", resp.CurrentVersion)
	assert.Contains(t, resp.Note, "未配置更新检查源")
	assert.Contains(t, resp.Note, "不自动执行升级")
}

func TestSystemCheckUpdate_NewVersionAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":"9.9.9"}`))
	}))
	t.Cleanup(srv.Close)
	stubUpdateHooks(t, srv.URL, nil)

	resp := systemCheckUpdate(context.Background(), &svc.ServiceContext{ServerVersion: "v1.0.0"})
	require.True(t, resp.Checked)
	assert.True(t, resp.HasUpdate)
	assert.Equal(t, "9.9.9", resp.LatestVersion)
	assert.Contains(t, resp.Note, "发现新版本 9.9.9")
}

// Already up to date (both the v-prefix and latestVersion key shapes).
func TestSystemCheckUpdate_UpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"latestVersion":"v1.0.0"}`))
	}))
	t.Cleanup(srv.Close)
	stubUpdateHooks(t, srv.URL, nil)

	resp := systemCheckUpdate(context.Background(), &svc.ServiceContext{ServerVersion: "v1.0.0"})
	require.True(t, resp.Checked)
	assert.False(t, resp.HasUpdate)
	assert.Contains(t, resp.Note, "已是最新版本")
}

// tag_name key shape (GitHub releases/latest).
func TestSystemCheckUpdate_GitHubTagNameShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v2.0.0","name":"release"}`))
	}))
	t.Cleanup(srv.Close)
	stubUpdateHooks(t, srv.URL, nil)

	resp := systemCheckUpdate(context.Background(), &svc.ServiceContext{ServerVersion: "v1.0.0"})
	require.True(t, resp.Checked)
	assert.True(t, resp.HasUpdate)
	assert.Equal(t, "v2.0.0", resp.LatestVersion)
}

func TestSystemCheckUpdate_FetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	stubUpdateHooks(t, srv.URL, nil)

	resp := systemCheckUpdate(context.Background(), &svc.ServiceContext{ServerVersion: "v1.0.0"})
	assert.False(t, resp.Checked)
	assert.Contains(t, resp.Note, "更新检查失败")
	assert.Contains(t, resp.Note, "500")
}

func TestSystemCheckUpdate_InvalidManifest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	t.Cleanup(srv.Close)
	stubUpdateHooks(t, srv.URL, nil)

	resp := systemCheckUpdate(context.Background(), &svc.ServiceContext{ServerVersion: "v1.0.0"})
	assert.False(t, resp.Checked)
	assert.Contains(t, resp.Note, "不是 JSON 版本清单")
}

// Non-numeric version segments: do not guess an ordering, honestly report "incomparable".
func TestSystemCheckUpdate_IncomparableVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":"2026.09-beta"}`))
	}))
	t.Cleanup(srv.Close)
	stubUpdateHooks(t, srv.URL, nil)

	resp := systemCheckUpdate(context.Background(), &svc.ServiceContext{ServerVersion: "v1.0.0"})
	assert.False(t, resp.HasUpdate)
	assert.Contains(t, resp.Note, "无法比较")
}

func TestCompareVersions_Table(t *testing.T) {
	cases := []struct {
		a, b string
		want int // -1/0/1; -2 = parse failure
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "1.10.0", -1}, // numeric comparison, not lexicographic
		{"2.0.0", "1.99.99", 1},
		{"v1.2.3", "1.2.3", 0}, // v prefix normalized
		{"1.2", "1.2.0", 0},    // missing segment padded with 0
		{"1.2", "1.2.1", -1},
		{"dev", "1.0.0", -2}, // non-numeric segment → incomparable
		{"", "1.0.0", -2},    // empty version string
	}
	for _, tc := range cases {
		va, errA := parseVersionParts(tc.a)
		vb, errB := parseVersionParts(tc.b)
		if tc.want == -2 {
			assert.True(t, errA != nil || errB != nil, "parse(%q,%q) 应报错", tc.a, tc.b)
			continue
		}
		require.NoError(t, errA)
		require.NoError(t, errB)
		got := compareVersions(va, vb)
		if got < 0 {
			got = -1
		} else if got > 0 {
			got = 1
		}
		assert.Equal(t, tc.want, got, "compare(%q,%q)", tc.a, tc.b)
	}
}

// ---- handler-layer smoke tests (real handler methods → response.Success direct-return contract) ----

func setupSystemRouter() (*gin.Engine, *Handler) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(NewService(&svc.ServiceContext{ServerVersion: "v9.8.7"}))
	r.GET("/api/v1/ops/system/runtime", h.SystemRuntime)
	r.POST("/api/v1/ops/system/check-update", h.SystemCheckUpdate)
	return r, h
}

func TestHandler_SystemRuntimeRoute(t *testing.T) {
	r, _ := setupSystemRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ops/system/runtime", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"version":"v9.8.7"`)
	assert.Contains(t, w.Body.String(), `"uptimeSeconds":0`)
}

func TestHandler_SystemCheckUpdateRoute(t *testing.T) {
	stubUpdateHooks(t, "", nil)
	r, _ := setupSystemRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/ops/system/check-update", nil))
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"checked":false`)
	assert.Contains(t, body, "未配置更新检查源")
}
