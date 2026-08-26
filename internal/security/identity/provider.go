// Package identity 定义认证身份提供方抽象。
//
// 设计意图：登录链路不得把"用户必须来自本地 admins 表"这一假设写死在
// 核心逻辑里。所有凭证校验都必须经过本包的接口，以便接入外部身份源
// （OIDC/LDAP 等）时以新 Provider 实现的形式接入，而不侵入登录流程。
//
// 边界约定：角色与权限始终由本地 RBAC 体系（admins/roles 表）裁决，
// Provider 只负责"证明你是谁"，不负责"你能做什么"。外部身份源登录的
// 用户由登录链路按需 JIT 落库为本地影子账号，再走统一的角色加载。
package identity

import (
	"context"
	"errors"
)

// ProviderKind 是身份提供方的种类标识。
const (
	KindLocal = "local"
	KindLDAP  = "ldap"
	KindOIDC  = "oidc"
)

// ErrInvalidCredentials 表示凭证校验失败（用户不存在或密码错误）。
// 提供方实现必须返回该错误（可用 errors.Is 判定），调用方据此统一
// 返回"用户名或密码错误"，避免泄露用户是否存在。
var ErrInvalidCredentials = errors.New("invalid credentials")

// Identity 是认证通过后的身份主体。Provider 字段标识来源；
// 对 local 提供方，登录链路通常已直接拿到本地 admin 记录，
// 对外部提供方则由登录链路按需 JIT 建号后再解析本地记录。
type Identity struct {
	Provider string
	Username string
	Nickname string
	Email    string
}

// PasswordProvider 是用户名/密码型身份提供方（local、LDAP）。
//
// 实现要求：
//   - 凭证非法时必须返回 ErrInvalidCredentials（或包裹它的错误）
//   - 实现必须是并发安全的
type PasswordProvider interface {
	// Kind 返回提供方种类标识（KindLocal / KindLDAP / ...）。
	Kind() string
	// Authenticate 校验用户名/密码凭证，成功返回 Identity。
	Authenticate(ctx context.Context, username, password string) (*Identity, error)
}

// OAuthProvider 是重定向授权型身份提供方（OIDC）。
// 登录入口生成跳转 URL，回调处用授权码换取身份。
type OAuthProvider interface {
	// Kind 返回提供方种类标识（KindOIDC）。
	Kind() string
	// AuthCodeURL 生成跳转到身份源的授权 URL，state 原样透传。
	AuthCodeURL(state string) string
	// Exchange 用回调携带的授权码换取并验证身份。
	Exchange(ctx context.Context, code string) (*Identity, error)
}
