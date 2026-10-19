package healthprobe

import (
	"context"
	"log/slog"
	"time"
)

// ProbeOnce 执行一次探测并推进状态机；窗口开/合在此产生（事件回调在解锁后
// 触发，listener 内再调 Probe 的查询方法不会死锁）。
func (p *Probe) ProbeOnce(ctx context.Context) Result {
	start := time.Now()
	err := p.check.Check(ctx)
	latency := time.Since(start).Milliseconds()
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
				Category:   p.category,
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
		OK:                  err == nil,
		At:                  now,
		Target:              p.target,
		Category:            p.category,
		Available:           p.available,
		WindowOpen:          p.current != nil,
		ConsecutiveFailures: p.failStreak,
		LatencyMS:           latency,
	}
	if err != nil {
		res.Err = err.Error()
	}
	if d, ok := p.check.(Detailing); ok {
		res.Details = d.Details()
	}
	p.mu.Unlock()

	if p.listener != nil {
		for _, ev := range events {
			p.listener(ev)
		}
	}
	if p.resultFn != nil {
		p.resultFn(res)
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
	p.RunDynamic(ctx, func(time.Time) time.Duration { return interval })
}

// RunDynamic 周期探测（动态间隔）：每轮探测后向 intervalFn 取下一间隔——
// 采样策略热更（probe.policy configsync 下拉）与异常态提频经此生效，无需
// 推送通道或 ticker 重建协议（函数读侧自取最新值）。返回 <=0 按
// DefaultInterval。intervalFn 在探测循环 goroutine 内调用（可在其内读探针
// 自身 Available()——ProbeOnce 已解锁，无死锁）。
func (p *Probe) RunDynamic(ctx context.Context, intervalFn func(now time.Time) time.Duration) {
	if intervalFn == nil {
		intervalFn = func(time.Time) time.Duration { return DefaultInterval }
	}
	for {
		p.ProbeOnce(ctx)
		next := intervalFn(time.Now())
		if next <= 0 {
			next = DefaultInterval
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(next):
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
