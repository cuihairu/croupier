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

// gitlabCIProvider 对接 GitLab CI（REST v4，PRIVATE-TOKEN 头）。
//
// Extra 键：
//   - project：项目数字 ID 或 URL 路径（如 group/repo，自动转义）
//   - ref：默认分支/tag（Trigger 未指定 Pipeline 时回退）
//
// ExternalID = pipeline 数字 ID。状态映射：created/waiting_for_resource/
// pending/preparing→queued；running→running；success→success；failed→failed；
// canceled/skipped→cancelled；其余→unknown。
type gitlabCIProvider struct {
	base    string
	token   string
	project string
	ref     string
	client  *http.Client
	now     func() time.Time
}

func init() {
	cicd.Register("gitlab-ci", func(cfg cicd.Config) (cicd.Provider, error) {
		ep := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
		if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
			return nil, errors.New("gitlab-ci: endpoint 必须是 http(s) URL")
		}
		project := strings.TrimSpace(cfg.Extra["project"])
		if project == "" {
			return nil, errors.New("gitlab-ci: 缺少 project（数字 ID 或 group/repo）")
		}
		p := &gitlabCIProvider{
			base:    ep,
			token:   cfg.Token,
			project: project,
			ref:     cfg.Extra["ref"],
			client:  cfg.HTTP,
			now:     cfg.Now,
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

func (p *gitlabCIProvider) Kind() string { return "gitlab-ci" }

// projectEsc 返回 URL 路径安全的 project 标识（纯数字直接用，路径做转义）。
func (p *gitlabCIProvider) projectEsc() string {
	if isAllDigits(p.project) {
		return p.project
	}
	return url.PathEscape(p.project)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (p *gitlabCIProvider) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, p.base+path, body)
	if err != nil {
		return nil, err
	}
	if p.token != "" {
		req.Header.Set("PRIVATE-TOKEN", p.token)
	}
	return p.client.Do(req)
}

func (p *gitlabCIProvider) TriggerBuild(ctx context.Context, req cicd.TriggerRequest) (*cicd.BuildRef, error) {
	refName := req.Pipeline
	if refName == "" {
		refName = p.ref
	}
	if strings.TrimSpace(refName) == "" {
		return nil, errors.New("gitlab-ci: 未指定 ref（分支或 tag）")
	}
	q := url.Values{"ref": {refName}}
	for k, v := range req.Params {
		q.Set("variables["+k+"]", v)
	}
	resp, err := p.do(ctx, http.MethodPost,
		"/api/v4/projects/"+p.projectEsc()+"/pipeline?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("gitlab-ci: trigger http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		ID     int    `json:"id"`
		WebURL string `json:"web_url"`
		Refs   string `json:"ref"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &cicd.BuildRef{
		Pipeline:   refName,
		ExternalID: fmt.Sprint(out.ID),
		WebURL:     out.WebURL,
	}, nil
}

type gitlabPipeline struct {
	ID         int     `json:"id"`
	Status     string  `json:"status"`
	WebURL     string  `json:"web_url"`
	Ref        string  `json:"ref"`
	CreatedAt  *string `json:"created_at"`
	UpdatedAt  *string `json:"updated_at"`
	StartedAt  *string `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
}

func mapGitlabStatus(s string) string {
	switch s {
	case "created", "waiting_for_resource", "pending", "preparing":
		return cicd.StatusQueued
	case "running":
		return cicd.StatusRunning
	case "success":
		return cicd.StatusSuccess
	case "failed":
		return cicd.StatusFailed
	case "canceled", "skipped":
		return cicd.StatusCancelled
	default:
		return cicd.StatusUnknown
	}
}

func parseRFC3339(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, *s); err == nil {
		return &t
	}
	return nil
}

func (p *gitlabCIProvider) GetBuild(ctx context.Context, ref cicd.BuildRef) (*cicd.BuildStatus, error) {
	pid := strings.TrimSpace(ref.ExternalID)
	if pid == "" {
		return nil, errors.New("gitlab-ci: 缺少 pipeline id")
	}
	resp, err := p.do(ctx, http.MethodGet,
		"/api/v4/projects/"+p.projectEsc()+"/pipelines/"+url.PathEscape(pid), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab-ci: build http %d", resp.StatusCode)
	}
	var pl gitlabPipeline
	if err := json.NewDecoder(resp.Body).Decode(&pl); err != nil {
		return nil, err
	}
	return &cicd.BuildStatus{
		Pipeline:   ref.Pipeline,
		ExternalID: fmt.Sprint(pl.ID),
		Status:     mapGitlabStatus(pl.Status),
		WebURL:     pl.WebURL,
		StartedAt:  parseRFC3339(pl.StartedAt),
		FinishedAt: parseRFC3339(pl.FinishedAt),
	}, nil
}

type gitlabJob struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Stage  string `json:"stage"`
}

// ListArtifacts 经 pipeline jobs 枚举产物下载链接（v1：每个 job 的
// artifacts 归档按 job 粒度给出）。
func (p *gitlabCIProvider) ListArtifacts(ctx context.Context, ref cicd.BuildRef) ([]cicd.Artifact, error) {
	pid := strings.TrimSpace(ref.ExternalID)
	if pid == "" {
		return nil, errors.New("gitlab-ci: 缺少 pipeline id")
	}
	resp, err := p.do(ctx, http.MethodGet,
		"/api/v4/projects/"+p.projectEsc()+"/pipelines/"+url.PathEscape(pid)+"/jobs", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab-ci: jobs http %d", resp.StatusCode)
	}
	var jobs []gitlabJob
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, err
	}
	out := make([]cicd.Artifact, 0, len(jobs))
	for _, j := range jobs {
		if j.Status != "success" {
			continue
		}
		out = append(out, cicd.Artifact{
			Name: j.Name,
			Path: j.Stage + "/" + j.Name,
			URL: fmt.Sprintf("%s/api/v4/projects/%s/jobs/%d/artifacts",
				p.base, p.projectEsc(), j.ID),
		})
	}
	return out, nil
}
