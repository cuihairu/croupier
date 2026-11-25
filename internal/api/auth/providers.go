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

// BuildIdentityProviders builds the external identity providers from the
// layered settings (yaml initial values + database L3 overrides), called during
// route assembly. The returned handle is wired onto the Service via Attach.
//
// The build strategy is "degrade on failure" rather than fail startup:
//   - LDAP has no network dependency (it dials only at authentication time);
//     an incomplete configuration is reported as an error;
//   - building OIDC requires reaching the Issuer's discovery endpoint, so a
//     failure only warns and skips it (a briefly unavailable identity source
//     must not drag down the whole Server startup).
func BuildIdentityProviders(cfg config.AuthProvidersConfig) (*IdentityProviders, error) {
	return buildIdentityProviders(cfg)
}

// IdentityProviders is the assembly result: the providers to be attached to the
// Service together with their companion parameters.
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
	// wechat / genericOAuth are extended external OAuth providers (#51, third
	// batch).
	wechat            identity.OAuthProvider
	wechatRoles       []string
	wechatURL         string
	genericOAuth      identity.OAuthProvider
	genericOAuthRoles []string
	genericOAuthURL   string
	// register is the self-service registration switch (#51b, off by default)
	// and the default roles for registration.
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
		// bare-background-ok: BuildIdentityProviders runs during startup
		// assembly (no request ctx); the 10s timeout covers OIDC discovery only
		// (CR-FIX-22 exemption list).
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
			// degrade on failure: skip OIDC; local and the enabled password
			// sources are unaffected.
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
			// degrade on failure: skip GitHub; local and the enabled password
			// sources are unaffected.
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
			// degrade on failure: skip WeChat; local and the enabled password
			// sources are unaffected.
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
			// degrade on failure: skip custom OAuth; local and the enabled
			// password sources are unaffected.
			slog.Default().Error("Generic OAuth provider init failed, custom OAuth login disabled", "error", err)
		} else {
			out.genericOAuth = genericProvider
			out.genericOAuthRoles = gc.DefaultRoles
			out.genericOAuthURL = gc.LoginSuccessURL
		}
	}

	return out, nil
}

// splitScopes splits a comma-separated scope string (dropping blank entries;
// an empty string yields nil).
func splitScopes(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Attach wires the assembly result onto the Service.
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
