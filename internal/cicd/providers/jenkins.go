// Package providers 注册各外部 CI/CD 系统的 provider 实现（OPEN-ISSUES
// #58）。每个实现经 init 自注册到 internal/cicd 注册表；消费方 import 本
// 包即完成注册。新增 provider（drone、teamcity……）= 新文件 + Register。
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

// jenkinsProvider 对接 Jenkins（REST，basic auth + CSRF crumb）。
//
// Extra 键：
//   - job：默认 job 名（Trigger 未指定 Pipeline 时回退）
//   - crumbDisabled："true" 时跳过 crumb 获取（Jenkins 关闭 CSRF 时）
//
// ExternalID 形态：触发返回 Jenkins 队列项 URL（…/queue/item/N/），
// GetBuild 先查队列项解析出构建号（排队中 → queued），再查构建详情。
// 状态映射：building→running；result 空→queued；SUCCESS→success；
// FAILURE/UNSTABLE→failed；ABORTED→cancelled；其余→unknown。
type jenkinsProvider struct {
	base    string
	user    string
	token   string
	job     string
	noCrumb bool
	client  *http.Client
	now     func() time.Time
}

func init() {
	cicd.Register("jenkins", func(cfg cicd.Config) (cicd.Provider, error) {
		ep := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
		if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
			return nil, errors.New("jenkins: endpoint 必须是 http(s) URL")
		}
		p := &jenkinsProvider{
			base:    ep,
			job:     cfg.Extra["job"],
			noCrumb: strings.EqualFold(cfg.Extra["crumbDisabled"], "true"),
			client:  cfg.HTTP,
			now:     cfg.Now,
		}
		// token 形态支持 "token" 或 "user:token"（apitoken 需要用户名）
		if t := cfg.Token; t != "" {
			if i := strings.Index(t, ":"); i > 0 {
				p.user, p.token = t[:i], t[i+1:]
			} else {
				p.token = t
			}
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

func (p *jenkinsProvider) Kind() string { return "jenkins" }

type jenkinsCrumb struct {
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

// do 执行一次认证请求；crumb 为 true 时自动附带 CSRF 头。
func (p *jenkinsProvider) do(ctx context.Context, method, path string, body io.Reader, crumb bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, p.base+path, body)
	if err != nil {
		return nil, err
	}
	if p.token != "" {
		req.SetBasicAuth(p.user, p.token)
	}
	if crumb {
		c, err := p.fetchCrumb(ctx)
		if err == nil && c.Crumb != "" {
			req.Header.Set(c.CrumbRequestField, c.Crumb)
		}
		// crumb 获取失败不阻断：关闭 CSRF 的实例无 crumbIssuer
	}
	return p.client.Do(req)
}

func (p *jenkinsProvider) fetchCrumb(ctx context.Context) (*jenkinsCrumb, error) {
	if p.noCrumb {
		return &jenkinsCrumb{}, nil
	}
	resp, err := p.do(ctx, http.MethodGet, "/crumbIssuer/api/json", nil, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crumb http %d", resp.StatusCode)
	}
	var c jenkinsCrumb
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *jenkinsProvider) TriggerBuild(ctx context.Context, req cicd.TriggerRequest) (*cicd.BuildRef, error) {
	job := req.Pipeline
	if job == "" {
		job = p.job
	}
	if strings.TrimSpace(job) == "" {
		return nil, errors.New("jenkins: 未指定 job")
	}
	hasParams := len(req.Params) > 0
	path := "/job/" + url.PathEscape(job) + "/build"
	if hasParams {
		path = "/job/" + url.PathEscape(job) + "/buildWithParameters"
		q := url.Values{}
		for k, v := range req.Params {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	resp, err := p.do(ctx, http.MethodPost, path, nil, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("jenkins: trigger http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	// 201 + Location: …/queue/item/N/
	loc := resp.Header.Get("Location")
	if loc != "" {
		if u, err := url.Parse(loc); err == nil && u.Path != "" {
			loc = p.base + u.Path
		}
	}
	return &cicd.BuildRef{Pipeline: job, ExternalID: loc}, nil
}

type jenkinsQueueItem struct {
	Executable *struct {
		Number int `json:"number"`
	} `json:"executable"`
}

type jenkinsBuild struct {
	Number    int     `json:"number"`
	Building  bool    `json:"building"`
	Result    *string `json:"result"`
	URL       string  `json:"url"`
	Artifacts []struct {
		RelativePath string `json:"relativePath"`
		FileName     string `json:"fileName"`
	} `json:"artifacts"`
	Timestamp int64 `json:"timestamp"` // ms epoch
	Duration  int64 `json:"duration"`  // ms
}

func mapJenkinsResult(b *jenkinsBuild) string {
	switch {
	case b.Building:
		return cicd.StatusRunning
	case b.Result == nil:
		return cicd.StatusQueued
	}
	switch *b.Result {
	case "SUCCESS":
		return cicd.StatusSuccess
	case "FAILURE", "UNSTABLE":
		return cicd.StatusFailed
	case "ABORTED":
		return cicd.StatusCancelled
	default:
		return cicd.StatusUnknown
	}
}

// resolveExternal 把 ExternalID（队列 URL 或 "job#n"）解析为 (job, buildNumber)。
func (p *jenkinsProvider) resolveBuild(ctx context.Context, ref cicd.BuildRef) (*jenkinsBuild, error) {
	job := ref.Pipeline
	if job == "" {
		job = p.job
	}
	if strings.TrimSpace(job) == "" {
		return nil, errors.New("jenkins: 缺少 job")
	}
	// 情形一：ExternalID 为队列项 URL → 轮询解析构建号
	if ref.ExternalID != "" && strings.HasPrefix(ref.ExternalID, "http") {
		resp, err := p.do(ctx, http.MethodGet, strings.TrimPrefix(ref.ExternalID, p.base)+"api/json", nil, false)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("jenkins: queue http %d", resp.StatusCode)
		}
		var qi jenkinsQueueItem
		if err := json.NewDecoder(resp.Body).Decode(&qi); err != nil {
			return nil, err
		}
		if qi.Executable == nil || qi.Executable.Number == 0 {
			return &jenkinsBuild{Building: false, Result: nil, URL: ref.ExternalID}, nil // 仍在排队
		}
		return p.fetchBuild(ctx, job, qi.Executable.Number)
	}
	// 情形二："job#n" 或纯 "n"（配合 Pipeline/ref）
	n := ref.ExternalID
	if i := strings.LastIndex(n, "#"); i >= 0 {
		n = n[i+1:]
	}
	num := 0
	if _, err := fmt.Sscanf(n, "%d", &num); err != nil || num <= 0 {
		// 无构建号：取最近一次构建
		return p.fetchLastBuild(ctx, job)
	}
	return p.fetchBuild(ctx, job, num)
}

func (p *jenkinsProvider) fetchBuild(ctx context.Context, job string, num int) (*jenkinsBuild, error) {
	resp, err := p.do(ctx, http.MethodGet,
		"/job/"+url.PathEscape(job)+"/"+fmt.Sprint(num)+"/api/json", nil, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jenkins: build http %d", resp.StatusCode)
	}
	var b jenkinsBuild
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (p *jenkinsProvider) fetchLastBuild(ctx context.Context, job string) (*jenkinsBuild, error) {
	resp, err := p.do(ctx, http.MethodGet,
		"/job/"+url.PathEscape(job)+"/lastBuild/api/json", nil, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jenkins: lastBuild http %d", resp.StatusCode)
	}
	var b jenkinsBuild
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (p *jenkinsProvider) GetBuild(ctx context.Context, ref cicd.BuildRef) (*cicd.BuildStatus, error) {
	b, err := p.resolveBuild(ctx, ref)
	if err != nil {
		return nil, err
	}
	st := &cicd.BuildStatus{
		Pipeline:   ref.Pipeline,
		ExternalID: ref.ExternalID,
		Status:     mapJenkinsResult(b),
		WebURL:     b.URL,
	}
	if b.Timestamp > 0 {
		t := time.UnixMilli(b.Timestamp)
		st.StartedAt = &t
		if !b.Building && b.Result != nil {
			f := t.Add(time.Duration(b.Duration) * time.Millisecond)
			st.FinishedAt = &f
		}
	}
	// 解析出真实构建号后规范化 ExternalID（队列 URL → 稳定的 job#n 标识）
	if b.Number > 0 {
		job := ref.Pipeline
		if job == "" {
			job = p.job
		}
		st.ExternalID = fmt.Sprintf("%s#%d", job, b.Number)
	}
	return st, nil
}

func (p *jenkinsProvider) ListArtifacts(ctx context.Context, ref cicd.BuildRef) ([]cicd.Artifact, error) {
	b, err := p.resolveBuild(ctx, ref)
	if err != nil {
		return nil, err
	}
	out := make([]cicd.Artifact, 0, len(b.Artifacts))
	for _, a := range b.Artifacts {
		base := b.URL
		if base == "" && b.Number > 0 {
			base = fmt.Sprintf("%s/job/%s/%d/", p.base, url.PathEscape(ref.Pipeline), b.Number)
		}
		out = append(out, cicd.Artifact{
			Name: a.FileName,
			Path: a.RelativePath,
			URL:  base + "artifact/" + a.RelativePath,
		})
	}
	return out, nil
}
