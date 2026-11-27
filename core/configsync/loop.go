package configsync

import (
	"context"
	"fmt"
	"time"
)

// PullOnce 执行一轮「拉取 → 版本比对 → 热生效 → 提交游标」。返回本轮是否实际生效；
// 版本未变返回 (false, nil)。拉取/生效错误向上传播并触发 onError 回调。
//
// 两层防护保证「定时拉取与手动拉取并发时配置与游标一致，较早请求的迟到响应不会
// 覆盖已生效的更新」：
//  1. pullSlot（容量 1 的槽位）把「拉取 → 生效 → 提交游标」整轮串行，任何时刻
//     只有一个拉取在途；
//  2. seq/appliedSeq 是单调请求纪元：一轮在提交前若发现已有更晚发起的拉取提交，
//     跳过本次生效（迟到响应作废），不覆盖新配置。
//
// 等待槽位时不持 mu、不持任何锁，ctx 取消可退出，不会与调用方 context 互锁；
// 回调（onError/onApply）在槽位释放后触发，且不在持锁状态下调用。
func (p *Puller[T]) PullOnce(ctx context.Context) (bool, error) {
	if p == nil || p.fetch == nil || p.apply == nil {
		return false, fmt.Errorf("configsync: puller not initialized (fetch/apply required)")
	}
	if p.pullSlot == nil {
		return false, fmt.Errorf("configsync: puller not initialized (pullSlot missing)")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	// 请求纪元在进入串行区之前分配：seq 单调递增，代表发起顺序。
	seq := p.seq.Add(1)
	select {
	case p.pullSlot <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	applied, version, err := func() (bool, string, error) {
		defer func() { <-p.pullSlot }()
		return p.pullOnceLocked(ctx, seq)
	}()
	if err != nil {
		p.fail(err)
		return false, err
	}
	if !applied {
		return false, nil
	}
	p.mu.Lock()
	onApply := p.onApply
	p.mu.Unlock()
	if onApply != nil {
		onApply(version)
	}
	return true, nil
}

// pullOnceLocked 在持有 pullSlot 时执行一轮：读版本 → 拉取 → 生效 → 提交游标。
// apply 失败时游标不推进，下个 PullOnce 会重试。调用方负责持有槽位。
func (p *Puller[T]) pullOnceLocked(ctx context.Context, seq uint64) (bool, string, error) {
	v, err := p.fetch(ctx)
	if err != nil {
		return false, "", err
	}
	if v == nil {
		return false, "", nil
	}
	// 迟到响应作废：本次请求在途期间已有更晚发起的拉取完成生效，则不覆盖。
	if p.appliedSeq.Load() > seq {
		return false, "", nil
	}
	if v.Version != "" && v.Version == p.LastVersion() {
		return false, "", nil
	}
	if err := p.apply(ctx, v.Payload); err != nil {
		return false, "", fmt.Errorf("configsync: apply version %q: %w", v.Version, err)
	}
	p.mu.Lock()
	p.lastVersion = v.Version
	p.mu.Unlock()
	p.appliedSeq.Store(seq)
	return true, v.Version, nil
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
	p.mu.Lock()
	onError := p.onError
	p.mu.Unlock()
	if onError != nil {
		onError(err)
	}
}
