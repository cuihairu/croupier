// ARCH-017 停机收尾故障钻机：注入卡死的调度器 tick（R30-07 场景——tick 用
// context.Background 驱动库调用，卡死时不观察 stopCh），经真实停机入口
// （SIGINT → runServer 优雅停机序列）核对：
//
//  1. 总截止：停机序列与 runServer 的 defer 兜底都有界返回——卡死调度
//     不拖死进程（旧行为：外层 30s 返回后仍执行无界 defer StopScheduler，
//     最终返回无限等待）。
//  2. 关池顺序：shutdownDone 只在 svcCtx.Close()（冲刷留痕 + 释放池）之后
//     关闭——runServer 正常返回（非「timed out, forcing exit」强制路径）即
//     证明关池已推进执行，而非仅打超时日志就当作资源已释放。
//  3. 资源释放：HTTP 与控制 TCP 监听在返回后不再接受连接（socket 已关）。
//
// 边界诚实：卡死 tick 的 goroutine 在 block 释放前仍存活（随进程退出回收），
// 本钻机不断言其「已终止」，只断言停机入口不被它拖死、关池照常推进。
package main

import (
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	scheduler "github.com/cuihairu/croupier/internal/tasks/scheduler"
	"github.com/stretchr/testify/require"
)

func TestDrill_BlockedScheduler_ShutdownBoundedAndCloseRuns(t *testing.T) {
	// 注入：tick 卡死 + 短扫描周期（首 tick 即卡死）+ 1s Stop 上限
	// （缺省 5s，缩短以压钻机时长；仍验证「超时返回不无限等待」语义）。
	entered := make(chan struct{}, 1)
	block := make(chan struct{})
	oldHook := drillSchedulerHook
	drillSchedulerHook = func(mgr *scheduler.Manager) {
		mgr.SetTickHook(func() {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-block
		})
		mgr.SetStopTimeout(time.Second)
		mgr.SetInterval(5 * time.Millisecond)
	}
	t.Cleanup(func() {
		drillSchedulerHook = oldHook
		close(block) // 释放卡死 tick：循环退出，不跨用例泄漏 goroutine
	})

	s := drillBoot(t)

	// 等待首个 tick 进入卡死点（循环停在 tick 内，不观察 stopCh）。
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("scheduler tick did not enter the blocked hook")
	}

	// 卡死调度下的真实停机入口。
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	start := time.Now()
	require.NoError(t, s.waitExit(t, 30*time.Second),
		"runServer must return cleanly under a blocked scheduler (defer StopScheduler bounded)")
	require.Less(t, time.Since(start), 30*time.Second,
		"shutdown exceeded the outer 30s budget")

	// 资源释放：HTTP 与控制监听不再接受连接（socket 已关）。
	require.Eventually(t, func() bool {
		_, err := net.DialTimeout("tcp", strings.TrimPrefix(s.base, "http://"), 200*time.Millisecond)
		return err != nil
	}, 3*time.Second, 100*time.Millisecond, "HTTP listener still accepting after shutdown")
	require.Eventually(t, func() bool {
		_, err := net.DialTimeout("tcp", s.ctrlAddr, 200*time.Millisecond)
		return err != nil
	}, 3*time.Second, 100*time.Millisecond, "control listener still accepting after shutdown")
}
