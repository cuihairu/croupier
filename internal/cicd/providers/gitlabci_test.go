package providers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/cicd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- gitlab-ci ----

func TestGitlabCIProvider_TriggerStatusArtifacts(t *testing.T) {
	state := &struct{ lastToken string }{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/7/pipeline", func(w http.ResponseWriter, r *http.Request) {
		state.lastToken = r.Header.Get("PRIVATE-TOKEN")
		if r.URL.Query().Get("ref") != "release/v1.2" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 900, "web_url": "http://gl/p/7/-/pipelines/900"})
	})
	mux.HandleFunc("/api/v4/projects/7/pipelines/900", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 900, "status": "running", "web_url": "http://gl/p/7/-/pipelines/900",
			"started_at": "2026-09-29T00:00:00Z",
		})
	})
	mux.HandleFunc("/api/v4/projects/7/pipelines/900/jobs", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "build", "status": "success", "stage": "build"},
			{"id": 2, "name": "deploy", "status": "failed", "stage": "deploy"},
		})
	})
	// 路径形态 project（group/repo）转义
	mux.HandleFunc("/api/v4/projects/group%2Frepo/pipelines/901", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 901, "status": "canceled"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p, err := cicd.New("gitlab-ci", cicd.Config{
		Endpoint: srv.URL, Token: "glpat-xyz",
		Extra: map[string]string{"project": "7", "ref": "main"},
		HTTP:  srv.Client(),
	})
	require.NoError(t, err)

	ref, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "release/v1.2"})
	require.NoError(t, err)
	assert.Equal(t, "900", ref.ExternalID)
	assert.Equal(t, "glpat-xyz", state.lastToken)

	st, err := p.GetBuild(context.Background(), *ref)
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusRunning, st.Status)
	require.NotNil(t, st.StartedAt)

	arts, err := p.ListArtifacts(context.Background(), *ref)
	require.NoError(t, err)
	require.Len(t, arts, 1, "仅 success job 出产物")
	assert.Equal(t, "build", arts[0].Name)
	assert.True(t, strings.HasSuffix(arts[0].URL, "/jobs/1/artifacts"))

	// group/repo 路径形态
	p2, err := cicd.New("gitlab-ci", cicd.Config{
		Endpoint: srv.URL, Extra: map[string]string{"project": "group/repo"}, HTTP: srv.Client(),
	})
	require.NoError(t, err)
	st2, err := p2.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "901"})
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusCancelled, st2.Status)
}

func TestGitlabCIProvider_ConfigValidation(t *testing.T) {
	_, err := cicd.New("gitlab-ci", cicd.Config{Endpoint: srvPlaceholder})
	require.ErrorContains(t, err, "project")
	_, err = cicd.New("gitlab-ci", cicd.Config{Endpoint: "nope", Extra: map[string]string{"project": "1"}})
	require.Error(t, err)
}

var srvPlaceholder = "http://x"

// ---- github-actions ----

func TestGitHubActionsProvider_DispatchAndRuns(t *testing.T) {
	state := &struct {
		auth        string
		dispatchRef string
	}{}
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/actions/workflows/build.yml/dispatches", func(w http.ResponseWriter, r *http.Request) {
		state.auth = r.Header.Get("Authorization")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		state.dispatchRef, _ = body["ref"].(string)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/repos/o/r/actions/runs/55", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 55, "status": "completed", "conclusion": "success",
			"html_url":       "http://gh/o/r/actions/runs/55",
			"run_started_at": "2026-09-29T00:00:00Z", "updated_at": "2026-09-29T00:10:00Z",
		})
	})
	mux.HandleFunc("/repos/o/r/actions/workflows/build.yml/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ref") != "main" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workflow_runs": []map[string]any{{
				"id": 66, "status": "in_progress", "html_url": "http://gh/o/r/actions/runs/66",
			}},
		})
	})
	mux.HandleFunc("/repos/o/r/actions/runs/66/artifacts", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": 2,
			"artifacts": []map[string]any{
				{"id": 1, "name": "bin", "size_in_bytes": 123, "archive_download_url": "http://gh/dl/1", "expired": false},
				{"id": 2, "name": "old", "size_in_bytes": 1, "archive_download_url": "http://gh/dl/2", "expired": true},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p, err := cicd.New("github-actions", cicd.Config{
		Endpoint: srv.URL, Token: "ghp-token",
		Extra: map[string]string{"repo": "o/r", "workflow": "build.yml", "ref": "main"},
		HTTP:  srv.Client(),
	})
	require.NoError(t, err)

	ref, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "build.yml"})
	require.NoError(t, err)
	assert.Equal(t, "Bearer ghp-token", state.auth)
	assert.Equal(t, "main", state.dispatchRef)
	assert.Empty(t, ref.ExternalID, "dispatch 不返回 run id")

	// 空 ExternalID → 最近 run
	st, err := p.GetBuild(context.Background(), *ref)
	require.NoError(t, err)
	assert.Equal(t, "66", st.ExternalID)
	assert.Equal(t, cicd.StatusRunning, st.Status)

	// 带 run id → completed+success
	st2, err := p.GetBuild(context.Background(), cicd.BuildRef{Pipeline: "build.yml", ExternalID: "55"})
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusSuccess, st2.Status)

	arts, err := p.ListArtifacts(context.Background(), cicd.BuildRef{ExternalID: "66"})
	require.NoError(t, err)
	require.Len(t, arts, 1, "expired 产物应过滤")
	assert.Equal(t, "bin", arts[0].Name)
}

func TestGitHubActionsProvider_ConfigValidation(t *testing.T) {
	_, err := cicd.New("github-actions", cicd.Config{Extra: map[string]string{"workflow": "w.yml"}})
	require.ErrorContains(t, err, "repo")
	_, err = cicd.New("github-actions", cicd.Config{Extra: map[string]string{"repo": "no-slash"}})
	require.ErrorContains(t, err, "repo")
}

// ---- generic ----

func TestGenericProvider_TriggerStatus(t *testing.T) {
	state := &struct {
		triggerHeader string
		triggerBody   string
	}{}
	mux := http.NewServeMux()
	mux.HandleFunc("/ci/trigger", func(w http.ResponseWriter, r *http.Request) {
		state.triggerHeader = r.Header.Get("X-Internal-Token")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		state.triggerBody = string(buf[:n])
		_, _ = w.Write([]byte(`{"id":"build-77","webUrl":"http://ci/builds/77"}`))
	})
	mux.HandleFunc("/ci/status/build-77", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"passed","webUrl":"http://ci/builds/77","artifactUrl":"http://ci/artifacts/77.zip","checksum":"sha256:abc"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{
			"triggerUrl": srv.URL + "/ci/trigger",
			"statusUrl":  srv.URL + "/ci/status/{id}",
			"headerName": "X-Internal-Token", "headerValue": "tok-1",
		},
		HTTP: srv.Client(),
	})
	require.NoError(t, err)

	ref, err := p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "pack", Params: map[string]string{"branch": "main"}})
	require.NoError(t, err)
	assert.Equal(t, "build-77", ref.ExternalID)
	assert.Equal(t, "tok-1", state.triggerHeader)
	assert.Contains(t, state.triggerBody, `"pipeline":"pack"`)

	st, err := p.GetBuild(context.Background(), *ref)
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusSuccess, st.Status, "passed 应归一为 success")
	assert.Equal(t, "http://ci/artifacts/77.zip", st.ArtifactURL)
	assert.Equal(t, "sha256:abc", st.ArtifactChecksum)

	// 未知状态 → unknown
	mux.HandleFunc("/ci/status/build-88", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"weird"}`))
	})
	st2, err := p.GetBuild(context.Background(), cicd.BuildRef{ExternalID: "build-88"})
	require.NoError(t, err)
	assert.Equal(t, cicd.StatusUnknown, st2.Status)
}

func TestGenericProvider_ConfigValidation(t *testing.T) {
	_, err := cicd.New("generic", cicd.Config{})
	require.ErrorContains(t, err, "至少配置一个")
	_, err = cicd.New("generic", cicd.Config{Extra: map[string]string{
		"triggerUrl": srvPlaceholder, "headerName": "A",
	}})
	require.ErrorContains(t, err, "成对")
	_, err = cicd.New("generic", cicd.Config{Extra: map[string]string{"triggerUrl": "ftp://x"}})
	require.Error(t, err)
}

func TestGenericProvider_TriggerWithoutURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	t.Cleanup(srv.Close)
	p, err := cicd.New("generic", cicd.Config{
		Extra: map[string]string{"statusUrl": srv.URL + "/s/{id}"}, HTTP: srv.Client(),
	})
	require.NoError(t, err)
	_, err = p.TriggerBuild(context.Background(), cicd.TriggerRequest{Pipeline: "x"})
	require.ErrorContains(t, err, "triggerUrl")
}
