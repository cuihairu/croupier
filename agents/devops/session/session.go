// Package session devops-agent 会话通道（业务插件位，非 source Provider，
// agent-core 简档 §7.5）。协议三件套 SSH/RDP/VNC 闭集；会话建立链顺序不可倒：
// capability 门禁（面板侧，另批）→ 白名单校验 → 审计落库（audit-first）→
// Open，任一步失败即拒绝。协议实现（guacd 通道/SSH 终端）另批，骨架交付
// Provider 接口位 + 会话存档 + audit-first 放行闸的会话级应用。
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// Protocol 会话协议（三协议闭集，§7.2）。
type Protocol string

const (
	ProtocolSSH Protocol = "ssh"
	ProtocolRDP Protocol = "rdp"
	ProtocolVNC Protocol = "vnc"
)

// Valid 协议是否在闭集内。
func (p Protocol) Valid() bool {
	switch p {
	case ProtocolSSH, ProtocolRDP, ProtocolVNC:
		return true
	}
	return false
}

// SessionRequest 会话建立请求（谁、连哪台、走什么协议）。
type SessionRequest struct {
	// SessionID 会话唯一 id（空则由 Broker 生成）。
	SessionID string
	// Protocol 会话协议（闭集）。
	Protocol Protocol
	// Host 目标主机（白名单精确匹配）。
	Host string
	// Port 目标端口（白名单显式列出）。
	Port int
	// Operator 操作者身份（审计必填——「谁」是堡垒机留痕的第一要素）。
	Operator string
}

// Session 一条已建立的会话（协议实现另批；骨架期仅 Close 语义）。
type Session interface {
	ID() string
	Close() error
}

// Target 协议 Provider（业务插件位）：一个协议通道一个实现（guacd 承载
// RDP/VNC/SSH-web、本地 SSH 兜底等），Open 前的白名单/审计由 Broker 统一
// 前置，实现方只管协议本身。
type Target interface {
	// Name 渠道名（guacd / local-ssh / ...，审计与日志可辨）。
	Name() string
	Open(ctx context.Context, req SessionRequest) (Session, error)
}

// noopTarget 骨架占位 Provider：协议通道另批实施，Open 一律拒绝——白名单
// 与审计链先行落地，协议到位即换实现，建立链零改动。
type noopTarget struct{}

// NewNoopTarget 骨架占位 Provider。
func NewNoopTarget() Target { return noopTarget{} }

func (noopTarget) Name() string { return "noop" }

func (noopTarget) Open(_ context.Context, req SessionRequest) (Session, error) {
	return nil, fmt.Errorf("session channel %s not implemented yet (skeleton)", req.Protocol)
}

// newSessionID 生成会话 id（16 hex，crypto/rand）。
func newSessionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 失败极罕见（系统熵源故障）；拒绝生成比可预测 id 安全。
		panic(fmt.Errorf("session: generate id: %w", err))
	}
	return hex.EncodeToString(b[:])
}
