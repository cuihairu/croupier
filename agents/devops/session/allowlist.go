package session

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/cuihairu/croupier/agents/devops/config"
)

// Allowlist 白名单（§7.6：能操作但仅限白名单主机/端口；白名单外一律拒绝）。
// 精确匹配语义：主机大小写不敏感精确等值、端口/协议须显式列出——不做 CIDR、
// 通配与隐式默认放行。
//
// 热更语义：快照原子整体替换（Swap，configsync 下发 apply 走这里）——Check
// 永远读到完整一致的一份名单，不撕裂；非法名单在 apply 侧已校验拒绝，
// keep last-good，本类型不做二次校验。
type Allowlist struct {
	snap atomic.Pointer[allowSnapshot]
}

// allowSnapshot 一份不可变名单快照（构造后只读）。
type allowSnapshot struct {
	entries []config.AllowHost
	byHost  map[string][]config.AllowHost
}

// NewAllowlist 装配白名单（entries 须已经 config.Validate 校验：host/ports/
// protocols 非空、端口合法）。
func NewAllowlist(entries []config.AllowHost) *Allowlist {
	a := &Allowlist{}
	a.snap.Store(buildSnapshot(entries))
	return a
}

func buildSnapshot(entries []config.AllowHost) *allowSnapshot {
	s := &allowSnapshot{entries: entries, byHost: make(map[string][]config.AllowHost, len(entries))}
	for _, e := range entries {
		key := strings.ToLower(strings.TrimSpace(e.Host))
		s.byHost[key] = append(s.byHost[key], e)
	}
	return s
}

// Swap 原子整体替换名单（configsync 下发热生效；当前名单大小返回给日志）。
func (a *Allowlist) Swap(entries []config.AllowHost) (prev, cur int) {
	old := a.snap.Load()
	a.snap.Store(buildSnapshot(entries))
	return len(old.entries), len(entries)
}

// Len 当前名单条目数。
func (a *Allowlist) Len() int {
	return len(a.snap.Load().entries)
}

// Check 校验 (host, port, protocol) 是否在白名单内；不在即拒绝（错误信息
// 点名原因，可进审计 summary）。
func (a *Allowlist) Check(host string, port int, p Protocol) error {
	snap := a.snap.Load()
	key := strings.ToLower(strings.TrimSpace(host))
	entries, ok := snap.byHost[key]
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
