package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/cicd"
)

// genericProvider 对接任意自定义 REST 构建系统的通用 provider：用户配置
// 触发/查询端点模板与可选认证头，约定轻量 JSON 契约。
//
// Extra 键：
//   - triggerUrl：触发端点（可选；缺省时只能 webhook 回写，不能站内触发）
//   - statusUrl：状态查询模板，{id} 占位符替换为 ExternalID（可选）
//   - headerName / headerValue：两者都配置时为触发与查询请求附带认证头
//   - pipeline：默认流水线标识（Trigger 未指定 Pipeline 时回退）
//
// 触发请求：POST triggerUrl，JSON body {"pipeline":…,"params":{…}}；响应
// 取 JSON {"id","webUrl"}（id 缺省时回退 Location 头）。
// 状态响应：JSON {"status","webUrl","artifactUrl","checksum"}（status 经
// model.NormalizeCicdBuildStatus 归一：success/ok/passed/done→success 等，
// 未识别→unknown）。
type genericProvider struct {
	pipeline    string
	triggerURL  string
	statusURL   string
	headerName  string
	headerValue string
	client      *http.Client
	now         func() time.Time
}

func init() {
	cicd.Register("generic", func(cfg cicd.Config) (cicd.Provider, error) {
		p := &genericProvider{
			pipeline:    cfg.Extra["pipeline"],
			triggerURL:  strings.TrimSpace(cfg.Extra["triggerUrl"]),
			statusURL:   strings.TrimSpace(cfg.Extra["statusUrl"]),
			headerName:  strings.TrimSpace(cfg.Extra["headerName"]),
			headerValue: cfg.Extra["headerValue"],
			client:      cfg.HTTP,
			now:         cfg.Now,
		}
		for k, v := range map[string]*string{"triggerUrl": &p.triggerURL, "statusUrl": &p.statusURL} {
			if *v != "" && !strings.HasPrefix(*v, "http://") && !strings.HasPrefix(*v, "https://") {
				return nil, errors.New("generic: " + k + " 必须是 http(s) URL")
			}
		}
		if p.triggerURL == "" && p.statusURL == "" {
			return nil, errors.New("generic: triggerUrl 与 statusUrl 至少配置一个")
		}
		if (p.headerName == "") != (p.headerValue == "") {
			return nil, errors.New("generic: headerName 与 headerValue 需成对配置")
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

func (p *genericProvider) Kind() string { return "generic" }

func (p *genericProvider) do(ctx context.Context, method, rawURL string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.headerName != "" {
		req.Header.Set(p.headerName, p.headerValue)
	}
	return p.client.Do(req)
}

type genericTriggerResp struct {
	ID     string `json:"id"`
	WebURL string `json:"webUrl"`
}

func (p *genericProvider) TriggerBuild(ctx context.Context, req cicd.TriggerRequest) (*cicd.BuildRef, error) {
	if p.triggerURL == "" {
		return nil, errors.New("generic: 未配置 triggerUrl，不支持站内触发（可用 webhook 回写）")
	}
	pipeline := req.Pipeline
	if pipeline == "" {
		pipeline = p.pipeline
	}
	if strings.TrimSpace(pipeline) == "" {
		return nil, errors.New("generic: 未指定 pipeline")
	}
	body, _ := json.Marshal(map[string]interface{}{
		"pipeline": pipeline,
		"params":   req.Params,
	})
	resp, err := p.do(ctx, http.MethodPost, p.triggerURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("generic: trigger http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out genericTriggerResp
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out)
	ext := out.ID
	if ext == "" {
		ext = resp.Header.Get("Location")
	}
	return &cicd.BuildRef{Pipeline: pipeline, ExternalID: ext, WebURL: out.WebURL}, nil
}

type genericStatusResp struct {
	Status      string  `json:"status"`
	WebURL      string  `json:"webUrl"`
	ArtifactURL string  `json:"artifactUrl"`
	Checksum    string  `json:"checksum"`
	StartedAt   *string `json:"startedAt"`
	FinishedAt  *string `json:"finishedAt"`
}

func (p *genericProvider) GetBuild(ctx context.Context, ref cicd.BuildRef) (*cicd.BuildStatus, error) {
	if p.statusURL == "" {
		return nil, errors.New("generic: 未配置 statusUrl，不支持状态拉取（可用 webhook 回写）")
	}
	if strings.TrimSpace(ref.ExternalID) == "" {
		return nil, errors.New("generic: 缺少构建 id")
	}
	resp, err := p.do(ctx, http.MethodGet,
		strings.ReplaceAll(p.statusURL, "{id}", ref.ExternalID), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("generic: status http %d", resp.StatusCode)
	}
	var out genericStatusResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return nil, err
	}
	return &cicd.BuildStatus{
		Pipeline:         ref.Pipeline,
		ExternalID:       ref.ExternalID,
		Status:           normalizeGenericStatus(out.Status),
		WebURL:           out.WebURL,
		ArtifactURL:      out.ArtifactURL,
		ArtifactChecksum: out.Checksum,
		StartedAt:        parseRFC3339(out.StartedAt),
		FinishedAt:       parseRFC3339(out.FinishedAt),
	}, nil
}

// normalizeGenericStatus 宽收常见状态词，未识别回落 unknown（与 model
// 归一同语义，独立实现避免 cicd→model import）。
func normalizeGenericStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "queued", "pending", "created", "waiting", "preparing":
		return cicd.StatusQueued
	case "running", "in_progress", "building":
		return cicd.StatusRunning
	case "success", "succeeded", "ok", "passed", "done", "complete", "completed":
		return cicd.StatusSuccess
	case "failed", "failure", "error", "errored", "unstable":
		return cicd.StatusFailed
	case "cancelled", "canceled", "aborted", "skipped":
		return cicd.StatusCancelled
	default:
		return cicd.StatusUnknown
	}
}

func (p *genericProvider) ListArtifacts(ctx context.Context, ref cicd.BuildRef) ([]cicd.Artifact, error) {
	st, err := p.GetBuild(ctx, ref)
	if err != nil {
		return nil, err
	}
	if st.ArtifactURL == "" {
		return []cicd.Artifact{}, nil
	}
	return []cicd.Artifact{{
		Name: "artifact",
		Path: "artifact",
		URL:  st.ArtifactURL,
	}}, nil
}
