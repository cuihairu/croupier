package settings

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/config"
)

// AuthSnapshot 是登录方式（外部身份源）的读视图（凭据脱敏）。
type AuthSnapshot struct {
	Local        LocalAuthSnapshot    `json:"local"`
	GitHub       AuthProviderSnapshot `json:"github"`
	WeChat       AuthProviderSnapshot `json:"wechat"`
	GenericOAuth AuthProviderSnapshot `json:"genericoauth"`
	LDAP         AuthProviderSnapshot `json:"ldap"`
	OIDC         AuthProviderSnapshot `json:"oidc"`
	Register     AuthProviderSnapshot `json:"register"` // 自助注册（#51b）：enabled + defaultRoles
	Email        EmailPolicySnapshot  `json:"email"`    // 注册邮箱策略（#51c）
}

// EmailPolicySnapshot 是注册邮箱策略（OPEN-ISSUES #51c）的读视图：
// 域白名单空串 = 不限；别名限制默认关。语义在 auth service.Register
// 注册链路执行。
type EmailPolicySnapshot struct {
	DomainWhitelist  string `json:"domainWhitelist"`
	AliasRestriction bool   `json:"aliasRestriction"`
	// VerificationRequired：注册后须邮箱验证才能登录（令牌链路见
	// auth service #51c 第二批）。
	VerificationRequired bool              `json:"verificationRequired"`
	Sources              map[string]string `json:"sources"`
}

// EmailPolicy resolves the register email policy.
func (l *Layered) EmailPolicy() EmailPolicySnapshot {
	snap := EmailPolicySnapshot{Sources: map[string]string{}}
	v, src, ok := l.GetString(context.Background(), KeyAuthEmailDomainWhitelist)
	if ok {
		snap.DomainWhitelist = v
		snap.Sources[KeyAuthEmailDomainWhitelist] = src
	} else {
		snap.Sources[KeyAuthEmailDomainWhitelist] = "default"
	}
	snap.AliasRestriction, snap.Sources[KeyAuthEmailAliasRestriction], _ = l.getBoolWithSource(KeyAuthEmailAliasRestriction, false)
	snap.VerificationRequired, snap.Sources[KeyAuthEmailVerificationRequired], _ = l.getBoolWithSource(KeyAuthEmailVerificationRequired, false)
	return snap
}

// LocalAuthSnapshot 账号密码登录开关读视图（默认启用，显式覆盖 false 才停用；
// 停用前置约束见 auth.RefreshIdentityProviders 防锁死守卫）。
type LocalAuthSnapshot struct {
	Enabled    bool   `json:"enabled"`
	Overridden bool   `json:"overridden"`
	Source     string `json:"source,omitempty"` // database/config
}

// AuthProviderSnapshot 单一身份源的生效配置（secret 只回 set+尾4）。
type AuthProviderSnapshot struct {
	Enabled      bool              `json:"enabled"`
	Fields       map[string]string `json:"fields"`       // 非凭据字段的生效值
	SecretSet    bool              `json:"secretSet"`    // 凭据是否已设置
	SecretMasked string            `json:"secretMasked"` // ****+尾4
	Sources      map[string]string `json:"sources"`      // 每键来源（database/yaml/default）
}

// AuthSnapshot resolves the identity provider settings (masked).
func (l *Layered) AuthSnapshot() AuthSnapshot {
	local := LocalAuthSnapshot{Enabled: true} // L1 默认启用
	if v, src, ok := l.getBoolWithSource(KeyAuthLocalEnabled, true); ok {
		local.Overridden = true
		local.Source = src
		local.Enabled = v
	}
	return AuthSnapshot{
		Local:        local,
		GitHub:       l.authProviderSnapshot("github"),
		WeChat:       l.authProviderSnapshot("wechat"),
		GenericOAuth: l.authProviderSnapshot("genericoauth"),
		LDAP:         l.authProviderSnapshot("ldap"),
		OIDC:         l.authProviderSnapshot("oidc"),
		Register:     l.authProviderSnapshot("register"),
		Email:        l.EmailPolicy(),
	}
}

// SecurityPolicySnapshot 是账号安全策略的读视图（登录/改密/建号校验链
// 与设置页共用；全零值 = 关闭，维持内置基线；唯一例外 ApprovalStepUpOTP
// 默认 true——审批 step-up 是 #75 落地的强制门槛，缺省安全姿态不放宽，
// #61 开关只用于「出问题时一键降级」）。
type SecurityPolicySnapshot struct {
	MFARequired              bool `json:"mfaRequired"`
	PasswordMinLength        int  `json:"passwordMinLength"`
	PasswordRequireUppercase bool `json:"passwordRequireUppercase"`
	PasswordRequireSpecial   bool `json:"passwordRequireSpecial"`
	PasswordMaxAgeDays       int  `json:"passwordMaxAgeDays"`
	ApprovalStepUpOTP        bool `json:"approvalStepUpOtp"`
}

// SecurityPolicy resolves the account security policy (defaults off;
// ApprovalStepUpOTP defaults on).
func (l *Layered) SecurityPolicy() SecurityPolicySnapshot {
	return SecurityPolicySnapshot{
		MFARequired:              l.GetBool(KeySecurityMFARequired, false),
		PasswordMinLength:        l.GetInt(KeySecurityPasswordMinLength, 0),
		PasswordRequireUppercase: l.GetBool(KeySecurityPasswordRequireUpper, false),
		PasswordRequireSpecial:   l.GetBool(KeySecurityPasswordRequireSpecial, false),
		PasswordMaxAgeDays:       l.GetInt(KeySecurityPasswordMaxAgeDays, 0),
		ApprovalStepUpOTP:        l.GetBool(KeySecurityApprovalStepUpOTP, true),
	}
}

func (l *Layered) authProviderSnapshot(kind string) AuthProviderSnapshot {
	ctx := context.Background()
	prefix := "auth." + kind + "."
	snap := AuthProviderSnapshot{Fields: map[string]string{}, Sources: map[string]string{}}
	snap.Enabled = l.GetBool(settingsKey(prefix+"enabled"), false)
	for _, f := range []string{"addr", "baseDn", "bindDn", "userFilter", "issuer", "clientId", "redirectUrl", "defaultRoles", "startTls", "successUrl", "appId", "authUrl", "tokenUrl", "userInfoUrl", "scopes", "usernameField", "nicknameField", "emailField"} {
		if v, src, ok := l.GetString(ctx, settingsKey(prefix+f)); ok && v != "" {
			snap.Fields[f] = v
			snap.Sources[f] = src
		}
	}
	// startTls 是 bool，放进 Fields 的字符串视图
	if l.GetBool(settingsKey(prefix+"startTls"), false) {
		snap.Fields["startTls"] = "true"
	}
	var secretKey string
	switch kind {
	case "ldap":
		secretKey = KeyAuthLdapBindPassword
	case "github":
		secretKey = KeyAuthGitHubClientSecret
	case "wechat":
		secretKey = KeyAuthWeChatAppSecret
	case "genericoauth":
		secretKey = KeyAuthGenericOAuthClientSecret
	default:
		secretKey = KeyAuthOidcClientSecret
	}
	if v, src, ok := l.GetString(ctx, secretKey); ok && v != "" {
		snap.SecretSet = true
		snap.Sources["secret"] = src
		if len(v) > 4 {
			snap.SecretMasked = "****" + v[len(v)-4:]
		} else {
			snap.SecretMasked = "****"
		}
	}
	return snap
}

// AuthProviderConfig 从分层配置解析出当前生效的外部身份源配置
// （L3 DB 覆盖 yaml；供登录方式热刷新与 Test Connection 使用）。
func (l *Layered) AuthProviderConfig() config.AuthProvidersConfig {
	ctx := context.Background()
	str := func(key string) string {
		v, _, ok := l.GetString(ctx, key)
		if !ok {
			return ""
		}
		return v
	}
	// secret 与 str 同源，但值内 vault:// 引用经密码箱展开（todo 4251）。
	secret := func(key string) string { return l.secretStr(ctx, key) }
	roles := func(key string) []string {
		raw := str(key)
		if raw == "" {
			return nil
		}
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return config.AuthProvidersConfig{
		Local: config.LocalProviderConfig{
			// 默认启用（L3 未覆盖时 true）；显式 false 才停用账号密码登录
			Enabled: boolPtr(l.GetBool(KeyAuthLocalEnabled, true)),
		},
		LDAP: config.LDAPProviderConfig{
			Enabled:        l.GetBool(KeyAuthLdapEnabled, false),
			Addr:           str(KeyAuthLdapAddr),
			BaseDN:         str(KeyAuthLdapBaseDn),
			BindDN:         str(KeyAuthLdapBindDn),
			BindPassword:   secret(KeyAuthLdapBindPassword),
			UserFilter:     str(KeyAuthLdapUserFilter),
			StartTLS:       l.GetBool(KeyAuthLdapStartTLS, false),
			UserDNTemplate: str(KeyAuthLdapUserFilter), // 兼容：filter 未配时回退模板语义由 build 侧处理
			DefaultRoles:   roles(KeyAuthLdapDefaultRoles),
		},
		OIDC: config.OIDCProviderConfig{
			Enabled:      l.GetBool(KeyAuthOidcEnabled, false),
			Issuer:       str(KeyAuthOidcIssuer),
			ClientID:     str(KeyAuthOidcClientId),
			ClientSecret: secret(KeyAuthOidcClientSecret),
			RedirectURL:  str(KeyAuthOidcRedirectUrl),
			DefaultRoles: roles(KeyAuthOidcDefaultRoles),
		},
		GitHub: config.GitHubProviderConfig{
			Enabled:         l.GetBool(KeyAuthGitHubEnabled, false),
			ClientID:        str(KeyAuthGitHubClientId),
			ClientSecret:    secret(KeyAuthGitHubClientSecret),
			RedirectURL:     str(KeyAuthGitHubRedirectUrl),
			DefaultRoles:    roles(KeyAuthGitHubDefaultRoles),
			LoginSuccessURL: str(KeyAuthGitHubSuccessURL),
		},
		WeChat: config.WeChatProviderConfig{
			Enabled:         l.GetBool(KeyAuthWeChatEnabled, false),
			AppID:           str(KeyAuthWeChatAppId),
			AppSecret:       secret(KeyAuthWeChatAppSecret),
			RedirectURL:     str(KeyAuthWeChatRedirectUrl),
			DefaultRoles:    roles(KeyAuthWeChatDefaultRoles),
			LoginSuccessURL: str(KeyAuthWeChatSuccessURL),
		},
		GenericOAuth: config.GenericOAuthProviderConfig{
			Enabled:       l.GetBool(KeyAuthGenericOAuthEnabled, false),
			ClientID:      str(KeyAuthGenericOAuthClientId),
			ClientSecret:  secret(KeyAuthGenericOAuthClientSecret),
			RedirectURL:   str(KeyAuthGenericOAuthRedirectUrl),
			AuthURL:       str(KeyAuthGenericOAuthAuthUrl),
			TokenURL:      str(KeyAuthGenericOAuthTokenUrl),
			UserInfoURL:   str(KeyAuthGenericOAuthUserInfoUrl),
			Scopes:        str(KeyAuthGenericOAuthScopes),
			UsernameField: str(KeyAuthGenericOAuthUsernameField),
			NicknameField: str(KeyAuthGenericOAuthNicknameField),
			EmailField:    str(KeyAuthGenericOAuthEmailField),
			DefaultRoles:  roles(KeyAuthGenericOAuthDefaultRoles),
		},
		Register: config.RegisterConfig{
			Enabled:      l.GetBool(KeyAuthRegisterEnabled, false),
			DefaultRoles: roles(KeyAuthRegisterDefaultRoles),
		},
	}
}
