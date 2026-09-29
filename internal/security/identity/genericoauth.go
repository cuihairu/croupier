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
)

// KindGenericOAuth 是自定义 OAuth2 提供方的种类标识（auth.genericoauth.*）。
const KindGenericOAuth = "generic-oauth"

// GenericOAuthConfig 是自定义 OAuth2（授权码流程）登录的参数。适配任意
// 遵循标准 authorization code flow 的身份源（Keycloak/Authentik/Auth0/
// Gitea/企业内网 SSO 等），身份字段经 UserInfo 端点的 JSON 属性映射。
type GenericOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// AuthURL / TokenURL 是身份源的授权与令牌端点（必填）。
	AuthURL  string
	TokenURL string
	// UserInfoURL 是换取身份的端点（必填），以 Bearer 访问令牌请求，
	// 响应必须是 JSON 对象。
	UserInfoURL string
	// Scopes 是授权范围（可空）。
	Scopes []string
	// UsernameField / NicknameField / EmailField 是 UserInfo JSON 中的
	// 属性名（默认 username / name / email；仅支持顶层字段，嵌套属性
	// 属于 IdP 侧映射职责，本实现不做 JSON Path 求值）。
	UsernameField string
	NicknameField string
	EmailField    string
	// HTTPClient 覆盖出站客户端（测试注入 httptest；生产留空）。
	HTTPClient *http.Client
}

// GenericOAuthProvider 实现标准 OAuth2 授权码流程的自定义身份源。
type GenericOAuthProvider struct {
	oauth2Config oauth2.Config
	userInfoURL  string
	fieldMap     genericFieldMap
	client       *http.Client
}

type genericFieldMap struct {
	username string
	nickname string
	email    string
}

// NewGenericOAuthProvider 创建自定义 OAuth2 提供方（无网络依赖：构建只做
// 参数校验，认证时才发起请求——与 GitHub 同款失效时序）。
func NewGenericOAuthProvider(cfg GenericOAuthConfig) (*GenericOAuthProvider, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, errors.New("generic-oauth: clientId/clientSecret is required")
	}
	if strings.TrimSpace(cfg.AuthURL) == "" || strings.TrimSpace(cfg.TokenURL) == "" {
		return nil, errors.New("generic-oauth: authUrl/tokenUrl is required")
	}
	if strings.TrimSpace(cfg.UserInfoURL) == "" {
		return nil, errors.New("generic-oauth: userInfoUrl is required")
	}
	return &GenericOAuthProvider{
		oauth2Config: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  cfg.AuthURL,
				TokenURL: cfg.TokenURL,
			},
			RedirectURL: cfg.RedirectURL,
			Scopes:      cfg.Scopes,
		},
		userInfoURL: strings.TrimSpace(cfg.UserInfoURL),
		fieldMap: genericFieldMap{
			username: firstNonEmpty(cfg.UsernameField, "username"),
			nickname: firstNonEmpty(cfg.NicknameField, "name"),
			email:    firstNonEmpty(cfg.EmailField, "email"),
		},
		client: cfg.HTTPClient,
	}, nil
}

// Kind 实现 OAuthProvider。
func (p *GenericOAuthProvider) Kind() string { return KindGenericOAuth }

// AuthCodeURL 实现 OAuthProvider。
func (p *GenericOAuthProvider) AuthCodeURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

// Exchange 实现 OAuthProvider：授权码换 token 后请求 UserInfo 端点，
// 按属性映射提取身份。Username 属性缺失或为空视为失败（登录必需）；
// Nickname 回退 Username；Email 缺失为空（非必需）。
func (p *GenericOAuthProvider) Exchange(ctx context.Context, code string) (*Identity, error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("generic-oauth code exchange: %w", err)
	}

	attrs, err := p.fetchUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}

	username := strings.TrimSpace(stringField(attrs, p.fieldMap.username))
	if username == "" {
		return nil, fmt.Errorf("generic-oauth: userinfo has no %q field", p.fieldMap.username)
	}
	nickname := strings.TrimSpace(stringField(attrs, p.fieldMap.nickname))
	return &Identity{
		Provider: KindGenericOAuth,
		Username: username,
		Nickname: firstNonEmpty(nickname, username),
		Email:    strings.TrimSpace(stringField(attrs, p.fieldMap.email)),
	}, nil
}

func (p *GenericOAuthProvider) fetchUserInfo(ctx context.Context, accessToken string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	client := p.client
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generic-oauth userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("generic-oauth userinfo: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("generic-oauth userinfo: HTTP %d: %s", resp.StatusCode, truncateForLog(string(body)))
	}
	var attrs map[string]any
	if err := json.Unmarshal(body, &attrs); err != nil {
		return nil, fmt.Errorf("generic-oauth userinfo: decode: %w", err)
	}
	return attrs, nil
}

// stringField 从 JSON 对象取顶层字符串属性（数值/布尔不适用）。
func stringField(attrs map[string]any, key string) string {
	v, _ := attrs[key].(string)
	return v
}
