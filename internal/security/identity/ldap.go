package identity

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// LDAPConfig 是 LDAP 提供方的参数。身份参数与业务配置（默认角色等）
// 分离，本结构只描述"如何连上并校验 LDAP"。
type LDAPConfig struct {
	// Addr 是 LDAP 服务地址，如 "ldap://ldap.example.com:389" 或 "ldaps://host:636"。
	Addr string
	// BaseDN 是用户搜索的基础 DN，如 "dc=example,dc=com"。
	BaseDN string
	// BindDN / BindPassword 是可选的服务账号，用于先搜索再绑定（推荐）。
	// 留空时使用匿名搜索（要求目录允许）或 UserDNTemplate 直连绑定。
	BindDN       string
	BindPassword string
	// UserFilter 是按用户名搜索的过滤器模板，%s 会被替换为转义后的用户名。
	// 默认 "(uid=%s)"。
	UserFilter string
	// UserDNTemplate 是免搜索直连绑定的 DN 模板，如 "uid=%s,ou=people,dc=example,dc=com"。
	// 仅当 BindDN 为空且目录禁止匿名搜索时需要。
	UserDNTemplate string
	// StartTLS 对非 ldaps 连接升级 TLS。
	StartTLS bool
	// InsecureSkipVerify 跳过 TLS 证书校验（仅自签名目录的开发环境使用）。
	InsecureSkipVerify bool
}

// ldapConn 抽象 go-ldap 连接，便于测试替身。
type ldapConn interface {
	Bind(username, password string) error
	Search(searchRequest *ldap.SearchRequest) (*ldap.SearchResult, error)
	StartTLS(config *tls.Config) error
	Close() error
}

// LDAPProvider 基于 LDAP 目录校验用户名/密码。
//
// 认证流程：
//  1. 建立（可选 StartTLS 的）连接
//  2. 若配置了 BindDN：服务账号绑定 → 按 UserFilter 搜索用户 DN → 用用户 DN + 密码绑定
//     若未配置：按 UserDNTemplate 直接拼 DN 绑定（无模板则匿名搜索）
//  3. 绑定成功即认证通过，顺带提取 cn/mail 作为展示属性
type LDAPProvider struct {
	cfg    LDAPConfig
	dial   func(addr string) (ldapConn, error)
	tlsCfg *tls.Config
}

// NewLDAPProvider 创建 LDAP 身份提供方。
func NewLDAPProvider(cfg LDAPConfig) *LDAPProvider {
	if cfg.UserFilter == "" {
		cfg.UserFilter = "(uid=%s)"
	}
	return &LDAPProvider{
		cfg: cfg,
		dial: func(addr string) (ldapConn, error) {
			conn, err := ldap.DialURL(addr)
			if err != nil {
				return nil, err
			}
			return conn, nil
		},
		tlsCfg: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}, // #nosec G402 -- 显式配置项，默认 false
	}
}

// Kind 实现 PasswordProvider。
func (p *LDAPProvider) Kind() string { return KindLDAP }

// Authenticate 实现 PasswordProvider。
func (p *LDAPProvider) Authenticate(ctx context.Context, username, password string) (*Identity, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	conn, err := p.dial(p.cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("ldap dial: %w", err)
	}
	defer conn.Close()

	if p.cfg.StartTLS && !strings.HasPrefix(strings.ToLower(p.cfg.Addr), "ldaps://") {
		if err := conn.StartTLS(p.tlsCfg); err != nil {
			return nil, fmt.Errorf("ldap starttls: %w", err)
		}
	}

	userDN, nickname, email, err := p.locateUser(conn, username)
	if err != nil {
		return nil, err
	}

	// 用户 DN + 密码绑定，绑定成功即凭证有效。
	if err := conn.Bind(userDN, password); err != nil {
		return nil, ErrInvalidCredentials
	}

	ident := &Identity{
		Provider: KindLDAP,
		Username: username,
		Nickname: nickname,
		Email:    email,
	}
	if ident.Nickname == "" {
		ident.Nickname = username
	}
	return ident, nil
}

// locateUser 解析用户 DN。优先服务账号搜索，其次 DN 模板，最后匿名搜索。
func (p *LDAPProvider) locateUser(conn ldapConn, username string) (dn, nickname, email string, err error) {
	if p.cfg.BindDN != "" {
		if err := conn.Bind(p.cfg.BindDN, p.cfg.BindPassword); err != nil {
			return "", "", "", fmt.Errorf("ldap service bind: %w", err)
		}
		return p.searchUser(conn, username)
	}
	if p.cfg.UserDNTemplate != "" {
		return fmt.Sprintf(p.cfg.UserDNTemplate, ldap.EscapeFilter(username)), "", "", nil
	}
	// 匿名搜索（部分目录允许匿名读，但禁止匿名绑定校验）。
	return p.searchUser(conn, username)
}

func (p *LDAPProvider) searchUser(conn ldapConn, username string) (dn, nickname, email string, err error) {
	filter := fmt.Sprintf(p.cfg.UserFilter, ldap.EscapeFilter(username))
	req := ldap.NewSearchRequest(
		p.cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		2, 0, false,
		filter,
		[]string{"dn", "cn", "mail"},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		return "", "", "", fmt.Errorf("ldap search: %w", err)
	}
	if len(res.Entries) == 0 {
		return "", "", "", ErrInvalidCredentials
	}
	if len(res.Entries) > 1 {
		return "", "", "", fmt.Errorf("ldap search: multiple entries for user %q", username)
	}
	entry := res.Entries[0]
	return entry.DN, entry.GetAttributeValue("cn"), entry.GetAttributeValue("mail"), nil
}
