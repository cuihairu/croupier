package providers_test

// provider 行为测试（OPEN-ISSUES #58）：httptest 假服务器逐 provider 验证
// 触发/状态映射/产物枚举与配置校验。ExternalID 解析、状态映射矩阵为回归
// 重点——provider 是外部系统的唯一适配面。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/cicd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- jenkins ----

// fakeJenkins 实现最小 Jenkins REST 面：crumbIssuer、job 触发（记录
// Authorization/crumb 头）、queue item → build、build 详情。
func fakeJenkins(t *testing.T) (*httptest.Server, *struct {
	lastAuth   string
	lastCrumb  string
	lastMethod string
	triggered  int
}) {
	t.Helper()
	state := &struct {
		lastAuth   string
		lastCrumb  string
		lastMethod string
		triggered  int
	}{}
	mux := http.NewServeMux()
	mux.HandleFunc("/crumbIssuer/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"crumb": "crumb-xyz", "crumbRequestField": "Jenkins-Crumb",
		})
	})
	mux.HandleFunc("/job/build-app/build", func(w http.ResponseWriter, r *http.Request) {
		state.lastMethod = r.Method
		state.lastAuth = r.Header.Get("Authorization")
		state.lastCrumb = r.Header.Get("Jenkins-Crumb")
		state.triggered++
		w.Header().Set("Location", "http://svc/queue/item/7/")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/job/build-app/queue/item/7/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"executable": map[string]any{"number": 42},
		})
	})
	mux.HandleFunc("/job/build-app/42/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"number": 42, "building": false, "result": "SUCCESS",
			"url": "http://svc/job/build-app/42/", "timestamp": 1727500000000, "duration": 60000,
			"artifacts": []map[string]string{
				{"relativePath": "dist/app.zip", "fileName": "app.zip"},
			},
		})
	})
	mux.HandleFunc("/job/build-app/lastBuild/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"number": 41, "building": true, "result": nil,
			"url": "http://svc/job/build-app/41/",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	// 触发时 Location 里的 host 是 http://svc/（占位）——重写为真实 host
	mux.HandleFunc("/queue/item/7/api/json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"executable": map[string]any{"number": 42},
		})
	})
	_ = state
	return srv, state
}

func TestJenkinsProvider_TriggerAndStatus(t *testing.T) {
	srv, state := fakeJenkins(t)
	p, err := cicd.New("jenkins", cicd.Config{
		Endpoint: srv.URL,
		Token:    "ci-user:apitoken-123",
		HTTP:     srv.Client(),
	})
	require.NoError(t, err)

	ref, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "build-app"})
	require.NoError(t, err)
	assert.Equal(t, "build-app", ref.Pipeline)
	assert.Contains(t, ref.ExternalID, "/queue/item/7")
	assert.Equal(t, "POST", state.lastMethod)
	assert.True(t, strings.HasPrefix(state.lastAuth, "Basic "), "应带 basic auth")
	assert.Equal(t, "crumb-xyz", state.lastCrumb, "触发应带 CSRF crumb")

	// 队列 URL → 解析构建号 → 构建详情（SUCCESS）
	st, err := p.GetBuild(context.Background(), *ref)
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusSuccess, st.Status)
	assert.Equal(t, "http://svc/job/build-app/42/", st.WebURL)
	require.NotNil(t, st.StartedAt)
	require.NotNil(t, st.FinishedAt)
	assert.Contains(t, st.ExternalID, "42")

	// 空 ExternalID → lastBuild（building=true → running）
	st2, err := p.GetBuild(context.Background(), cicd.BuildRef{Pipeline: "build-app"})
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusRunning, st2.Status)
}

func TestJenkinsProvider_Artifacts(t *testing.T) {
	srv, _ := fakeJenkins(t)
	p, err := cicd.New("jenkins", cicd.Config{Endpoint: srv.URL, HTTP: srv.Client()})
	require.NoError(t, err)
	arts, err := p.ListArtifacts(context.Background(),
		cicd.BuildRef{Pipeline: "build-app", ExternalID: "42"})
	require.NoError(t, err)
	require.Len(t, arts, 1)
	assert.Equal(t, "app.zip", arts[0].Name)
	assert.Equal(t, "dist/app.zip", arts[0].Path)
	assert.True(t, strings.HasSuffix(arts[0].URL, "/artifact/dist/app.zip"))
}

func TestJenkinsProvider_ConfigValidation(t *testing.T) {
	_, err := cicd.New("jenkins", cicd.Config{Endpoint: "ftp://nope"})
	require.ErrorContains(t, err, "endpoint")
	_, err = cicd.New("jenkins", cicd.Config{Endpoint: ""})
	require.Error(t, err)
}

func TestJenkinsProvider_StatusMappingMatrix(t *testing.T) {
	// 逐 result 值验证映射（不依赖真实服务器：直接构造 build json）
	cases := []struct {
		result   *string
		building bool
		want     string
	}{
		{strPtr("SUCCESS"), false, cicd.StatusSuccess},
		{strPtr("FAILURE"), false, cicd.StatusFailed},
		{strPtr("UNSTABLE"), false, cicd.StatusFailed},
		{strPtr("ABORTED"), false, cicd.StatusCancelled},
		{strPtr("NOT_BUILT"), false, cicd.StatusUnknown},
		{nil, true, cicd.StatusRunning},
		{nil, false, cicd.StatusQueued},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var buildingKey string
			if tc.building {
				buildingKey = "true"
			} else {
				buildingKey = "false"
			}
			res := "null"
			if tc.result != nil {
				res = `"` + *tc.result + `"`
			}
			body := fmt.Sprintf(`{"number":1,"building":%s,"result":%s,"url":"http://x/1/"}`,
				buildingKey, res)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			p, err := cicd.New("jenkins", cicd.Config{Endpoint: srv.URL, HTTP: srv.Client()})
			require.NoError(t, err)
			st, err := p.GetBuild(context.Background(), cicd.BuildRef{Pipeline: "j", ExternalID: "1"})
			require.NoError(t, err)
			assert.Equal(t, tc.want, st.Status)
		})
	}
}

func strPtr(s string) *string { return &s }
