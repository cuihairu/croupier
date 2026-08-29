package auth

import (
	"context"
	"errors"
	"log/slog"
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
	ldap      identity.PasswordProvider
	ldapRoles []string
	oidc      identity.OAuthProvider
	oidcRoles []string
	oidcURL   string
}

func buildIdentityProviders(cfg config.AuthProvidersConfig) (*IdentityProviders, error) {
	out := &IdentityProviders{}

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

	return out, nil
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
	return svc
}
