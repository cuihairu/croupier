package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// KindWeChat 是微信开放平台扫码登录提供方的种类标识（auth.wechat.*）。
const KindWeChat = "wechat"

// WeChatConfig 是微信开放平台「网站应用扫码登录」的参数。
//
// 语义边界：微信不提供邮箱；username 取 openid（按应用唯一）。同一开放
// 平台主体下的 unionid 不做身份主键——应用在 standalone 与开放平台间迁移
// 时 openid 不变，主键选择以稳定优先（unionid 映射列为后续演进）。
type WeChatConfig struct {
	// AppID / AppSecret 是微信开放平台网站应用凭证。
	AppID     string
	AppSecret string
	// RedirectURL 是回调地址（须与开放平台「授权回调域」一致），
	// 如 "https://croupier.example.com/api/v1/auth/wechat/callback"。
	RedirectURL string
	// AuthBase 覆盖授权页基地址（默认 https://open.weixin.qq.com，测试注入）。
	AuthBase string
	// APIBase 覆盖 sns API 基地址（默认 https://api.weixin.qq.com，测试注入）。
	APIBase string
}

// WeChatProvider 基于微信开放平台扫码登录（qrconnect）认证。
// 流程形似 OAuth2 授权码，但令牌经 GET query 明文传输（appid/secret 做
// client 凭证）、身份经 /sns/userinfo 以 access_token+openid 换取，
// 与标准 oauth2.Config 不兼容，故独立实现 OAuthProvider。
type WeChatProvider struct {
	appID       string
	appSecret   string
	redirectURL string
	authBase    string
	apiBase     string
	client      *http.Client
}

// NewWeChatProvider 创建微信提供方（无网络依赖：认证时才发起请求）。
func NewWeChatProvider(cfg WeChatConfig) (*WeChatProvider, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" {
		return nil, errors.New("wechat: appId/appSecret is required")
	}
	authBase := strings.TrimRight(cfg.AuthBase, "/")
	if authBase == "" {
		authBase = "https://open.weixin.qq.com"
	}
	apiBase := strings.TrimRight(cfg.APIBase, "/")
	if apiBase == "" {
		apiBase = "https://api.weixin.qq.com"
	}
	return &WeChatProvider{
		appID:       strings.TrimSpace(cfg.AppID),
		appSecret:   strings.TrimSpace(cfg.AppSecret),
		redirectURL: cfg.RedirectURL,
		authBase:    authBase,
		apiBase:     apiBase,
		client:      &http.Client{},
	}, nil
}

// Kind 实现 OAuthProvider。
func (p *WeChatProvider) Kind() string { return KindWeChat }

// AuthCodeURL 实现 OAuthProvider：生成扫码登录页地址（qrconnect）。
// scope 固定 snsapi_login；尾缀 #wechat_redirect 为微信侧协议要求。
func (p *WeChatProvider) AuthCodeURL(state string) string {
	q := url.Values{}
	q.Set("appid", p.appID)
	q.Set("redirect_uri", p.RedirectURLOf())
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_login")
	q.Set("state", state)
	return p.authBase + "/connect/qrconnect?" + q.Encode() + "#wechat_redirect"
}

// RedirectURLOf 返回回调地址（测试断言用）。
func (p *WeChatProvider) RedirectURLOf() string { return p.redirectURL }

// snsToken 是 /sns/oauth2/access_token 的响应（取其 access_token/openid；
// 微信成功响应不带 refresh_token 场景不适用本登录态——登录即用即弃）。
type snsToken struct {
	AccessToken string `json:"access_token"`
	OpenID      string `json:"openid"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// snsUser 是 /sns/userinfo 的响应（email 不存在是微信协议事实）。
type snsUser struct {
	OpenID   string `json:"openid"`
	Nickname string `json:"nickname"`
	UnionID  string `json:"unionid"`
}

// Exchange 实现 OAuthProvider：授权码换 access_token 后拉取用户身份。
// 微信侧业务错误（errcode != 0，如 code 已用/过期）返回包裹
// ErrInvalidCredentials 的错误，调用方可与网络故障区分。
func (p *WeChatProvider) Exchange(ctx context.Context, code string) (*Identity, error) {
	q := url.Values{}
	q.Set("appid", p.appID)
	q.Set("secret", p.appSecret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")
	var token snsToken
	if err := p.apiGet(ctx, "/sns/oauth2/access_token?"+q.Encode(), &token); err != nil {
		return nil, fmt.Errorf("wechat token: %w", err)
	}
	if token.ErrCode != 0 {
		return nil, fmt.Errorf("wechat token: errcode=%d %s: %w", token.ErrCode, token.ErrMsg, ErrInvalidCredentials)
	}
	if strings.TrimSpace(token.AccessToken) == "" || strings.TrimSpace(token.OpenID) == "" {
		return nil, errors.New("wechat token: response missing access_token/openid")
	}

	uq := url.Values{}
	uq.Set("access_token", token.AccessToken)
	uq.Set("openid", token.OpenID)
	var user snsUser
	if err := p.apiGet(ctx, "/sns/userinfo?"+uq.Encode(), &user); err != nil {
		return nil, fmt.Errorf("wechat userinfo: %w", err)
	}
	openID := strings.TrimSpace(token.OpenID)
	if openID == "" {
		openID = strings.TrimSpace(user.OpenID)
	}
	if openID == "" {
		return nil, errors.New("wechat: identity has no openid")
	}
	return &Identity{
		Provider: KindWeChat,
		Username: openID,
		Nickname: firstNonEmpty(strings.TrimSpace(user.Nickname), openID),
	}, nil
}

func (p *WeChatProvider) apiGet(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiBase+path, nil)
	if err != nil {
		return err
	}
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
		return fmt.Errorf("wechat api: HTTP %d: %s", resp.StatusCode, truncateForLog(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("wechat api: decode: %w", err)
	}
	return nil
}
