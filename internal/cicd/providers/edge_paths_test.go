package providers_test

// provider 边界翼补测（覆盖率巡检）：四个 provider 的 Kind 契约、构造默认
// 兜底（client/now 缺省、token 形态）、出站传输失败、URL 解析失败、非 2xx、
// 畸形 JSON、状态映射全矩阵、空标识域与时间字段解析。
//
// 手法统一：① epFailClient 恒传输失败打穿 client.Do 错误翼；② epBadURL 过
// provider 的「必须 http(s) 前缀」校验但让 http.NewRequestWithContext 解析
// 失败；③ 假服务器回固定状态码/负载覆盖非 2xx 与解码失败；④ 状态映射一律
// 经真实 HTTP 面验证（不直接调未导出的归一函数）。

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/cicd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- 通用夹具 ----

// epBadURLEndpoint 能过各 provider 的「必须 http(s) 前缀」校验，但主机名含
// 空格 → url.Parse 在 http.NewRequestWithContext 处失败。
const epBadURLEndpoint = "http://exa mple.com"

// epFailTransport 恒定传输失败，模拟目标系统不可达/连接被拒。
type epFailTransport struct{}

func (epFailTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("dial tcp: connection refused")
}

// epFailClient 返回走恒失败 Transport 的客户端。
func epFailClient() *http.Client { return &http.Client{Transport: epFailTransport{}} }

// epFixedServer 对任意路径回固定状态码与负载。
func epFixedServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ---- Kind 契约与构造默认兜底 ----

// TestProviders_KindContract 四个 provider 的 Kind 必须与注册名一致。
func TestProviders_KindContract(t *testing.T) {
	srv := epFixedServer(t, http.StatusOK, `{}`)
	cases := []struct {
		kind string
		cfg  cicd.Config
	}{
		{"jenkins", cicd.Config{Endpoint: srv.URL}},
		{"gitlab-ci", cicd.Config{Endpoint: srv.URL, Extra: map[string]string{"project": "1"}}},
		{"github-actions", cicd.Config{Endpoint: srv.URL, Extra: map[string]string{"repo": "o/r"}}},
		{"generic", cicd.Config{Extra: map[string]string{"statusUrl": srv.URL + "/s/{id}"}}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			p, err := cicd.New(tc.kind, tc.cfg)
			require.NoError(t, err)
			assert.Equal(t, tc.kind, p.Kind())
		})
	}
	assert.Contains(t, cicd.Kinds(), "generic")
}

// TestProviders_NilClientDefaults Config.HTTP/Now 缺省时回落 http.DefaultClient
// 与 time.Now，且仍能正常出站（生产路径的兜底语义）。
func TestProviders_NilClientDefaults(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/s/{id}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"b-1","status":"ok"}`))
	})
	mux.HandleFunc("/t", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"b-2"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// generic：仅注入 URL，未注入 client/now → 默认 client 直连本地假服务器
	p, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{"statusUrl": srv.URL + "/s/{id}", "triggerUrl": srv.URL + "/t"},
	})
	require.NoError(t, err)
	ref, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "x"})
	require.NoError(t, err)
	assert.Equal(t, "b-2", ref.ExternalID)

	for _, cfg := range []cicd.Config{
		{Endpoint: srv.URL, Extra: map[string]string{"job": "j"}},
		{Endpoint: srv.URL, Token: "raw-token"},
	} {
		_, err = cicd.New("jenkins", cfg)
		require.NoError(t, err)
	}
	_, err = cicd.New("gitlab-ci", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"project": "9"},
	})
	require.NoError(t, err)
	_, err = cicd.New("github-actions", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"repo": "o/r"},
	})
	require.NoError(t, err)
}

// ---- generic ----

// TestGenericProvider_ConfigValidationWings 构造期剩余 URL 形态分支与空格 URL
// 的请求构造失败。
func TestGenericProvider_ConfigValidationWings(t *testing.T) {
	_, err := cicd.New("generic", cicd.Config{Extra: map[string]string{"statusUrl": "ftp://x"}})
	require.ErrorContains(t, err, "statusUrl")

	_, err = cicd.New("generic", cicd.Config{Extra: map[string]string{"pipeline": "p"}})
	require.ErrorContains(t, err, "至少配置一个")

	// 过前缀校验但 url.Parse 失败 → 构造期不报，出站时报 NewRequest 错误
	p, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{
			"triggerUrl": epBadURLEndpoint + "/t", "statusUrl": epBadURLEndpoint + "/s/{id}",
		},
	})
	require.NoError(t, err)
	_, err = p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "x"})
	require.Error(t, err)
	_, err = p.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.Error(t, err)
	_, err = p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.Error(t, err)
}

// TestGenericProvider_TriggerWings pipeline 回退链、出站失败、非 2xx 与
// id 缺省回退 Location 头。
func TestGenericProvider_TriggerWings(t *testing.T) {
	state := &struct{ body, loc, ct string }{}
	mux := http.NewServeMux()
	mux.HandleFunc("/t-ok", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		state.body = string(buf[:n])
		state.ct = r.Header.Get("Content-Type")
		w.Header().Set("Location", "http://ci/queue/9")
		_, _ = w.Write([]byte(`{"webUrl":"http://ci/9"}`)) // 无 id → 回退 Location
	})
	mux.HandleFunc("/t-500", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("  boom  "))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{
			"triggerUrl": srv.URL + "/t-ok", "statusUrl": srv.URL + "/s/{id}",
			"pipeline": "default-pipeline",
		},
		HTTP: srv.Client(),
	})
	require.NoError(t, err)

	// Pipeline 未指定 → 回退 Extra["pipeline"]
	ref, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{})
	require.NoError(t, err)
	assert.Equal(t, "default-pipeline", ref.Pipeline)
	assert.Equal(t, "http://ci/queue/9", ref.ExternalID, "id 缺省应回退 Location 头")
	assert.Equal(t, "http://ci/9", ref.WebURL)
	assert.Equal(t, "application/json", state.ct)
	assert.Contains(t, state.body, `"pipeline":"default-pipeline"`)

	// pipeline 与默认值均为空白 → 报错
	_, err = p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "   "})
	require.ErrorContains(t, err, "pipeline")

	// 传恒失败 client → 出站错误透传
	fail, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{"triggerUrl": srv.URL + "/t-ok"},
		HTTP:  epFailClient(),
	})
	require.NoError(t, err)
	_, err = fail.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "x"})
	require.ErrorContains(t, err, "connection refused")

	// 非 2xx → 错误含状态码与截断的响应体
	bad, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{"triggerUrl": srv.URL + "/t-500"},
		HTTP:  srv.Client(),
	})
	require.NoError(t, err)
	_, err = bad.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "x"})
	require.ErrorContains(t, err, "trigger http 500")
	require.ErrorContains(t, err, "boom")
}

// TestGenericProvider_GetBuildWings 状态拉取的空 statusUrl / 空 id / 非 200 /
// 畸形 JSON / 传输失败。
func TestGenericProvider_GetBuildWings(t *testing.T) {
	// 未配置 statusUrl
	trig, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{"triggerUrl": epBadURLEndpoint},
	})
	require.NoError(t, err)
	_, err = trig.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.ErrorContains(t, err, "statusUrl")
	_, err = trig.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.ErrorContains(t, err, "statusUrl")

	// 缺构建 id
	ok, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{"statusUrl": epBadURLEndpoint + "/s/{id}"},
	})
	require.NoError(t, err)
	_, err = ok.GetBuild(context.Background(), cicd.BuildRef{})
	require.ErrorContains(t, err, "构建 id")

	// 非 200
	srv404 := epFixedServer(t, http.StatusNotFound, `nope`)
	p404, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{"statusUrl": srv404.URL + "/s/{id}"}, HTTP: srv404.Client(),
	})
	require.NoError(t, err)
	_, err = p404.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.ErrorContains(t, err, "status http 404")

	// 畸形 JSON → 解码错误
	srvBad := epFixedServer(t, http.StatusOK, `{"status":`)
	pBad, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{"statusUrl": srvBad.URL + "/s/{id}"}, HTTP: srvBad.Client(),
	})
	require.NoError(t, err)
	_, err = pBad.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.Error(t, err)

	// 传输失败
	pFail, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{"statusUrl": "http://ci.invalid/s/{id}"}, HTTP: epFailClient(),
	})
	require.NoError(t, err)
	_, err = pFail.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.ErrorContains(t, err, "connection refused")
}

// TestGenericProvider_ListArtifacts 有 artifactUrl 时合成单条目；无则回非 nil
// 空切片；状态失败时透传错误。
func TestGenericProvider_ListArtifacts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/s/with", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","artifactUrl":"http://ci/a.zip","checksum":"c1"}`))
	})
	mux.HandleFunc("/s/without", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"running"}`))
	})
	mux.HandleFunc("/s/boom", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{"statusUrl": srv.URL + "/s/{id}"}, HTTP: srv.Client(),
	})
	require.NoError(t, err)

	arts, err := p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "with"})
	require.NoError(t, err)
	require.Len(t, arts, 1)
	assert.Equal(t, "artifact", arts[0].Name)
	assert.Equal(t, "artifact", arts[0].Path)
	assert.Equal(t, "http://ci/a.zip", arts[0].URL)

	empty, err := p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "without"})
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)

	_, err = p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "boom"})
	require.ErrorContains(t, err, "status http 502")
}

// TestGenericProvider_StatusMatrix 状态词归一全矩阵（大小写/空白不敏感，未
// 识别回落 unknown），并顺带验证 startedAt/finishedAt 的解析与丢弃。
func TestGenericProvider_StatusMatrix(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"queued", cicd.StatusQueued}, {"PENDING", cicd.StatusQueued},
		{"created", cicd.StatusQueued}, {" waiting ", cicd.StatusQueued},
		{"preparing", cicd.StatusQueued},
		{"running", cicd.StatusRunning}, {"in_progress", cicd.StatusRunning},
		{"Building", cicd.StatusRunning},
		{"success", cicd.StatusSuccess}, {"succeeded", cicd.StatusSuccess},
		{"ok", cicd.StatusSuccess}, {"passed", cicd.StatusSuccess},
		{"done", cicd.StatusSuccess}, {"complete", cicd.StatusSuccess},
		{"completed", cicd.StatusSuccess},
		{"failed", cicd.StatusFailed}, {"failure", cicd.StatusFailed},
		{"error", cicd.StatusFailed}, {"errored", cicd.StatusFailed},
		{"unstable", cicd.StatusFailed},
		{"cancelled", cicd.StatusCancelled}, {"canceled", cicd.StatusCancelled},
		{"aborted", cicd.StatusCancelled}, {"skipped", cicd.StatusCancelled},
		{"weird", cicd.StatusUnknown}, {"", cicd.StatusUnknown},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.raw), func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/s/{id}", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w,
					`{"status":%q,"startedAt":"2026-09-29T00:00:00Z","finishedAt":"not-a-time"}`,
					tc.raw)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			p, err := cicd.New("generic", cicd.Config{
				Extra: map[string]string{"statusUrl": srv.URL + "/s/{id}"}, HTTP: srv.Client(),
			})
			require.NoError(t, err)
			st, err := p.GetBuild(context.Background(), cicd.BuildRef{Pipeline: "pl", ExternalID: "b1"})
			require.NoError(t, err)
			assert.Equal(t, tc.want, st.Status)
			assert.Equal(t, "pl", st.Pipeline)
			assert.Equal(t, "b1", st.ExternalID)
			require.NotNil(t, st.StartedAt, "合法 RFC3339 应解析")
			assert.Nil(t, st.FinishedAt, "非法时间应回落 nil")
		})
	}
}

// ---- github-actions ----

// TestGitHubActionsProvider_ConfigWings endpoint 形态校验与默认 ref=main。
func TestGitHubActionsProvider_ConfigWings(t *testing.T) {
	_, err := cicd.New("github-actions", cicd.Config{
		Endpoint: "ftp://x", Extra: map[string]string{"repo": "o/r"},
	})
	require.ErrorContains(t, err, "endpoint")

	srv := epFixedServer(t, http.StatusNoContent, ``)
	p, err := cicd.New("github-actions", cicd.Config{
		Endpoint: srv.URL + "/", // 尾斜杠应被 TrimRight 修掉
		Extra:    map[string]string{"repo": "o/r", "workflow": "w.yml"},
	})
	require.NoError(t, err)
	// 未配 ref → 默认 main
	ref, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{})
	require.NoError(t, err)
	assert.Equal(t, "w.yml", ref.Pipeline)
	assert.Equal(t, "https://github.com/o/r/actions/workflows/w.yml", ref.WebURL)
}

// TestGitHubActionsProvider_TriggerWings 无 token 不带 Authorization、ref
// 参数覆盖、inputs 剔除 ref/branch、传输失败、非 204。
func TestGitHubActionsProvider_TriggerWings(t *testing.T) {
	state := &struct{ auth, accept, body string }{}
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/actions/workflows/w.yml/dispatches",
		func(w http.ResponseWriter, r *http.Request) {
			state.auth = r.Header.Get("Authorization")
			state.accept = r.Header.Get("Accept")
			buf := make([]byte, 512)
			n, _ := r.Body.Read(buf)
			state.body = string(buf[:n])
			w.WriteHeader(http.StatusNoContent)
		})
	mux.HandleFunc("/repos/o/r/actions/workflows/bad.yml/dispatches",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("no access"))
		})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// 无 token → 不带 Authorization 头
	p, err := cicd.New("github-actions", cicd.Config{
		Endpoint: srv.URL, HTTP: srv.Client(),
		Extra: map[string]string{"repo": "o/r", "workflow": "w.yml", "ref": "main"},
	})
	require.NoError(t, err)
	_, err = p.TriggerBuild(context.Background(), cicd.TriggerRequest{
		Params: map[string]string{"branch": "release", "env": "prod", "ref": "ignored"},
	})
	require.NoError(t, err)
	assert.Empty(t, state.auth, "无 token 不应带 Authorization")
	assert.Equal(t, "application/vnd.github+json", state.accept)
	assert.Contains(t, state.body, `"ref":"release"`, "branch 参数应覆盖默认 ref")
	assert.Contains(t, state.body, `"inputs":{"env":"prod"}`, "ref/branch 不应进 inputs")

	// workflow 缺失（请求与配置都无）→ 报错
	noWf, err := cicd.New("github-actions", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"repo": "o/r"}, HTTP: srv.Client(),
	})
	require.NoError(t, err)
	_, err = noWf.TriggerBuild(context.Background(), cicd.TriggerRequest{})
	require.ErrorContains(t, err, "workflow")

	// 传输失败
	fail, err := cicd.New("github-actions", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"repo": "o/r", "workflow": "w.yml"},
		HTTP: epFailClient(),
	})
	require.NoError(t, err)
	_, err = fail.TriggerBuild(context.Background(), cicd.TriggerRequest{})
	require.ErrorContains(t, err, "connection refused")

	// 非 204
	bad, err := cicd.New("github-actions", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"repo": "o/r", "workflow": "bad.yml"},
		HTTP: srv.Client(),
	})
	require.NoError(t, err)
	_, err = bad.TriggerBuild(context.Background(), cicd.TriggerRequest{})
	require.ErrorContains(t, err, "dispatch http 403")
	require.ErrorContains(t, err, "no access")
}

// TestGitHubActionsProvider_GetBuildWings run 查询与最近 run 查询的错误域。
func TestGitHubActionsProvider_GetBuildWings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/actions/runs/7", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/repos/o/r/actions/runs/8", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":`))
	})
	mux.HandleFunc("/repos/o/r/actions/runs/8/artifacts", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{`))
	})
	mux.HandleFunc("/repos/o/r/actions/workflows/empty.yml/runs",
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"workflow_runs":[]}`))
		})
	mux.HandleFunc("/repos/o/r/actions/workflows/broken.yml/runs",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
	mux.HandleFunc("/repos/o/r/actions/workflows/garbage.yml/runs",
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`nope`))
		})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	base := cicd.Config{Endpoint: srv.URL, Extra: map[string]string{"repo": "o/r"}, HTTP: srv.Client()}

	p, err := cicd.New("github-actions", base)
	require.NoError(t, err)
	// ref.Pipeline 缺省 → 回退 Extra["workflow"]；此处两者皆空 → 报错
	_, err = p.GetBuild(context.Background(), cicd.BuildRef{})
	require.ErrorContains(t, err, "workflow")

	cfgWF := base
	cfgWF.Extra = map[string]string{"repo": "o/r", "workflow": "empty.yml"}
	pWF, err := cicd.New("github-actions", cfgWF)
	require.NoError(t, err)
	_, err = pWF.GetBuild(context.Background(), cicd.BuildRef{})
	require.ErrorContains(t, err, "暂无运行记录")

	// 带 run id 的错误域
	_, err = p.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "7"})
	require.ErrorContains(t, err, "run http 404")
	_, err = p.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "8"})
	require.Error(t, err)
	_, err = p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "7"})
	require.ErrorContains(t, err, "artifacts http 404")
	_, err = p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "8"})
	require.Error(t, err)
	_, err = p.ListArtifacts(context.Background(), cicd.BuildRef{})
	require.ErrorContains(t, err, "run id")

	// 最近 run 查询的错误域
	for _, wf := range []string{"broken.yml", "garbage.yml"} {
		c := base
		c.Extra = map[string]string{"repo": "o/r", "workflow": wf}
		pp, err := cicd.New("github-actions", c)
		require.NoError(t, err)
		_, err = pp.GetBuild(context.Background(), cicd.BuildRef{})
		require.Error(t, err, wf)
	}

	// 传输失败
	pf, err := cicd.New("github-actions", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"repo": "o/r", "workflow": "w.yml"},
		HTTP: epFailClient(),
	})
	require.NoError(t, err)
	_, err = pf.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "9"})
	require.ErrorContains(t, err, "connection refused")
	_, err = pf.GetBuild(context.Background(), cicd.BuildRef{})
	require.ErrorContains(t, err, "connection refused")
	_, err = pf.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "9"})
	require.ErrorContains(t, err, "connection refused")

	// 空格 URL → 请求构造失败
	pBad, err := cicd.New("github-actions", cicd.Config{
		Endpoint: epBadURLEndpoint, Extra: map[string]string{"repo": "o/r", "workflow": "w.yml"},
	})
	require.NoError(t, err)
	_, err = pBad.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "9"})
	require.Error(t, err)
}

// TestGitHubActionsProvider_StatusMatrix run 状态 × conclusion 映射全矩阵。
func TestGitHubActionsProvider_StatusMatrix(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"queued", `{"id":1,"status":"queued"}`, cicd.StatusQueued},
		{"requested", `{"id":1,"status":"requested"}`, cicd.StatusQueued},
		{"waiting", `{"id":1,"status":"waiting"}`, cicd.StatusQueued},
		{"pending", `{"id":1,"status":"pending"}`, cicd.StatusQueued},
		{"in_progress", `{"id":1,"status":"in_progress"}`, cicd.StatusRunning},
		{"completed_no_conclusion", `{"id":1,"status":"completed"}`, cicd.StatusUnknown},
		{"success", `{"id":1,"status":"completed","conclusion":"success"}`, cicd.StatusSuccess},
		{"failure", `{"id":1,"status":"completed","conclusion":"failure"}`, cicd.StatusFailed},
		{"timed_out", `{"id":1,"status":"completed","conclusion":"timed_out"}`, cicd.StatusFailed},
		{"startup_failure", `{"id":1,"status":"completed","conclusion":"startup_failure"}`, cicd.StatusFailed},
		{"cancelled", `{"id":1,"status":"completed","conclusion":"cancelled"}`, cicd.StatusCancelled},
		{"neutral", `{"id":1,"status":"completed","conclusion":"neutral"}`, cicd.StatusUnknown},
		{"weird_status", `{"id":1,"status":"weird"}`, cicd.StatusUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/repos/o/r/actions/runs/5", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			p, err := cicd.New("github-actions", cicd.Config{
				Endpoint: srv.URL, Extra: map[string]string{"repo": "o/r"}, HTTP: srv.Client(),
			})
			require.NoError(t, err)
			st, err := p.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "5"})
			require.NoError(t, err)
			assert.Equal(t, tc.want, st.Status)
			assert.Equal(t, "1", st.ExternalID, "ExternalID 回填为 run id")
		})
	}
}

// TestGitHubActionsProvider_ArtifactsEmptyList 产物列表为空数组时回非 nil 空
// 切片（消费方按切片语义判断）。
func TestGitHubActionsProvider_ArtifactsEmptyList(t *testing.T) {
	srv := epFixedServer(t, http.StatusOK, `{"total_count":0,"artifacts":[]}`)
	p, err := cicd.New("github-actions", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"repo": "o/r"}, HTTP: srv.Client(),
	})
	require.NoError(t, err)
	arts, err := p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.NoError(t, err)
	assert.NotNil(t, arts)
	assert.Empty(t, arts)
}

// ---- gitlab-ci ----

// TestGitlabCIProvider_TriggerWings ref 回退、variables 拼接、project 路径
// 转义、PRIVATE-TOKEN 头、非 2xx、畸形 JSON、传输失败。
func TestGitlabCIProvider_TriggerWings(t *testing.T) {
	state := &struct{ path, token string }{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.path = r.URL.String()
		state.token = r.Header.Get("PRIVATE-TOKEN")
		switch r.URL.EscapedPath() {
		case "/api/v4/projects/7/pipeline":
			switch r.URL.Query().Get("ref") {
			case "boom":
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"kaboom"}`))
			case "garbage":
				_, _ = w.Write([]byte(`{`))
			default:
				_, _ = w.Write([]byte(`{"id":32,"web_url":"http://gl/32"}`))
			}
		case "/api/v4/projects/group%2Frepo/pipeline":
			_, _ = w.Write([]byte(`{"id":31,"web_url":"http://gl/31","ref":"dev"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	// 纯数字 project 不转义；无 token 不带 PRIVATE-TOKEN；ref 回退 Extra
	p, err := cicd.New("gitlab-ci", cicd.Config{
		Endpoint: srv.URL, HTTP: srv.Client(),
		Extra: map[string]string{"project": "7", "ref": "main"},
	})
	require.NoError(t, err)
	ref, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{
		Params: map[string]string{"BRANCH": "main"},
	})
	require.NoError(t, err)
	assert.Equal(t, "main", ref.Pipeline)
	assert.Equal(t, "32", ref.ExternalID)
	assert.Contains(t, state.path, "ref=main")
	assert.Contains(t, state.path, "variables%5BBRANCH%5D=main")
	assert.Empty(t, state.token)

	// group/repo 形态走 PathEscape 且带上 token 头
	pr, err := cicd.New("gitlab-ci", cicd.Config{
		Endpoint: srv.URL, HTTP: srv.Client(), Token: "glpat-1",
		Extra: map[string]string{"project": "group/repo"},
	})
	require.NoError(t, err)
	ref2, err := pr.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "dev"})
	require.NoError(t, err)
	assert.Equal(t, "31", ref2.ExternalID)
	assert.Equal(t, "http://gl/31", ref2.WebURL)
	assert.Equal(t, "glpat-1", state.token)
	assert.Contains(t, state.path, "/api/v4/projects/group%2Frepo/pipeline")

	// ref 与默认 ref 均空 → 报错
	noRef, err := cicd.New("gitlab-ci", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"project": "7"}, HTTP: srv.Client(),
	})
	require.NoError(t, err)
	_, err = noRef.TriggerBuild(context.Background(), cicd.TriggerRequest{})
	require.ErrorContains(t, err, "ref")

	// 非 2xx
	_, err = p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "boom"})
	require.ErrorContains(t, err, "trigger http 500")
	require.ErrorContains(t, err, "kaboom")
	// 2xx 但响应体畸形
	_, err = p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "garbage"})
	require.Error(t, err)

	// 传输失败
	pf, err := cicd.New("gitlab-ci", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"project": "7"}, HTTP: epFailClient(),
	})
	require.NoError(t, err)
	_, err = pf.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "main"})
	require.ErrorContains(t, err, "connection refused")

	// 空格 URL → 请求构造失败
	pb, err := cicd.New("gitlab-ci", cicd.Config{
		Endpoint: epBadURLEndpoint, Extra: map[string]string{"project": "7"},
	})
	require.NoError(t, err)
	_, err = pb.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "main"})
	require.Error(t, err)
}

// TestGitlabCIProvider_StatusMatrix pipeline 状态词全矩阵 + 时间字段四形态。
func TestGitlabCIProvider_StatusMatrix(t *testing.T) {
	cases := []struct {
		raw, want         string
		started, finished string
		wantStarted       bool
		wantFinished      bool
	}{
		{"created", cicd.StatusQueued,
			`"2026-09-29T01:02:03Z"`, `"2026-09-29T01:03:03Z"`, true, true},
		{"waiting_for_resource", cicd.StatusQueued, "null", "null", false, false},
		{"pending", cicd.StatusQueued, `"2026-09-29T01:02:03Z"`, `""`, true, false},
		{"preparing", cicd.StatusQueued, `"bad"`, `"also-bad"`, false, false},
		{"running", cicd.StatusRunning, "null", "null", false, false},
		{"success", cicd.StatusSuccess, "null", "null", false, false},
		{"failed", cicd.StatusFailed, "null", "null", false, false},
		{"canceled", cicd.StatusCancelled, "null", "null", false, false},
		{"skipped", cicd.StatusCancelled, "null", "null", false, false},
		{"weird", cicd.StatusUnknown, "null", "null", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v4/projects/7/pipelines/1", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w,
					`{"id":1,"status":%q,"web_url":"http://gl/1","started_at":%s,"finished_at":%s}`,
					tc.raw, tc.started, tc.finished)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			p, err := cicd.New("gitlab-ci", cicd.Config{
				Endpoint: srv.URL, Extra: map[string]string{"project": "7"}, HTTP: srv.Client(),
			})
			require.NoError(t, err)
			st, err := p.GetBuild(context.Background(), cicd.BuildRef{Pipeline: "main", ExternalID: "1"})
			require.NoError(t, err)
			assert.Equal(t, tc.want, st.Status)
			assert.Equal(t, "http://gl/1", st.WebURL)
			if tc.wantStarted {
				assert.NotNil(t, st.StartedAt)
			} else {
				assert.Nil(t, st.StartedAt)
			}
			if tc.wantFinished {
				assert.NotNil(t, st.FinishedAt)
			} else {
				assert.Nil(t, st.FinishedAt)
			}
		})
	}
}

// TestGitlabCIProvider_GetBuildWings 空 pipeline id、非 200、畸形 JSON、传输
// 失败；ListArtifacts 同族四翼 + 非 success job 过滤。
func TestGitlabCIProvider_GetBuildWings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/7/pipelines/1", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":1,"status":"running"}`))
	})
	mux.HandleFunc("/api/v4/projects/7/pipelines/2", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/api/v4/projects/7/pipelines/3", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[`))
	})
	mux.HandleFunc("/api/v4/projects/7/pipelines/1/jobs", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":5,"name":"build","status":"success","stage":"build"},` +
			`{"id":6,"name":"test","status":"failed","stage":"test"}]`))
	})
	mux.HandleFunc("/api/v4/projects/7/pipelines/2/jobs", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/api/v4/projects/7/pipelines/3/jobs", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfg := cicd.Config{Endpoint: srv.URL, Extra: map[string]string{"project": "7"}, HTTP: srv.Client()}
	p, err := cicd.New("gitlab-ci", cfg)
	require.NoError(t, err)

	_, err = p.GetBuild(context.Background(), cicd.BuildRef{})
	require.ErrorContains(t, err, "pipeline id")
	_, err = p.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "  "})
	require.ErrorContains(t, err, "pipeline id")
	_, err = p.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "2"})
	require.ErrorContains(t, err, "build http 403")
	_, err = p.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "3"})
	require.Error(t, err)

	_, err = p.ListArtifacts(context.Background(), cicd.BuildRef{})
	require.ErrorContains(t, err, "pipeline id")
	_, err = p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "2"})
	require.ErrorContains(t, err, "jobs http 404")
	_, err = p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "3"})
	require.Error(t, err)

	arts, err := p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.NoError(t, err)
	require.Len(t, arts, 1, "非 success job 应被过滤")
	assert.Equal(t, "build/build", arts[0].Path)
	assert.Equal(t, srv.URL+"/api/v4/projects/7/jobs/5/artifacts", arts[0].URL)

	pf, err := cicd.New("gitlab-ci", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"project": "7"}, HTTP: epFailClient(),
	})
	require.NoError(t, err)
	_, err = pf.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.ErrorContains(t, err, "connection refused")
	_, err = pf.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.ErrorContains(t, err, "connection refused")

	pb, err := cicd.New("gitlab-ci", cicd.Config{
		Endpoint: epBadURLEndpoint, Extra: map[string]string{"project": "7"},
	})
	require.NoError(t, err)
	_, err = pb.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.Error(t, err)
	_, err = pb.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "1"})
	require.Error(t, err)
}

// ---- jenkins ----

// TestJenkinsProvider_TriggerWings job 回退、buildWithParameters、Location 重写
// 与解析失败、无 Location、crumb 关闭、非 2xx、job 缺失、传输失败。
func TestJenkinsProvider_TriggerWings(t *testing.T) {
	state := &struct {
		path, crumb, user, pass string
	}{}
	mux := http.NewServeMux()
	mux.HandleFunc("/crumbIssuer/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"crumb":"cr","crumbRequestField":"J-Crumb"}`))
	})
	build := func(w http.ResponseWriter, r *http.Request) {
		state.path = r.URL.RequestURI()
		state.crumb = r.Header.Get("J-Crumb")
		state.user, state.pass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusCreated)
	}
	mux.HandleFunc("/job/nc/build", build)
	mux.HandleFunc("/job/nc/buildWithParameters", func(w http.ResponseWriter, r *http.Request) {
		state.path = r.URL.RequestURI()
		w.Header().Set("Location", "http://svc/queue/item/3/")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/job/locrel/build", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "http://svc") // 无路径
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/job/locbad/build", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "http://[::1") // url.Parse 失败
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/job/err500/build", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("broken"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p, err := cicd.New("jenkins", cicd.Config{
		Endpoint: srv.URL, HTTP: srv.Client(), Token: "u1:tok-1",
		Extra: map[string]string{"job": "nc", "crumbDisabled": "TRUE"},
	})
	require.NoError(t, err)

	ref, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{})
	require.NoError(t, err)
	assert.Equal(t, "nc", ref.Pipeline)
	assert.Empty(t, ref.ExternalID, "无 Location 头时 ExternalID 为空")
	assert.Equal(t, "/job/nc/build", state.path)
	assert.Empty(t, state.crumb, "crumbDisabled 应跳过 crumb")
	assert.Equal(t, "u1", state.user)
	assert.Equal(t, "tok-1", state.pass)

	// 带参数 → buildWithParameters + query 拼接 + Location 重写到真实 host
	ref2, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{
		Pipeline: "nc", Params: map[string]string{"A": "1", "B": "2"},
	})
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/queue/item/3/", ref2.ExternalID)
	assert.Equal(t, "/job/nc/buildWithParameters?A=1&B=2", state.path)

	// Location 无路径 → 保持原样
	ref3, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "locrel"})
	require.NoError(t, err)
	assert.Equal(t, "http://svc", ref3.ExternalID)

	// Location 不可解析 → 保持原样
	ref4, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "locbad"})
	require.NoError(t, err)
	assert.Equal(t, "http://[::1", ref4.ExternalID)

	// 非 2xx
	_, err = p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "err500"})
	require.ErrorContains(t, err, "trigger http 500")
	require.ErrorContains(t, err, "broken")

	// job 缺失
	noJob, err := cicd.New("jenkins", cicd.Config{Endpoint: srv.URL, HTTP: srv.Client()})
	require.NoError(t, err)
	_, err = noJob.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "  "})
	require.ErrorContains(t, err, "job")

	// 传输失败
	pf, err := cicd.New("jenkins", cicd.Config{
		Endpoint: srv.URL, HTTP: epFailClient(),
		Extra: map[string]string{"job": "j", "crumbDisabled": "true"},
	})
	require.NoError(t, err)
	_, err = pf.TriggerBuild(context.Background(), cicd.TriggerRequest{})
	require.ErrorContains(t, err, "connection refused")

	// 空格 URL → 请求构造失败（crumb 取不到不阻断，随后主请求构造失败）
	pb, err := cicd.New("jenkins", cicd.Config{
		Endpoint: epBadURLEndpoint, Extra: map[string]string{"job": "j"},
	})
	require.NoError(t, err)
	_, err = pb.TriggerBuild(context.Background(), cicd.TriggerRequest{})
	require.Error(t, err)
}

// epModeTransport 在 crumbIssuer 请求上注入指定故障（经 X-Mode 头选路），
// 其余请求透传。
type epModeTransport struct {
	base http.RoundTripper
	mode string
}

func (t *epModeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.mode == "dial" && strings.HasSuffix(r.URL.Path, "/crumbIssuer/api/json") {
		return nil, errors.New("dial tcp: crumb issuer unreachable")
	}
	r2 := r.Clone(r.Context())
	r2.Header.Set("X-Mode", t.mode)
	if t.base != nil {
		return t.base.RoundTrip(r2)
	}
	return http.DefaultTransport.RoundTrip(r2)
}

// TestJenkinsProvider_CrumbErrorWings crumb 获取失败（非 200 / 畸形 JSON /
// 传输失败）不得阻断触发——关闭 CSRF 的实例无 crumbIssuer。
func TestJenkinsProvider_CrumbErrorWings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/crumbIssuer/api/json", func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("X-Mode") {
		case "500":
			w.WriteHeader(http.StatusInternalServerError)
		case "garbage":
			_, _ = w.Write([]byte(`{`))
		default:
			_, _ = w.Write([]byte(`{"crumb":"cr","crumbRequestField":"J-Crumb"}`))
		}
	})
	mux.HandleFunc("/job/j/build", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("J-Crumb") == "cr" {
			t.Log("crumb 已附带")
		}
		w.Header().Set("Location", "http://svc/queue/item/1/")
		w.WriteHeader(http.StatusCreated)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	for _, mode := range []string{"ok", "500", "garbage", "dial"} {
		t.Run(mode, func(t *testing.T) {
			p, err := cicd.New("jenkins", cicd.Config{
				Endpoint: srv.URL,
				HTTP:     &http.Client{Transport: &epModeTransport{base: srv.Client().Transport, mode: mode}},
				Extra:    map[string]string{"job": "j"},
			})
			require.NoError(t, err)
			ref, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{})
			require.NoError(t, err, "crumb 获取失败不得阻断触发")
			assert.Equal(t, srv.URL+"/queue/item/1/", ref.ExternalID)
		})
	}
}

// TestJenkinsProvider_ResolveBuildWings 队列项四种结果、job#n 与纯 n 两种
// ExternalID 形态、构建详情与 lastBuild 错误域、job 缺失、传输失败。
func TestJenkinsProvider_ResolveBuildWings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/queue/item/1/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"executable":null}`)) // 仍在排队
	})
	mux.HandleFunc("/queue/item/2/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"executable":{"number":0}}`)) // 0 号构建
	})
	mux.HandleFunc("/queue/item/3/api/json", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/queue/item/4/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[1,`))
	})
	mux.HandleFunc("/job/j/77/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"number":77,"building":false,"result":"ABORTED",` +
			`"url":"http://svc/job/j/77/","timestamp":1727500000000,"duration":1000,` +
			`"artifacts":[{"relativePath":"a/b.zip","fileName":"b.zip"}]}`))
	})
	mux.HandleFunc("/job/j/78/api/json", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/job/j/79/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{`))
	})
	mux.HandleFunc("/job/j/lastBuild/api/json", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mux.HandleFunc("/job/jb/lastBuild/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p, err := cicd.New("jenkins", cicd.Config{
		Endpoint: srv.URL, HTTP: srv.Client(), Extra: map[string]string{"job": "j"},
	})
	require.NoError(t, err)
	ctx := context.Background()

	// 队列项仍排队 → queued，WebURL 回填队列 URL
	st, err := p.GetBuild(ctx, cicd.BuildRef{Pipeline: "j", ExternalID: srv.URL + "/queue/item/1/"})
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusQueued, st.Status)
	assert.Equal(t, srv.URL+"/queue/item/1/", st.WebURL)

	// executable.number == 0 → 同为排队态
	st, err = p.GetBuild(ctx, cicd.BuildRef{Pipeline: "j", ExternalID: srv.URL + "/queue/item/2/"})
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusQueued, st.Status)

	// 队列项错误域
	_, err = p.GetBuild(ctx, cicd.BuildRef{Pipeline: "j", ExternalID: srv.URL + "/queue/item/3/"})
	require.ErrorContains(t, err, "queue http 404")
	_, err = p.GetBuild(ctx, cicd.BuildRef{Pipeline: "j", ExternalID: srv.URL + "/queue/item/4/"})
	require.Error(t, err)

	// "job#n" 形态 → 解析出构建号并规范化 ExternalID
	st, err = p.GetBuild(ctx, cicd.BuildRef{Pipeline: "j", ExternalID: "j#77"})
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusCancelled, st.Status)
	assert.Equal(t, "j#77", st.ExternalID)
	require.NotNil(t, st.StartedAt)
	require.NotNil(t, st.FinishedAt)

	// 纯数字形态（ref.Pipeline 缺省 → 回退 Extra["job"]）
	st, err = p.GetBuild(ctx, cicd.BuildRef{ExternalID: "77"})
	require.NoError(t, err)
	assert.Equal(t, "j#77", st.ExternalID)

	// 构建详情错误域
	_, err = p.GetBuild(ctx, cicd.BuildRef{Pipeline: "j", ExternalID: "78"})
	require.ErrorContains(t, err, "build http 404")
	_, err = p.GetBuild(ctx, cicd.BuildRef{Pipeline: "j", ExternalID: "79"})
	require.Error(t, err)

	// 非数字 ExternalID → lastBuild（此处 503 / 畸形负载两种）
	_, err = p.GetBuild(ctx, cicd.BuildRef{Pipeline: "j", ExternalID: "not-a-number"})
	require.ErrorContains(t, err, "lastBuild http 503")

	pb2, err := cicd.New("jenkins", cicd.Config{
		Endpoint: srv.URL, HTTP: srv.Client(), Extra: map[string]string{"job": "jb"},
	})
	require.NoError(t, err)
	_, err = pb2.GetBuild(ctx, cicd.BuildRef{Pipeline: "jb", ExternalID: "not-a-number"})
	require.Error(t, err)
	_, err = pb2.ListArtifacts(ctx, cicd.BuildRef{Pipeline: "jb", ExternalID: "not-a-number"})
	require.Error(t, err)

	// job 缺失
	noJob, err := cicd.New("jenkins", cicd.Config{Endpoint: srv.URL, HTTP: srv.Client()})
	require.NoError(t, err)
	_, err = noJob.GetBuild(ctx, cicd.BuildRef{ExternalID: "1"})
	require.ErrorContains(t, err, "缺少 job")
	_, err = noJob.ListArtifacts(ctx, cicd.BuildRef{ExternalID: "1"})
	require.ErrorContains(t, err, "缺少 job")

	// 传输失败（队列 / 构建 / lastBuild 三形态）
	pf, err := cicd.New("jenkins", cicd.Config{
		Endpoint: srv.URL, HTTP: epFailClient(), Extra: map[string]string{"job": "j"},
	})
	require.NoError(t, err)
	for _, ref := range []cicd.BuildRef{
		{Pipeline: "j", ExternalID: "77"},
		{Pipeline: "j", ExternalID: srv.URL + "/queue/item/1/"},
		{Pipeline: "j", ExternalID: "zzz"},
	} {
		_, err = pf.GetBuild(ctx, ref)
		require.ErrorContains(t, err, "connection refused")
		_, err = pf.ListArtifacts(ctx, ref)
		require.ErrorContains(t, err, "connection refused")
	}

	// 空格 URL
	pb, err := cicd.New("jenkins", cicd.Config{
		Endpoint: epBadURLEndpoint, Extra: map[string]string{"job": "j"},
	})
	require.NoError(t, err)
	_, err = pb.GetBuild(ctx, cicd.BuildRef{Pipeline: "j", ExternalID: "77"})
	require.Error(t, err)
}

// TestJenkinsProvider_ArtifactsBaseFallback 构建详情无 url 时按 p.base 兜底拼
// 产物下载基址；产物列表为空时回非 nil 空切片。
func TestJenkinsProvider_ArtifactsBaseFallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/job/nourl/5/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"number":5,"building":true,"result":null,` +
			`"artifacts":[{"relativePath":"out/app.tgz","fileName":"app.tgz"}]}`))
	})
	mux.HandleFunc("/job/noart/6/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"number":6,"building":true,"result":null,"artifacts":[]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p, err := cicd.New("jenkins", cicd.Config{
		Endpoint: srv.URL, HTTP: srv.Client(), Extra: map[string]string{"job": "nourl"},
	})
	require.NoError(t, err)
	ctx := context.Background()

	arts, err := p.ListArtifacts(ctx, cicd.BuildRef{Pipeline: "nourl", ExternalID: "5"})
	require.NoError(t, err)
	require.Len(t, arts, 1)
	assert.Equal(t, srv.URL+"/job/nourl/5/artifact/out/app.tgz", arts[0].URL)

	// building 且 result 为 nil → running；ExternalID 规范化为 job#n
	st, err := p.GetBuild(ctx, cicd.BuildRef{Pipeline: "nourl", ExternalID: "5"})
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusRunning, st.Status)
	assert.Equal(t, "nourl#5", st.ExternalID)

	empty, err := p.ListArtifacts(ctx, cicd.BuildRef{Pipeline: "noart", ExternalID: "6"})
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}
