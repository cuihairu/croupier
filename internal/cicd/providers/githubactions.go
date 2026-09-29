package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/cicd"
)

// githubActionsProvider 对接 GitHub Actions（REST，Bearer PAT）。
//
// Extra 键：
//   - repo：owner/name（必填）
//   - workflow：workflow 文件名（如 build.yml）或数字 ID（Trigger 未指定
//     Pipeline 时回退）
//   - ref：默认分支/tag（缺省 main）
//
// 触发走 workflow dispatch（返回 204 无 run id）→ GetBuild 以空 ExternalID
// 查询时取该 workflow+ref 最近一次 run 并回填 ExternalID（v1 边界：触发后
// 立即查询存在竞态，webhook 回写为主通道）。状态映射：queued/requested/
// waiting/pending→queued；in_progress→running；completed 按 conclusion
// success→success / failure/timed_out/startup_failure→failed /
// cancelled→cancelled；其余→unknown。
type githubActionsProvider struct {
	base     string
	token    string
	repo     string
	workflow string
	ref      string
	client   *http.Client
	now      func() time.Time
}

func init() {
	cicd.Register("github-actions", func(cfg cicd.Config) (cicd.Provider, error) {
		ep := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
		if ep == "" {
			ep = "https://api.github.com"
		}
		if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
			return nil, errors.New("github-actions: endpoint 必须是 http(s) URL")
		}
		repo := strings.TrimSpace(cfg.Extra["repo"])
		if repo == "" || !strings.Contains(repo, "/") {
			return nil, errors.New("github-actions: 缺少 repo（owner/name）")
		}
		p := &githubActionsProvider{
			base:     ep,
			token:    cfg.Token,
			repo:     repo,
			workflow: cfg.Extra["workflow"],
			ref:      cfg.Extra["ref"],
			client:   cfg.HTTP,
			now:      cfg.Now,
		}
		if p.ref == "" {
			p.ref = "main"
		}
		if p.client == nil {
			p.client = http.DefaultClient
		}
		if p.now == nil {
			p.now = time.Now
		}
		return p, nil
	})
}

func (p *githubActionsProvider) Kind() string { return "github-actions" }

func (p *githubActionsProvider) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, p.base+path, body)
	if err != nil {
		return nil, err
	}
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	return p.client.Do(req)
}

func (p *githubActionsProvider) TriggerBuild(ctx context.Context, req cicd.TriggerRequest) (*cicd.BuildRef, error) {
	workflow := req.Pipeline
	if workflow == "" {
		workflow = p.workflow
	}
	if strings.TrimSpace(workflow) == "" {
		return nil, errors.New("github-actions: 未指定 workflow")
	}
	refName := p.ref
	for _, k := range []string{"ref", "branch"} {
		if v, ok := req.Params[k]; ok && v != "" {
			refName = v
		}
	}
	inputs := map[string]interface{}{}
	for k, v := range req.Params {
		if k == "ref" || k == "branch" {
			continue
		}
		inputs[k] = v
	}
	payload, _ := json.Marshal(map[string]interface{}{"ref": refName, "inputs": inputs})
	resp, err := p.do(ctx, http.MethodPost,
		"/repos/"+p.repo+"/actions/workflows/"+workflow+"/dispatches", strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github-actions: dispatch http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return &cicd.BuildRef{
		Pipeline: workflow,
		WebURL:   "https://github.com/" + p.repo + "/actions/workflows/" + workflow,
	}, nil
}

type ghRun struct {
	ID           int     `json:"id"`
	RunNumber    int     `json:"run_number"`
	Status       string  `json:"status"`
	Conclusion   *string `json:"conclusion"`
	HTMLURL      string  `json:"html_url"`
	HeadBranch   string  `json:"head_branch"`
	CreatedAt    *string `json:"created_at"`
	RunStartedAt *string `json:"run_started_at"`
	UpdatedAt    *string `json:"updated_at"`
}

func mapGHRun(r *ghRun) string {
	switch r.Status {
	case "queued", "requested", "waiting", "pending":
		return cicd.StatusQueued
	case "in_progress":
		return cicd.StatusRunning
	case "completed":
		if r.Conclusion == nil {
			return cicd.StatusUnknown
		}
		switch *r.Conclusion {
		case "success":
			return cicd.StatusSuccess
		case "failure", "timed_out", "startup_failure":
			return cicd.StatusFailed
		case "cancelled":
			return cicd.StatusCancelled
		default:
			return cicd.StatusUnknown
		}
	default:
		return cicd.StatusUnknown
	}
}

func (p *githubActionsProvider) GetBuild(ctx context.Context, ref cicd.BuildRef) (*cicd.BuildStatus, error) {
	workflow := ref.Pipeline
	if workflow == "" {
		workflow = p.workflow
	}
	if strings.TrimSpace(ref.ExternalID) != "" {
		resp, err := p.do(ctx, http.MethodGet,
			"/repos/"+p.repo+"/actions/runs/"+ref.ExternalID, nil)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("github-actions: run http %d", resp.StatusCode)
		}
		var r ghRun
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return nil, err
		}
		return p.statusFromRun(workflow, &r), nil
	}
	// 空 ExternalID：取 workflow+ref 最近一次 run
	if strings.TrimSpace(workflow) == "" {
		return nil, errors.New("github-actions: 缺少 workflow")
	}
	resp, err := p.do(ctx, http.MethodGet,
		"/repos/"+p.repo+"/actions/workflows/"+workflow+
			"/runs?per_page=1&ref="+url.PathEscape(p.ref), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github-actions: runs http %d", resp.StatusCode)
	}
	var out struct {
		WorkflowRuns []ghRun `json:"workflow_runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.WorkflowRuns) == 0 {
		return nil, errors.New("github-actions: 该 workflow 暂无运行记录")
	}
	return p.statusFromRun(workflow, &out.WorkflowRuns[0]), nil
}

func (p *githubActionsProvider) statusFromRun(workflow string, r *ghRun) *cicd.BuildStatus {
	st := &cicd.BuildStatus{
		Pipeline:   workflow,
		ExternalID: fmt.Sprint(r.ID),
		Status:     mapGHRun(r),
		WebURL:     r.HTMLURL,
		StartedAt:  parseRFC3339(r.RunStartedAt),
		FinishedAt: parseRFC3339(r.UpdatedAt),
	}
	return st
}

type ghArtifacts struct {
	TotalCount int `json:"total_count"`
	Artifacts  []struct {
		ID                 int    `json:"id"`
		Name               string `json:"name"`
		SizeInBytes        int64  `json:"size_in_bytes"`
		ArchiveDownloadURL string `json:"archive_download_url"`
		Expired            bool   `json:"expired"`
	} `json:"artifacts"`
}

func (p *githubActionsProvider) ListArtifacts(ctx context.Context, ref cicd.BuildRef) ([]cicd.Artifact, error) {
	if strings.TrimSpace(ref.ExternalID) == "" {
		return nil, errors.New("github-actions: 缺少 run id")
	}
	resp, err := p.do(ctx, http.MethodGet,
		"/repos/"+p.repo+"/actions/runs/"+ref.ExternalID+"/artifacts?per_page=100", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github-actions: artifacts http %d", resp.StatusCode)
	}
	var out ghArtifacts
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	list := make([]cicd.Artifact, 0, len(out.Artifacts))
	for _, a := range out.Artifacts {
		if a.Expired {
			continue
		}
		list = append(list, cicd.Artifact{
			Name: a.Name,
			Path: a.Name,
			URL:  a.ArchiveDownloadURL,
			Size: a.SizeInBytes,
		})
	}
	return list, nil
}
