package configsource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// nacosSource browses a Nacos config center via its OpenAPI. dataId 即路径
// （dataId 中的 '/' 视作目录分隔）；group 固定为一个绑定值。
// 可写：应急编辑 = publish 回 Nacos（游戏服长轮询消费方式不变）。
//
// Config: {"endpoint", "namespaceId", "group", "username", "password"}.
type nacosSource struct {
	endpoint    string
	namespaceID string
	group       string
	username    string
	password    string
	httpClient  *http.Client

	token   string
	tokenAt time.Time
}

func newNacosSource(cfg map[string]interface{}) (Source, error) {
	endpoint := strings.TrimSuffix(configString(cfg, "endpoint", ""), "/")
	if endpoint == "" {
		return nil, fmt.Errorf("nacos source requires endpoint")
	}
	group := configString(cfg, "group", "DEFAULT_GROUP")
	return &nacosSource{
		endpoint:    endpoint,
		namespaceID: configString(cfg, "namespaceId", ""),
		group:       group,
		username:    configString(cfg, "username", ""),
		password:    configString(cfg, "password", ""),
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (s *nacosSource) Type() string { return "nacos" }

// accessToken logs in when credentials are configured (Nacos 开启鉴权时)。
func (s *nacosSource) accessToken(ctx context.Context) (string, error) {
	if s.username == "" {
		return "", nil
	}
	if s.token != "" && time.Since(s.tokenAt) < 4*time.Hour {
		return s.token, nil
	}
	form := url.Values{"username": {s.username}, "password": {s.password}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.endpoint+"/nacos/v1/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("nacos login: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nacos login failed: status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return "", fmt.Errorf("nacos login: bad response")
	}
	s.token, s.tokenAt = out.AccessToken, time.Now()
	return s.token, nil
}

func (s *nacosSource) withAuth(ctx context.Context, q url.Values) error {
	token, err := s.accessToken(ctx)
	if err != nil {
		return err
	}
	if token != "" {
		q.Set("accessToken", token)
	}
	if s.namespaceID != "" {
		q.Set("tenant", s.namespaceID)
	}
	return nil
}

// nacosConfigItem is one entry from the blur-search list API.
type nacosConfigItem struct {
	DataID string `json:"dataId"`
	Group  string `json:"group"`
}

func (s *nacosSource) listAll(ctx context.Context) ([]nacosConfigItem, error) {
	items := []nacosConfigItem{}
	for page := 1; page <= 50; page++ { // 页数上限保护
		q := url.Values{
			"search":   {"blur"},
			"dataId":   {""},
			"group":    {s.group},
			"pageNo":   {fmt.Sprintf("%d", page)},
			"pageSize": {"200"},
		}
		if err := s.withAuth(ctx, q); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			s.endpoint+"/nacos/v1/cs/configs?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("nacos list: %w", err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("nacos list failed: status %d", resp.StatusCode)
		}
		var pageOut struct {
			PageItems []nacosConfigItem `json:"pageItems"`
		}
		if err := json.Unmarshal(body, &pageOut); err != nil {
			return nil, fmt.Errorf("nacos list: bad response")
		}
		items = append(items, pageOut.PageItems...)
		if len(pageOut.PageItems) < 200 {
			break
		}
	}
	return items, nil
}

func (s *nacosSource) List(ctx context.Context, dir string) ([]Entry, error) {
	dir, err := cleanPath(dir)
	if err != nil {
		return nil, err
	}
	items, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	dirs := map[string]struct{}{}
	files := map[string]struct{}{}
	for _, item := range items {
		rest := strings.TrimPrefix(item.DataID, prefix)
		if rest == item.DataID && prefix != "" {
			continue
		}
		if rest == "" {
			continue
		}
		if i := strings.Index(rest, "/"); i >= 0 {
			dirs[rest[:i]] = struct{}{}
		} else {
			files[rest] = struct{}{}
		}
	}
	out := make([]Entry, 0, len(dirs)+len(files))
	for name := range dirs {
		out = append(out, Entry{Name: name, Path: joinSub(dir, name), Dir: true})
	}
	for name := range files {
		out = append(out, Entry{Name: name, Path: joinSub(dir, name)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *nacosSource) Read(ctx context.Context, path string) ([]byte, error) {
	path, err := cleanPath(path)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("path required")
	}
	q := url.Values{"dataId": {path}, "group": {s.group}}
	if err := s.withAuth(ctx, q); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		s.endpoint+"/nacos/v1/cs/configs?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nacos get: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("config not found: %s", path)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nacos get failed: status %d", resp.StatusCode)
	}
	return body, nil
}

// Write implements emergency edit: publish the content back to Nacos.
func (s *nacosSource) Write(ctx context.Context, path string, content []byte, _ string) error {
	path, err := cleanPath(path)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("path required")
	}
	if len(content) > 8<<20 {
		return fmt.Errorf("content too large (>8MiB)")
	}
	form := url.Values{
		"dataId":  {path},
		"group":   {s.group},
		"content": {string(content)},
	}
	if err := s.withAuth(ctx, form); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.endpoint+"/nacos/v1/cs/configs", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nacos publish: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "true" {
		return fmt.Errorf("nacos publish failed: status %d body %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
