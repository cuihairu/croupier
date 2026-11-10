package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/security/identity"
)

// OIDCAuthCodeURL 生成跳转到身份源的授权 URL，state 内含 HMAC 签名与时间戳。
func (s *Service) OIDCAuthCodeURL() (string, error) {
	_, oidc, _, _ := s.snapshotProviders()
	if oidc == nil {
		return "", errors.New("OIDC 登录未启用")
	}
	return oidc.AuthCodeURL(s.newOIDCState()), nil
}

// githubSnapshot 返回 GitHub 提供方与成功跳转地址（热刷新安全读取）。
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

// GitHubAuthCodeURL 生成跳转到 GitHub 的授权 URL（state 复用 OIDC 的
// HMAC 签名机制，含 10 分钟有效期）。
func (s *Service) GitHubAuthCodeURL() (string, error) {
	g, _ := s.githubSnapshot()
	if g == nil {
		return "", errors.New("GitHub 登录未启用")
	}
	return g.AuthCodeURL(s.newOIDCState()), nil
}

// GitHubLoginCallback 处理 GitHub 回调（公共出口见 externalOAuthCallback）。
func (s *Service) GitHubLoginCallback(ctx context.Context, code, state string, req *LoginRequest) (*LoginResponse, error) {
	g, _ := s.githubSnapshot()
	return s.externalOAuthCallback(ctx, g, identity.KindGitHub, "GitHub", code, state, req)
}

// GitHubSuccessURL 返回 GitHub 回调成功跳转地址（可为空）。
func (s *Service) GitHubSuccessURL() string {
	_, url := s.githubSnapshot()
	return url
}

// wechatSnapshot 返回微信扫码提供方与成功跳转地址（热刷新安全读取）。
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

// WeChatAuthCodeURL 生成微信扫码登录页地址（state 复用 OIDC 的 HMAC
// 签名机制，含 10 分钟有效期）。
func (s *Service) WeChatAuthCodeURL() (string, error) {
	p, _ := s.wechatSnapshot()
	if p == nil {
		return "", errors.New("微信登录未启用")
	}
	return p.AuthCodeURL(s.newOIDCState()), nil
}

// WeChatLoginCallback 处理微信回调（公共出口见 externalOAuthCallback）。
func (s *Service) WeChatLoginCallback(ctx context.Context, code, state string, req *LoginRequest) (*LoginResponse, error) {
	p, _ := s.wechatSnapshot()
	return s.externalOAuthCallback(ctx, p, identity.KindWeChat, "微信", code, state, req)
}

// WeChatSuccessURL 返回微信回调成功跳转地址（可为空）。
func (s *Service) WeChatSuccessURL() string {
	_, url := s.wechatSnapshot()
	return url
}

// genericOAuthSnapshot 返回自定义 OAuth 提供方与成功跳转地址（热刷新安全读取）。
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

// GenericOAuthAuthCodeURL 生成自定义 OAuth 的授权 URL（state 复用 OIDC 的
// HMAC 签名机制，含 10 分钟有效期）。
func (s *Service) GenericOAuthAuthCodeURL() (string, error) {
	p, _ := s.genericOAuthSnapshot()
	if p == nil {
		return "", errors.New("自定义 OAuth 登录未启用")
	}
	return p.AuthCodeURL(s.newOIDCState()), nil
}

// GenericOAuthLoginCallback 处理自定义 OAuth 回调（公共出口见 externalOAuthCallback）。
func (s *Service) GenericOAuthLoginCallback(ctx context.Context, code, state string, req *LoginRequest) (*LoginResponse, error) {
	p, _ := s.genericOAuthSnapshot()
	return s.externalOAuthCallback(ctx, p, identity.KindGenericOAuth, "自定义 OAuth", code, state, req)
}

// GenericOAuthSuccessURL 返回自定义 OAuth 回调成功跳转地址（可为空）。
func (s *Service) GenericOAuthSuccessURL() string {
	_, url := s.genericOAuthSnapshot()
	return url
}

// externalOAuthCallback 是重定向授权型回调的公共出口：校验 state（HMAC，
// 10 分钟有效）→ 授权码换身份 → JIT 解析/创建本地影子账号 → 签发平台
// token。OIDC/GitHub/WeChat/自定义 OAuth 四路逐行同构，仅 kind（审计
// metadata）与 label（错误文案）不同。kind/label 由调用方按提供方传入。
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

// OIDCLoginCallback 处理回调：校验 state，用授权码换取身份，JIT 解析本地
// 账号后签发平台 JWT。
func (s *Service) OIDCLoginCallback(ctx context.Context, code, state string, req *LoginRequest) (*LoginResponse, error) {
	_, oidc, _, _ := s.snapshotProviders()
	return s.externalOAuthCallback(ctx, oidc, identity.KindOIDC, "OIDC", code, state, req)
}

// OIDCSuccessURL 返回配置的登录成功跳转地址（可为空）。
func (s *Service) OIDCSuccessURL() string {
	_, _, url, _ := s.snapshotProviders()
	return url
}
