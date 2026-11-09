package config

import (
	"time"

	"gopkg.in/yaml.v3"
)

type AuthConfig struct {
	JWTSecret   string `json:"jwtSecret,omitempty" yaml:"jwtSecret,omitempty"`
	RBACConfig  string `json:"rbacConfig,omitempty" yaml:"rbacConfig,omitempty"`
	UsersConfig string `json:"usersConfig,omitempty" yaml:"usersConfig,omitempty"`
	GamesConfig string `json:"gamesConfig,omitempty" yaml:"gamesConfig,omitempty"`
	// LoginLockout 配置本地账号连续密码失败后的临时锁定策略。
	// 仅对 local provider 生效；LDAP/OIDC 的失败计数由外部身份源负责。
	LoginLockout LoginLockoutConfig `json:"loginLockout,omitempty" yaml:"loginLockout,omitempty"`
	// Providers 配置外部身份源（LDAP/OIDC）。默认全部关闭，仅本地账号登录。
	Providers AuthProvidersConfig `json:"providers,omitempty" yaml:"providers,omitempty"`
}

// LoginLockoutConfig 描述本地账号登录失败锁定策略。
// 零值（未配置）时使用默认值：阈值 5 次、锁定 15 分钟。
type LoginLockoutConfig struct {
	// Threshold 触发锁定的连续失败次数；<=0 时使用默认值 5。
	Threshold int `json:"threshold,omitempty" yaml:"threshold,omitempty"`
	// LockMinutes 锁定时长（分钟）；<=0 时使用默认值 15。
	LockMinutes int `json:"lockMinutes,omitempty" yaml:"lockMinutes,omitempty"`
}

// LoginLockoutDefaults 返回应用默认值后的锁定配置。
func (c LoginLockoutConfig) LoginLockoutDefaults() (threshold int, lock time.Duration) {
	threshold = c.Threshold
	if threshold <= 0 {
		threshold = 5
	}
	lock = time.Duration(c.LockMinutes) * time.Minute
	if lock <= 0 {
		lock = 15 * time.Minute
	}
	return threshold, lock
}

// AuthProvidersConfig 汇总外部身份提供方配置。
type AuthProvidersConfig struct {
	Local        LocalProviderConfig        `json:"local,omitempty" yaml:"local,omitempty"`
	LDAP         LDAPProviderConfig         `json:"ldap,omitempty" yaml:"ldap,omitempty"`
	OIDC         OIDCProviderConfig         `json:"oidc,omitempty" yaml:"oidc,omitempty"`
	GitHub       GitHubProviderConfig       `json:"github,omitempty" yaml:"github,omitempty"`
	WeChat       WeChatProviderConfig       `json:"wechat,omitempty" yaml:"wechat,omitempty"`
	GenericOAuth GenericOAuthProviderConfig `json:"genericoauth,omitempty" yaml:"genericoauth,omitempty"`
	Register     RegisterConfig             `json:"register,omitempty" yaml:"register,omitempty"`
}

// RegisterConfig 描述自助注册（OPEN-ISSUES #51b）：默认关闭；开启后匿名
// POST /api/v1/auth/register 可建本地账号并赋予 DefaultRoles（缺省 viewer）。
type RegisterConfig struct {
	Enabled      bool     `json:"enabled" yaml:"enabled"`
	DefaultRoles []string `json:"defaultRoles,omitempty" yaml:"defaultRoles,omitempty"`
}

// LDAPProviderConfig 描述如何连接并校验 LDAP 目录。
type LDAPProviderConfig struct {
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	Addr               string `json:"addr" yaml:"addr"`
	BaseDN             string `json:"baseDn" yaml:"baseDn"`
	BindDN             string `json:"bindDn,omitempty" yaml:"bindDn,omitempty"`
	BindPassword       string `json:"bindPassword,omitempty" yaml:"bindPassword,omitempty"`
	UserFilter         string `json:"userFilter,omitempty" yaml:"userFilter,omitempty"`
	UserDNTemplate     string `json:"userDnTemplate,omitempty" yaml:"userDnTemplate,omitempty"`
	StartTLS           bool   `json:"startTls,omitempty" yaml:"startTls,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	// DefaultRoles 是 JIT 建号时赋予的本地角色名列表。
	DefaultRoles []string `json:"defaultRoles,omitempty" yaml:"defaultRoles,omitempty"`
}

// OIDCProviderConfig 描述 OIDC 客户端与用户映射。
type OIDCProviderConfig struct {
	Enabled       bool     `json:"enabled" yaml:"enabled"`
	Issuer        string   `json:"issuer" yaml:"issuer"`
	ClientID      string   `json:"clientId" yaml:"clientId"`
	ClientSecret  string   `json:"clientSecret" yaml:"clientSecret"`
	RedirectURL   string   `json:"redirectUrl" yaml:"redirectUrl"`
	Scopes        []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	UsernameClaim string   `json:"usernameClaim,omitempty" yaml:"usernameClaim,omitempty"`
	// DefaultRoles 是 JIT 建号时赋予的本地角色名列表。
	DefaultRoles []string `json:"defaultRoles,omitempty" yaml:"defaultRoles,omitempty"`
	// LoginSuccessURL 是回调成功后携带 token 跳转的前端地址；
	// 留空则回调直接返回 JSON（供脚本/API 场景使用）。
	LoginSuccessURL string `json:"loginSuccessUrl,omitempty" yaml:"loginSuccessUrl,omitempty"`
}

// LocalProviderConfig 描述内置账号密码（admins 表）登录开关。
// Enabled 默认启用（nil 视为 true）：显式 false 才停用——停用后本地
// 账号密码级联被移除，登录须走 LDAP/OIDC/GitHub 等外部身份源；
// 全部登录方式关闭会被保存端拒绝（防锁死）。
type LocalProviderConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// LocalEnabled 归一化 nil → true（默认启用）。
func (l LocalProviderConfig) LocalEnabled() bool {
	return l.Enabled == nil || *l.Enabled
}

// GitHubProviderConfig 描述 GitHub OAuth2 App 登录（标准 authorization
// code flow，经 /user + /user/emails 换取身份，JIT 建号同 OIDC 语义）。
type GitHubProviderConfig struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	ClientID     string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	// DefaultRoles 是 JIT 建号时赋予的本地角色名列表。
	DefaultRoles []string `json:"defaultRoles,omitempty" yaml:"defaultRoles,omitempty"`
	// LoginSuccessURL 语义同 OIDC。
	LoginSuccessURL string `json:"loginSuccessUrl,omitempty" yaml:"loginSuccessUrl,omitempty"`
}

// WeChatProviderConfig 描述微信开放平台「网站应用扫码登录」（qrconnect
// 授权码流程，openid 为本地影子账号主键；微信不提供邮箱）。
type WeChatProviderConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	AppID     string `json:"appId,omitempty" yaml:"appId,omitempty"`
	AppSecret string `json:"appSecret,omitempty" yaml:"appSecret,omitempty"`
	// RedirectURL 须与开放平台「授权回调域」一致。
	RedirectURL string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	// DefaultRoles 是 JIT 建号时赋予的本地角色名列表。
	DefaultRoles []string `json:"defaultRoles,omitempty" yaml:"defaultRoles,omitempty"`
	// LoginSuccessURL 语义同 OIDC。
	LoginSuccessURL string `json:"loginSuccessUrl,omitempty" yaml:"loginSuccessUrl,omitempty"`
}

// GenericOAuthProviderConfig 描述自定义 OAuth2 身份源（标准授权码流程，
// 适配 Keycloak/Authentik/企业内网 SSO 等，身份字段经 UserInfo 属性映射）。
type GenericOAuthProviderConfig struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	ClientID     string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	// AuthURL / TokenURL / UserInfoURL 是身份源三端点（均必填）。
	AuthURL     string `json:"authUrl,omitempty" yaml:"authUrl,omitempty"`
	TokenURL    string `json:"tokenUrl,omitempty" yaml:"tokenUrl,omitempty"`
	UserInfoURL string `json:"userInfoUrl,omitempty" yaml:"userInfoUrl,omitempty"`
	// Scopes 是授权范围（逗号分隔配置，构建时切分）。
	Scopes string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	// UsernameField/NicknameField/EmailField 是 UserInfo JSON 顶层属性名
	// （默认 username/name/email；嵌套属性属 IdP 侧映射职责）。
	UsernameField string `json:"usernameField,omitempty" yaml:"usernameField,omitempty"`
	NicknameField string `json:"nicknameField,omitempty" yaml:"nicknameField,omitempty"`
	EmailField    string `json:"emailField,omitempty" yaml:"emailField,omitempty"`
	// DefaultRoles / LoginSuccessURL 语义同 OIDC。
	DefaultRoles    []string `json:"defaultRoles,omitempty" yaml:"defaultRoles,omitempty"`
	LoginSuccessURL string   `json:"loginSuccessUrl,omitempty" yaml:"loginSuccessUrl,omitempty"`
}

func (c *AuthConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain AuthConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		JWTSecret   string `yaml:"JWTSecret,omitempty"`
		RBACConfig  string `yaml:"RBACConfig,omitempty"`
		UsersConfig string `yaml:"UsersConfig,omitempty"`
		GamesConfig string `yaml:"GamesConfig,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.JWTSecret == "" {
		decoded.JWTSecret = compat.JWTSecret
	}
	if decoded.RBACConfig == "" {
		decoded.RBACConfig = compat.RBACConfig
	}
	if decoded.UsersConfig == "" {
		decoded.UsersConfig = compat.UsersConfig
	}
	if decoded.GamesConfig == "" {
		decoded.GamesConfig = compat.GamesConfig
	}
	*c = AuthConfig(decoded)
	return nil
}
