package configsync

import (
	"context"
	"fmt"
	"time"
)

// PullOnce 执行一轮「拉取 → 版本比对 → 热生效」。返回本轮是否实际生效；
// 版本未变返回 (false, nil)。拉取/生效错误向上传播并触发 onError 回调。
func (p *Puller[T]) PullOnce(ctx context.Context) (bool, error) {
	if p == nil || p.fetch == nil || p.apply == nil {
		return false, fmt.Errorf("configsync: puller not initialized (fetch/apply required)")
	}
	v, err := p.fetch(ctx)
	if err != nil {
		p.fail(err)
		return false, err
	}
	if v == nil {
		return false, nil
	}
	if v.Version != "" && v.Version == p.LastVersion() {
		return false, nil
	}
	if err := p.apply(ctx, v.Payload); err != nil {
		p.fail(fmt.Errorf("configsync: apply version %q: %w", v.Version, err))
		return false, err
	}
	p.mu.Lock()
	p.lastVersion = v.Version
	onApply := p.onApply
	p.mu.Unlock()
	if onApply != nil {
		onApply(v.Version)
	}
	return true, nil
}

// Start 启动轮询循环：立即一轮，之后按 interval 周期执行，直到 ctx 取消。
// 循环内单轮失败只回调 onError，不打断轮询。无 fetch/apply 时不启动。
func (p *Puller[T]) Start(ctx context.Context) {
	if p == nil || p.fetch == nil || p.apply == nil {
		return
	}
	go func() {
		_, _ = p.PullOnce(ctx)
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = p.PullOnce(ctx)
			}
		}
	}()
}

func (p *Puller[T]) fail(err error) {
	if p.onError != nil {
		p.onError(err)
	}
}
