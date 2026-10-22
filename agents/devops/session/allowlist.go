package session

import (
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/agents/devops/config"
)

// Allowlist 白名单（§7.6：能操作但仅限白名单主机/端口；白名单外一律拒绝）。
// 精确匹配语义：主机大小写不敏感精确等值、端口/协议须显式列出——不做 CIDR、
// 通配与隐式默认放行（下发走 configsync 另批，文件配置起步）。
type Allowlist struct {
	entries []config.AllowHost
	byHost  map[string][]config.AllowHost
}

// NewAllowlist 装配白名单（entries 须已经 config.Validate 校验：host/ports/
// protocols 非空、端口合法）。
func NewAllowlist(entries []config.AllowHost) *Allowlist {
	a := &Allowlist{entries: entries, byHost: make(map[string][]config.AllowHost, len(entries))}
	for _, e := range entries {
		key := strings.ToLower(strings.TrimSpace(e.Host))
		a.byHost[key] = append(a.byHost[key], e)
	}
	return a
}

// Check 校验 (host, port, protocol) 是否在白名单内；不在即拒绝（错误信息
// 点名原因，可进审计 summary）。
func (a *Allowlist) Check(host string, port int, p Protocol) error {
	key := strings.ToLower(strings.TrimSpace(host))
	entries, ok := a.byHost[key]
	if !ok {
		return fmt.Errorf("host %q not in allowlist", host)
	}
	for _, e := range entries {
		portOK := false
		for _, allowed := range e.Ports {
			if allowed == port {
				portOK = true
				break
			}
		}
		if !portOK {
			continue
		}
		for _, proto := range e.Protocols {
			if Protocol(strings.ToLower(strings.TrimSpace(proto))) == p {
				return nil
			}
		}
	}
	// 主机在名单但端口/协议不匹配：区分端口与协议给可读错误。
	for _, e := range entries {
		for _, allowed := range e.Ports {
			if allowed == port {
				return fmt.Errorf("protocol %s not allowed for %s:%d", p, host, port)
			}
		}
	}
	return fmt.Errorf("port %d not allowed for %s", port, host)
}
