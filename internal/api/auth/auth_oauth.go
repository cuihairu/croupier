package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/security/identity"
)

// OIDCAuthCodeURL generates the authorization URL for redirecting to the
// identity source; the state carries an HMAC signature and a timestamp.
func (s *Service) OIDCAuthCodeURL() (string, error) {
	_, oidc, _, _ := s.snapshotProviders()
	if oidc == nil {
		return "", errors.New("OIDC 登录未启用")
	}
	return oidc.AuthCodeURL(s.newOIDCState()), nil
}

// githubSnapshot returns the GitHub provider and success redirect URL (a safe
// read under hot refresh).
func (s *Service) githubSnapshot() (identity.OAuthProvider, string) {
	s.providersMu.RLock()
	defer s.providersMu.RUnlock()
	return s.github, s.githubSuccessURL
}

// GitHubEnabled reports whether the GitHub login flow is wired.
func (s *Service) GitHubEnabled() bool {
	g, _ := s.githubSnapshot()
	return g != nil
}

// GitHubAuthCodeURL generates the authorization URL for redirecting to GitHub
// (the state reuses the OIDC HMAC signature mechanism, with a 10 minute
// validity window).
func (s *Service) GitHubAuthCodeURL() (string, error) {
	g, _ := s.githubSnapshot()
	if g == nil {
		return "", errors.New("GitHub 登录未启用")
	}
	return g.AuthCodeURL(s.newOIDCState()), nil
}

// GitHubLoginCallback handles the GitHub callback (see externalOAuthCallback
// for the shared exit).
func (s *Service) GitHubLoginCallback(ctx context.Context, code, state string, req *LoginRequest) (*LoginResponse, error) {
	g, _ := s.githubSnapshot()
	return s.externalOAuthCallback(ctx, g, identity.KindGitHub, "GitHub", code, state, req)
}

// GitHubSuccessURL returns the GitHub callback success redirect URL (may be empty).
func (s *Service) GitHubSuccessURL() string {
	_, url := s.githubSnapshot()
	return url
}

// wechatSnapshot returns the WeChat QR-code provider and success redirect URL
// (a safe read under hot refresh).
func (s *Service) wechatSnapshot() (identity.OAuthProvider, string) {
	s.providersMu.RLock()
	defer s.providersMu.RUnlock()
	return s.wechat, s.wechatSuccessURL
}

// WeChatEnabled reports whether the WeChat QR login flow is wired.
func (s *Service) WeChatEnabled() bool {
	p, _ := s.wechatSnapshot()
	return p != nil
}

// WeChatAuthCodeURL generates the WeChat QR-code login page URL (the state
// reuses the OIDC HMAC signature mechanism, with a 10 minute validity window).
func (s *Service) WeChatAuthCodeURL() (string, error) {
	p, _ := s.wechatSnapshot()
	if p == nil {
		return "", errors.New("微信登录未启用")
	}
	return p.AuthCodeURL(s.newOIDCState()), nil
}

// WeChatLoginCallback handles the WeChat callback (see externalOAuthCallback
// for the shared exit).
func (s *Service) WeChatLoginCallback(ctx context.Context, code, state string, req *LoginRequest) (*LoginResponse, error) {
	p, _ := s.wechatSnapshot()
	return s.externalOAuthCallback(ctx, p, identity.KindWeChat, "微信", code, state, req)
}

// WeChatSuccessURL returns the WeChat callback success redirect URL (may be empty).
func (s *Service) WeChatSuccessURL() string {
	_, url := s.wechatSnapshot()
	return url
}

// genericOAuthSnapshot returns the custom OAuth provider and success redirect
// URL (a safe read under hot refresh).
func (s *Service) genericOAuthSnapshot() (identity.OAuthProvider, string) {
	s.providersMu.RLock()
	defer s.providersMu.RUnlock()
	return s.genericOAuth, s.genericOAuthSuccessURL
}

// GenericOAuthEnabled reports whether the custom OAuth login flow is wired.
func (s *Service) GenericOAuthEnabled() bool {
	p, _ := s.genericOAuthSnapshot()
	return p != nil
}

// GenericOAuthAuthCodeURL generates the custom OAuth authorization URL (the
// state reuses the OIDC HMAC signature mechanism, with a 10 minute validity
// window).
func (s *Service) GenericOAuthAuthCodeURL() (string, error) {
	p, _ := s.genericOAuthSnapshot()
	if p == nil {
		return "", errors.New("自定义 OAuth 登录未启用")
	}
	return p.AuthCodeURL(s.newOIDCState()), nil
}

// GenericOAuthLoginCallback handles the custom OAuth callback (see
// externalOAuthCallback for the shared exit).
func (s *Service) GenericOAuthLoginCallback(ctx context.Context, code, state string, req *LoginRequest) (*LoginResponse, error) {
	p, _ := s.genericOAuthSnapshot()
	return s.externalOAuthCallback(ctx, p, identity.KindGenericOAuth, "自定义 OAuth", code, state, req)
}

// GenericOAuthSuccessURL returns the custom OAuth callback success redirect
// URL (may be empty).
func (s *Service) GenericOAuthSuccessURL() string {
	_, url := s.genericOAuthSnapshot()
	return url
}

// externalOAuthCallback is the shared exit for redirect-based authorization
// callbacks: verify state (HMAC, 10 minute validity) → exchange the
// authorization code for an identity → JIT-resolve/provision the local shadow
// account → issue the platform token. The four flows (OIDC/GitHub/WeChat/custom
// OAuth) are line-for-line identical and differ only in kind (audit metadata)
// and label (error wording). kind/label are passed in per provider by the
// caller.
func (s *Service) externalOAuthCallback(ctx context.Context, p identity.OAuthProvider, kind, label, code, state string, req *LoginRequest) (*LoginResponse, error) {
	if p == nil {
		return nil, errors.New(label + " 登录未启用")
	}
	if !s.verifyOIDCState(state) {
		s.recordLoginAudit(ctx, "", "auth.login_failed", "failed", req, "invalid_state", kind)
		return nil, errors.New("登录状态校验失败，请重新发起登录")
	}
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("缺少授权码")
	}

	ident, err := p.Exchange(ctx, code)
	if err != nil {
		s.recordLoginAudit(ctx, "", "auth.login_failed", "failed", req, kind+"_exchange_failed", kind)
		return nil, errors.New(label + " 登录失败")
	}

	admin, err := s.resolveAdminForIdentity(ctx, ident)
	if err != nil {
		s.recordLoginAudit(ctx, ident.Username, "auth.login_failed", "failed", req, "provision_failed", ident.Provider)
		return nil, errors.New("登录失败")
	}
	return s.issueLogin(ctx, admin, ident, req)
}

// OIDCLoginCallback handles the callback: verifies state, exchanges the
// authorization code for an identity, JIT-resolves the local account, and
// issues the platform JWT.
func (s *Service) OIDCLoginCallback(ctx context.Context, code, state string, req *LoginRequest) (*LoginResponse, error) {
	_, oidc, _, _ := s.snapshotProviders()
	return s.externalOAuthCallback(ctx, oidc, identity.KindOIDC, "OIDC", code, state, req)
}

// OIDCSuccessURL returns the configured login success redirect URL (may be empty).
func (s *Service) OIDCSuccessURL() string {
	_, _, url, _ := s.snapshotProviders()
	return url
}
