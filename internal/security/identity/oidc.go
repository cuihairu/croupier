package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig 是 OIDC 提供方的参数。
type OIDCConfig struct {
	// Issuer 是身份源签发者地址，如 "https://keycloak.example.com/realms/main"。
	// 提供方会自动发现 /.well-known/openid-configuration。
	Issuer string
	// ClientID / ClientSecret 是在本平台注册的 OAuth2 客户端凭证。
	ClientID     string
	ClientSecret string
	// RedirectURL 是回调地址，须与身份源注册的一致，
	// 如 "https://croupier.example.com/api/auth/oidc/callback"。
	RedirectURL string
	// Scopes 默认 ["openid","profile","email"]。
	Scopes []string
	// UsernameClaim 是从 id_token 提取登录名的 claim，默认 "preferred_username"。
	UsernameClaim string
}

// OIDCProvider 基于标准 OIDC 授权码流程认证。
type OIDCProvider struct {
	provider      *oidc.Provider
	oauth2Config  oauth2.Config
	verifier      *oidc.IDTokenVerifier
	usernameClaim string
}

// NewOIDCProvider 创建 OIDC 提供方，构造时会访问 Issuer 的发现端点。
func NewOIDCProvider(ctx context.Context, cfg OIDCConfig) (*OIDCProvider, error) {
	if strings.TrimSpace(cfg.Issuer) == "" {
		return nil, errors.New("oidc: issuer is required")
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	usernameClaim := cfg.UsernameClaim
	if usernameClaim == "" {
		usernameClaim = "preferred_username"
	}
	return &OIDCProvider{
		provider: provider,
		oauth2Config: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       scopes,
		},
		verifier:      provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		usernameClaim: usernameClaim,
	}, nil
}

// Kind 实现 OAuthProvider。
func (p *OIDCProvider) Kind() string { return KindOIDC }

// AuthCodeURL 实现 OAuthProvider：生成跳转到身份源的授权 URL。
func (p *OIDCProvider) AuthCodeURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

// Exchange 实现 OAuthProvider：授权码换 token，校验 id_token 签名与签发者，
// 并从 claim 提取身份。username claim 缺失时依次回退 sub / email。
func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*Identity, error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oidc code exchange: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("oidc: no id_token in token response")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oidc verify id_token: %w", err)
	}

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc parse claims: %w", err)
	}

	username := claimString(claims, p.usernameClaim)
	if username == "" {
		username = claimString(claims, "sub")
	}
	if username == "" {
		username = claimString(claims, "email")
	}
	if username == "" {
		return nil, fmt.Errorf("oidc: id_token has no usable username claim (tried %q, sub, email)", p.usernameClaim)
	}

	ident := &Identity{
		Provider: KindOIDC,
		Username: strings.TrimSpace(username),
		Nickname: firstNonEmpty(claimString(claims, "name"), claimString(claims, "given_name"), username),
		Email:    claimString(claims, "email"),
	}
	return ident, nil
}

func claimString(claims map[string]interface{}, key string) string {
	v, ok := claims[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
