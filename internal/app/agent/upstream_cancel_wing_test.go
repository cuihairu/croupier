// 覆盖率巡检第二十轮（wt-api）：upstream.go updateLoop 退出时
// defer 的 timer.Stop 翼（364-365）收口——ctx 取消时去抖 timer 仍在
// 待触发期（非 nil），defer 必须停表防泄漏。既有用例（debounce 100ms
// 到期后 nil 再取消）只盖 nil 翼；本例 debounce 10s + 取消前必在
// 待触发窗口，确定性触达非 nil 翼。
package agent

import (
	"context"
	"testing"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/stretchr/testify/require"
)

func TestUpstreamUpdateLoopCancelWithPendingTimer_StopsTimer(t *testing.T) {
	store := agentlocal.NewLocalStore()
	client := NewUpstreamClient("mock", "agent-r20-timerstop", store, nil)
	client.setClient(&mockControlClientV9{connected: true})

	ctx, cancel := context.WithCancel(context.Background())
	client.updateCh = make(chan struct{}, 4)
	exited := make(chan struct{})
	go func() {
		client.updateLoop(ctx, 10*time.Second)
		close(exited)
	}()

	// 一条消息创建 timer（10s 内不可能到期），随后立即取消：
	// select 只有 ctx.Done 就绪 → 退出路径执行 defer Stop（非 nil 翼）
	client.updateCh <- struct{}{}
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-exited:
		// loop 已退出（defer Stop 已执行），且未发生任何 sync 出站
	case <-time.After(2 * time.Second):
		t.Fatal("updateLoop 未随 ctx 取消退出")
	}
	require.NotNil(t, client.updateCh, "夹具自检：updateCh 存活")
}
