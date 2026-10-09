// Package report 统一上行（agent-core K3）：任务事件与指标报告的序列化 +
// 单发信封，周期上报循环。core 只收拢「组包→发上行通道」的通用面；连接由
// 调用方以 Conn getter 注入（重连换连后快照自动跟随）。告警通道（wire 新增
// 消息）落位时在本包扩展 SendAlert，v1 不预置死接口。
package report

import (
	"context"
	"fmt"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// Conn 上行连接的最小面（一次性单发消息）；Connected 供发送前断连短路
// （对齐 sidecar 历史语义：断连报「not connected」而非透传传输层错误）。
type Conn interface {
	Connected() bool
	SendTaskEvent(ctx context.Context, body []byte) error
	SendMetricEvent(ctx context.Context, body []byte) error
}

// ConnGetter 返回当前上行连接（重连换连后自动跟随）；无连接返回 nil。
type ConnGetter func() Conn

// Reporter 统一上行信封。
type Reporter struct {
	get ConnGetter
}

// New 构造 Reporter。
func New(get ConnGetter) *Reporter { return &Reporter{get: get} }

// SendMetrics 序列化 MetricsReport 并单发。
func (r *Reporter) SendMetrics(ctx context.Context, report *opsv1.MetricsReport) error {
	if r == nil || r.get == nil {
		return fmt.Errorf("reporter is nil")
	}
	conn := r.get()
	if conn == nil {
		return fmt.Errorf("upstream client not connected")
	}
	if !conn.Connected() {
		return fmt.Errorf("upstream client not connected")
	}
	if report == nil {
		return fmt.Errorf("metrics report is nil")
	}
	data, err := proto.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal metrics report: %w", err)
	}
	return conn.SendMetricEvent(ctx, data)
}

// SendTaskEvent 序列化 TaskEvent 并单发。
func (r *Reporter) SendTaskEvent(ctx context.Context, event *sdkv1.TaskEvent) error {
	if r == nil || r.get == nil {
		return fmt.Errorf("reporter is nil")
	}
	conn := r.get()
	if conn == nil {
		return fmt.Errorf("upstream client not connected")
	}
	if !conn.Connected() {
		return fmt.Errorf("upstream client not connected")
	}
	if event == nil {
		return fmt.Errorf("task event is nil")
	}
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal task event: %w", err)
	}
	return conn.SendTaskEvent(ctx, data)
}

// RunLoop 周期上行循环：启动即执行一次 fn，随后按 interval 驱动，ctx 取消
// 退出；单次失败由 fn 自行决定是否吞掉（周期循环不因单周期失败停止）。
func RunLoop(ctx context.Context, interval time.Duration, fn func(ctx context.Context) error) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	_ = fn(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = fn(ctx)
		}
	}
}
