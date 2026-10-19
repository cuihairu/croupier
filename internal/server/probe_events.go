package server

import (
	"sync"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// 探测事件诊断面是持久化 probe_events 之外的短期现场缓存。
// 窗口事件→Alert 生命周期由 probe_lifecycle 处理；失败明细不触发告警。

// probeEventRingSize 每 agent 诊断环容量（§13.1 显式默认：同 supervisor 事件环
// 先例 registry/metrics.go supervisorEventRingSize=500）。
const probeEventRingSize = 500

// probeDetailRetention 单 agent 明细环保留时长：超出即整环丢弃，防长驻 agent
// 零频目标占内存（明示常量，不隐式）。
const probeDetailRetention = 30 * time.Minute

// probeEventSink 探测事件诊断环（per-agent 有界，重传/乱序不去重——明细是
// 现场事实，重复比缺失好排查）。
type probeEventSink struct {
	mu   sync.Mutex
	ring map[string][]probeEventEntry
	now  func() time.Time
}

type probeEventEntry struct {
	at   time.Time
	body *opsv1.AlertEvent
}

func newProbeEventSink() *probeEventSink {
	return &probeEventSink{ring: make(map[string][]probeEventEntry), now: time.Now}
}

// add 收一条探测事件（agent_id 已校验）。
func (p *probeEventSink) add(agentID string, ev *opsv1.AlertEvent) {
	if p == nil || agentID == "" || ev == nil {
		return
	}
	now := p.clock()
	p.mu.Lock()
	defer p.mu.Unlock()
	entries := append(p.ring[agentID], probeEventEntry{at: now, body: ev})
	if len(entries) > probeEventRingSize {
		entries = entries[len(entries)-probeEventRingSize:]
	}
	p.ring[agentID] = entries
}

// recent 返回该 agent 最近 limit 条（时间正序；limit<=0 取全部）。
func (p *probeEventSink) recent(agentID string, limit int) []*opsv1.AlertEvent {
	if p == nil || agentID == "" {
		return nil
	}
	p.sweep(agentID)
	p.mu.Lock()
	defer p.mu.Unlock()
	entries := p.ring[agentID]
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	out := make([]*opsv1.AlertEvent, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.body)
	}
	return out
}

// sweep 丢弃超过保留期的条目（调用方不持锁）。
func (p *probeEventSink) sweep(agentID string) {
	cutoff := p.clock().Add(-probeDetailRetention)
	p.mu.Lock()
	defer p.mu.Unlock()
	entries := p.ring[agentID]
	keep := 0
	for keep < len(entries) && entries[keep].at.Before(cutoff) {
		keep++
	}
	if keep > 0 {
		p.ring[agentID] = append([]probeEventEntry(nil), entries[keep:]...)
	}
}

// clock 便于测试注入固定时钟。
func (p *probeEventSink) clock() time.Time {
	if p != nil && p.now != nil {
		return p.now()
	}
	return time.Now()
}
