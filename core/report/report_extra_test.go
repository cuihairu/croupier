package report

// report.go / probe_uplink.go 补测：SendMetrics / SendTaskEvent /
// SendAlertEvent 的断连（Connected=false）与无连接（getter 返回 nil）臂、
// Marshal 失败臂（proto3 string 字段注入非法 UTF-8 触发 proto.Marshal
// 错误）、RunLoop 的 interval<=0 默认档；marshalWindow / marshalDetail 的
// 组包失败 nil 返回臂。
//
// 无诚实跳过：全臂可达（Marshal 失败经非法 UTF-8 注入；组包失败臂经同包
// 直调注入）。

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/core/healthprobe"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

func TestSendDisconnectedAndMarshalFailureArms(t *testing.T) {
	ctx := context.Background()

	// 断连：Connected=false → 短路报 not connected
	conn := &fakeConn{connected: false}
	r := New(func() Conn { return conn })
	require.ErrorContains(t, r.SendMetrics(ctx, &opsv1.MetricsReport{}), "not connected")
	require.ErrorContains(t, r.SendTaskEvent(ctx, &sdkv1.TaskEvent{}), "not connected")
	require.ErrorContains(t, r.SendAlertEvent(ctx, &opsv1.AlertEvent{}), "not connected")

	// 无连接：getter 返回 nil
	none := New(func() Conn { return nil })
	require.ErrorContains(t, none.SendTaskEvent(ctx, &sdkv1.TaskEvent{}), "not connected")
	require.ErrorContains(t, none.SendAlertEvent(ctx, &opsv1.AlertEvent{}), "not connected")

	// Marshal 失败：string 字段非法 UTF-8
	good := &fakeConn{connected: true}
	rm := New(func() Conn { return good })
	require.ErrorContains(t, rm.SendMetrics(ctx, &opsv1.MetricsReport{AgentId: "\xff"}),
		"marshal metrics report")
	require.ErrorContains(t, rm.SendTaskEvent(ctx, &sdkv1.TaskEvent{TaskId: "\xff"}),
		"marshal task event")
	require.ErrorContains(t, rm.SendAlertEvent(ctx, &opsv1.AlertEvent{AgentId: "\xff"}),
		"marshal alert event")
}

func TestRunLoopDefaultInterval(t *testing.T) {
	// interval<=0 → 默认 30s：测试窗内只有启动即执行的一次 fn，无 tick
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(60*time.Millisecond, cancel)
	RunLoop(ctx, 0, func(context.Context) error {
		calls++
		return nil
	})
	require.Equal(t, 1, calls)
}

func TestMarshalUplinkFailureArms(t *testing.T) {
	// cfg.AgentID 非法 UTF-8 → proto.Marshal 失败 → 组包返回 nil
	u := NewProbeUplink(ProbeUplinkConfig{AgentID: "\xff"}, nil)
	require.Nil(t, u.marshalWindow(healthprobe.WindowEvent{Window: healthprobe.Window{Target: "t"}}))
	require.Nil(t, u.marshalDetail(healthprobe.Result{Err: "boom"}))
}
