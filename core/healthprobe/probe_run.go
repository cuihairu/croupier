package healthprobe

import (
	"context"
	"log/slog"
	"time"
)

// ProbeOnce 执行一次探测并推进状态机；窗口开/合在此产生（事件回调在解锁后
// 触发，listener 内再调 Probe 的查询方法不会死锁）。
func (p *Probe) ProbeOnce(ctx context.Context) Result {
	err := p.check.Check(ctx)
	now := time.Now()

	p.mu.Lock()
	var events []WindowEvent
	if err != nil {
		p.failStreak++
		p.okStreak = 0
		if p.available && p.failStreak >= p.failThr {
			p.available = false
			p.current = &Window{
				Target:     p.target,
				Semantics:  string(p.sem),
				StartTS:    now.UnixMilli(),
				LastReason: err.Error(),
				GameID:     p.scopeGame,
				Env:        p.scopeEnv,
			}
			events = append(events, WindowEvent{Kind: EventUnavailable, Window: *p.current})
		} else if p.current != nil {
			p.current.LastReason = err.Error()
		}
	} else {
		p.okStreak++
		p.failStreak = 0
		if !p.available && p.current != nil && p.okStreak >= p.okThr {
			w := p.current
			w.EndTS = now.UnixMilli()
			w.DurationMS = w.EndTS - w.StartTS
			p.appendHistoryLocked(*w)
			p.current = nil
			p.available = true
			events = append(events, WindowEvent{Kind: EventRecovered, Window: *w})
		}
	}
	res := Result{
		At:                  now,
		Available:           p.available,
		WindowOpen:          p.current != nil,
		ConsecutiveFailures: p.failStreak,
	}
	if err != nil {
		res.Err = err.Error()
	}
	p.mu.Unlock()

	if p.listener != nil {
		for _, ev := range events {
			p.listener(ev)
		}
	}
	if p.timeline != nil {
		for _, ev := range events {
			if terr := p.timeline.AppendWindow(ev); terr != nil {
				slog.Warn("healthprobe timeline append failed", "target", p.target, "error", terr)
			}
		}
	}
	return res
}

// Run 周期探测：启动即探一次，随后按 interval 驱动，ctx 取消退出。
func (p *Probe) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	p.ProbeOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.ProbeOnce(ctx)
		}
	}
}

// Available 当下是否可用（两态机结论，非单次探测结果）。
func (p *Probe) Available() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.available
}

// CurrentWindow 返回进行中故障窗口（无则 false）。
func (p *Probe) CurrentWindow() (Window, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return Window{}, false
	}
	return *p.current, true
}

// History 返回已闭合窗口的快照（副本，时间倒序不保证——按闭合顺序追加）。
func (p *Probe) History() []Window {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Window, len(p.history))
	copy(out, p.history)
	return out
}

// appendHistoryLocked 追加历史并按 maxHistory 截断（调用方持锁）。
func (p *Probe) appendHistoryLocked(w Window) {
	p.history = append(p.history, w)
	if len(p.history) > p.maxHistory {
		p.history = p.history[len(p.history)-p.maxHistory:]
	}
}
