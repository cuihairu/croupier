// Package session devops-agent 会话通道（业务插件位，非 source Provider，
// agent-core 简档 §7.5）。协议三件套 SSH/RDP/VNC 闭集；会话建立链顺序不可倒：
// capability 门禁（面板侧，另批）→ 白名单校验 → 审计落库（audit-first）→
// Open，任一步失败即拒绝。SSH 通道为本包真实实现（x/crypto/ssh：pty+shell、
// 窗口调整、三向数据流）；RDP/VNC（guacd 通道）另批，Open 显式拒绝不伪装。
// 会话内容回放 = asciicast v2 存档（Hub 落盘），起止元数据走 sessions.jsonl。
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

// SessionRequest 会话建立请求（谁、连哪台、走什么协议、初始窗口多大）。
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
	// Cols/Rows 初始 pty 窗口（<=0 取 DefaultCols/DefaultRows；合法性由
	// ValidateWindow 把关，越界拒绝）。
	Cols int
	Rows int
}

// Session 一条已建立的会话。
type Session interface {
	ID() string
	Close() error
}

// Target 协议 Provider（业务插件位）：一个协议通道一个实现（guacd 承载
// RDP/VNC/SSH-web、本地 SSH 兜底等），Open 前的白名单/审计由 Broker 统一
// 前置，实现方只管协议本身。
type Target interface {
	// Name 渠道名（guacd / local-ssh / ssh，审计与日志可辨）。
	Name() string
	Open(ctx context.Context, req SessionRequest) (Session, error)
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
