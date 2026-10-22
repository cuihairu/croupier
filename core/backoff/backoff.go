// Package backoff 统一重试退避旋钮（矩阵 A 批拍板：cenkalti/backoff/v5）。
// 只收拢用法不另造实现：各调用点从「各自 NewExponentialBackOff 再逐字段
// 赋值」收敛到统一构造器，杜绝同一仓库出现多套退避写法。
package backoff

import (
	"context"
	"time"

	v5 "github.com/cenkalti/backoff/v5"
)

// ExponentialBackOff 与 cenkalti 实现同名别名，调用方注解返回类型无需引底层包。
type ExponentialBackOff = v5.ExponentialBackOff

// BackOff 与 cenkalti 接口同名别名。
type BackOff = v5.BackOff

// Stop 退避终止信号（v5 语义：NextBackOff 返回 Stop 表示放弃）。
const Stop = v5.Stop

// Exponential 返回指数退避：initial 起步、max 封顶、multiplier 倍率；
// multiplier<=0 时沿用 cenkalti 默认（1.5）。Reset 后即可驱动 NextBackOff。
// v5 默认带随机化抖动（0.5），重连场景天然防惊群。
func Exponential(initial, max time.Duration, multiplier float64) *ExponentialBackOff {
	b := v5.NewExponentialBackOff()
	b.InitialInterval = initial
	b.MaxInterval = max
	if multiplier > 0 {
		b.Multiplier = multiplier
	}
	b.Reset()
	return b
}

// Constant 返回固定间隔退避。
func Constant(interval time.Duration) v5.BackOff {
	return v5.NewConstantBackOff(interval)
}

// Sleep 等待退避时长 d 或 ctx 取消；返回 false 表示 ctx 已取消，调用方应
// 立即退出（time.Sleep 不响应取消，等待期间的 ctx 校验只能等下一轮）。
// d<=0（backoff.Stop 或零值）不等待，仅返回 ctx 是否存活。
func Sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
