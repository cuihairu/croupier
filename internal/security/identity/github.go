package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// GitHubConfig 是 GitHub OAuth App 登录的参数。
type GitHubConfig struct {
	// ClientID / ClientSecret 是 GitHub OAuth App 凭证。
	ClientID     string
	ClientSecret string
	// RedirectURL 是回调地址，须与 GitHub App 注册一致，
	// 如 "https://croupier.example.com/api/v1/auth/github/callback"。
	RedirectURL string
	// APIBase 覆盖 GitHub API 地址（企业版自建实例用；测试注入 httptest）。
	APIBase string
	// AuthBase 覆盖 GitHub 授权/令牌端点基地址（同上，默认 github.com）。
	AuthBase string
}

// GitHubProvider 基于标准 OAuth2 授权码流程认证（非 OIDC：GitHub 不签发
// id_token，身份经 REST API /user 与 /user/emails 换取）。
type GitHubProvider struct {
	oauth2Config oauth2.Config
	apiBase      string
	authBase     string
	client       *http.Client
}

// NewGitHubProvider 创建 GitHub 提供方（无网络依赖：端点静态已知，
// 认证时才发起请求——与 LDAP 同款失效时序）。
func NewGitHubProvider(cfg GitHubConfig) (*GitHubProvider, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, errors.New("github: clientId/clientSecret is required")
	}
	authBase := strings.TrimRight(cfg.AuthBase, "/")
	apiBase := strings.TrimRight(cfg.APIBase, "/")
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	// 默认端点取 x/oauth2 github 常量；自建实例经 AuthBase 覆盖。
	endpoint := github.Endpoint
	if authBase != "" {
		endpoint = oauth2.Endpoint{
			AuthURL:  authBase + "/login/oauth/authorize",
			TokenURL: authBase + "/login/oauth/access_token",
		}
	}
	return &GitHubProvider{
		oauth2Config: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     endpoint,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"read:user", "user:email"},
		},
		apiBase:  apiBase,
		authBase: authBase,
		client:   &http.Client{},
	}, nil
}

// Kind 实现 OAuthProvider。
func (p *GitHubProvider) Kind() string { return KindGitHub }

// AuthCodeURL 实现 OAuthProvider：生成跳转到 GitHub 的授权 URL。
func (p *GitHubProvider) AuthCodeURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

// Exchange 实现 OAuthProvider：授权码换 token 后经 REST API 拉取身份。
// Username 取 GitHub login（恒存在且唯一）；email 优先 profile 主邮箱，
// 私密邮箱场景回退 /user/emails 的 primary。
func (p *GitHubProvider) Exchange(ctx context.Context, code string) (*Identity, error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("github code exchange: %w", err)
	}

	var profile struct {
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := p.apiGet(ctx, token.AccessToken, "/user", &profile); err != nil {
		return nil, fmt.Errorf("github fetch user: %w", err)
	}
	if strings.TrimSpace(profile.Login) == "" {
		return nil, errors.New("github: profile has no login")
	}

	email := strings.TrimSpace(profile.Email)
	if email == "" {
		email = p.primaryEmail(ctx, token.AccessToken)
	}

	return &Identity{
		Provider: KindGitHub,
		Username: profile.Login,
		Nickname: firstNonEmpty(strings.TrimSpace(profile.Name), profile.Login),
		Email:    email,
	}, nil
}

// primaryEmail 拉取 /user/emails 取 primary 邮箱；失败静默回空
// （email 非登录必需要素，私有邮箱 API 拒绝不应阻断登录）。
func (p *GitHubProvider) primaryEmail(ctx context.Context, accessToken string) string {
	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	if err := p.apiGet(ctx, accessToken, "/user/emails", &emails); err != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary {
			return strings.TrimSpace(e.Email)
		}
	}
	return ""
}

func (p *GitHubProvider) apiGet(ctx context.Context, accessToken, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github api %s: HTTP %d: %s", path, resp.StatusCode, truncateForLog(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("github api %s: decode: %w", path, err)
	}
	return nil
}

func truncateForLog(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
