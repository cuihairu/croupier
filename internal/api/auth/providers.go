package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/security/identity"
)

// BuildIdentityProviders 从分层设置（yaml 初始值 + database L3 覆盖）
// 构建外部身份提供方，供路由装配时调用。返回的句柄通过 Attach 挂到 Service。
//
// 构建策略是"失效降级"而非启动失败：
//   - LDAP 无网络依赖（认证时才拨号），配置不完整直接报错提示；
//   - OIDC 构建需要访问 Issuer 发现端点，失败时告警并跳过
//     （避免身份源短暂不可用拖垮整个 Server 启动）。
func BuildIdentityProviders(cfg config.AuthProvidersConfig) (*IdentityProviders, error) {
	return buildIdentityProviders(cfg)
}

// IdentityProviders 是装配结果：待挂到 Service 上的提供方与其配套参数。
type IdentityProviders struct {
	localEnabled bool
	ldap         identity.PasswordProvider
	ldapRoles    []string
	oidc         identity.OAuthProvider
	oidcRoles    []string
	oidcURL      string
	github       identity.OAuthProvider
	githubRoles  []string
	githubURL    string
	// wechat / genericOAuth 是扩展外部 OAuth 提供方（#51 第三批）。
	wechat            identity.OAuthProvider
	wechatRoles       []string
	wechatURL         string
	genericOAuth      identity.OAuthProvider
	genericOAuthRoles []string
	genericOAuthURL   string
	// register 是自助注册开关（#51b，默认关）与注册默认角色。
	registerEnabled bool
	registerRoles   []string
}

func buildIdentityProviders(cfg config.AuthProvidersConfig) (*IdentityProviders, error) {
	out := &IdentityProviders{}
	out.localEnabled = cfg.Local.LocalEnabled()
	out.registerEnabled = cfg.Register.Enabled
	out.registerRoles = cfg.Register.DefaultRoles

	if cfg.LDAP.Enabled {
		lc := cfg.LDAP
		if lc.Addr == "" || lc.BaseDN == "" {
			if lc.Addr == "" && lc.UserDNTemplate == "" {
				return nil, errors.New("auth.providers.ldap: addr/baseDn 未配置")
			}
		}
		if lc.BindDN == "" && lc.UserDNTemplate == "" && lc.BaseDN == "" {
			return nil, errors.New("auth.providers.ldap: 需要 baseDn（搜索）或 userDnTemplate（直连）之一")
		}
		out.ldap = identity.NewLDAPProvider(identity.LDAPConfig{
			Addr:               lc.Addr,
			BaseDN:             lc.BaseDN,
			BindDN:             lc.BindDN,
			BindPassword:       lc.BindPassword,
			UserFilter:         lc.UserFilter,
			UserDNTemplate:     lc.UserDNTemplate,
			StartTLS:           lc.StartTLS,
			InsecureSkipVerify: lc.InsecureSkipVerify,
		})
		out.ldapRoles = lc.DefaultRoles
	}

	if cfg.OIDC.Enabled {
		oc := cfg.OIDC
		if oc.Issuer == "" || oc.ClientID == "" || oc.RedirectURL == "" {
			return nil, errors.New("auth.providers.oidc: issuer/clientId/redirectUrl 未配置")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		oidcProvider, err := identity.NewOIDCProvider(ctx, identity.OIDCConfig{
			Issuer:        oc.Issuer,
			ClientID:      oc.ClientID,
			ClientSecret:  oc.ClientSecret,
			RedirectURL:   oc.RedirectURL,
			Scopes:        oc.Scopes,
			UsernameClaim: oc.UsernameClaim,
		})
		if err != nil {
			// 失效降级：跳过 OIDC，本地与已启用的密码源不受影响。
			slog.Default().Error("OIDC provider init failed, OIDC login disabled", "error", err)
		} else {
			out.oidc = oidcProvider
			out.oidcRoles = oc.DefaultRoles
			out.oidcURL = oc.LoginSuccessURL
		}
	}

	if cfg.GitHub.Enabled {
		gc := cfg.GitHub
		if gc.ClientID == "" || gc.ClientSecret == "" || gc.RedirectURL == "" {
			return nil, errors.New("auth.providers.github: clientId/clientSecret/redirectUrl 未配置")
		}
		githubProvider, err := identity.NewGitHubProvider(identity.GitHubConfig{
			ClientID:     gc.ClientID,
			ClientSecret: gc.ClientSecret,
			RedirectURL:  gc.RedirectURL,
		})
		if err != nil {
			// 失效降级：跳过 GitHub，本地与已启用的密码源不受影响。
			slog.Default().Error("GitHub provider init failed, GitHub login disabled", "error", err)
		} else {
			out.github = githubProvider
			out.githubRoles = gc.DefaultRoles
			out.githubURL = gc.LoginSuccessURL
		}
	}

	if cfg.WeChat.Enabled {
		wc := cfg.WeChat
		if wc.AppID == "" || wc.AppSecret == "" || wc.RedirectURL == "" {
			return nil, errors.New("auth.providers.wechat: appId/appSecret/redirectUrl 未配置")
		}
		wechatProvider, err := identity.NewWeChatProvider(identity.WeChatConfig{
			AppID:       wc.AppID,
			AppSecret:   wc.AppSecret,
			RedirectURL: wc.RedirectURL,
		})
		if err != nil {
			// 失效降级：跳过 WeChat，本地与已启用的密码源不受影响。
			slog.Default().Error("WeChat provider init failed, WeChat login disabled", "error", err)
		} else {
			out.wechat = wechatProvider
			out.wechatRoles = wc.DefaultRoles
			out.wechatURL = wc.LoginSuccessURL
		}
	}

	if cfg.GenericOAuth.Enabled {
		gc := cfg.GenericOAuth
		if gc.ClientID == "" || gc.ClientSecret == "" || gc.RedirectURL == "" ||
			gc.AuthURL == "" || gc.TokenURL == "" || gc.UserInfoURL == "" {
			return nil, errors.New("auth.providers.genericoauth: clientId/clientSecret/redirectUrl/authUrl/tokenUrl/userInfoUrl 未配置")
		}
		genericProvider, err := identity.NewGenericOAuthProvider(identity.GenericOAuthConfig{
			ClientID:      gc.ClientID,
			ClientSecret:  gc.ClientSecret,
			RedirectURL:   gc.RedirectURL,
			AuthURL:       gc.AuthURL,
			TokenURL:      gc.TokenURL,
			UserInfoURL:   gc.UserInfoURL,
			Scopes:        splitScopes(gc.Scopes),
			UsernameField: gc.UsernameField,
			NicknameField: gc.NicknameField,
			EmailField:    gc.EmailField,
		})
		if err != nil {
			// 失效降级：跳过自定义 OAuth，本地与已启用的密码源不受影响。
			slog.Default().Error("Generic OAuth provider init failed, custom OAuth login disabled", "error", err)
		} else {
			out.genericOAuth = genericProvider
			out.genericOAuthRoles = gc.DefaultRoles
			out.genericOAuthURL = gc.LoginSuccessURL
		}
	}

	return out, nil
}

// splitScopes 切分逗号分隔的 scope 串（去空白项，空串返回 nil）。
func splitScopes(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// attach 将装配结果挂到 Service。
func (p *IdentityProviders) Attach(svc *Service) *Service {
	if p == nil {
		return svc
	}
	if p.ldap != nil {
		svc.WithPasswordProvider(p.ldap)
		svc.WithProviderDefaultRoles(identity.KindLDAP, p.ldapRoles)
	}
	if p.oidc != nil {
		svc.WithOIDCProvider(p.oidc, p.oidcRoles, p.oidcURL)
	}
	if p.github != nil {
		svc.WithGitHubProvider(p.github, p.githubRoles, p.githubURL)
	}
	if p.wechat != nil {
		svc.WithWeChatProvider(p.wechat, p.wechatRoles, p.wechatURL)
	}
	if p.genericOAuth != nil {
		svc.WithGenericOAuthProvider(p.genericOAuth, p.genericOAuthRoles, p.genericOAuthURL)
	}
	svc.WithLocalEnabled(p.localEnabled)
	svc.WithRegister(p.registerEnabled, p.registerRoles)
	return svc
}
