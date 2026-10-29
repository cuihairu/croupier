package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/server"
	"github.com/cuihairu/croupier/internal/svc"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// ARCH-017: 停机收尾必须先取消根 context 再关 listener——活跃 Agent 会话在
// mux.Run(ctx) 阻塞读，取消使 read deadline 触发、会话 goroutine 退出，
// listener.Close 的 wg.Wait 有界返回，不阻塞后续 HTTP 停机/根取消。此前把
// rootCancel 放在 listener.Close 之后，长连接会把 Close 卡到外层 30s 超时。
func TestRunGracefulShutdownDrainsLiveSessionBoundedV9(t *testing.T) {
	store := registry.NewStore()
	listener, err := server.NewTCPListener(&server.TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}, server.NewAgentSessionStore(), store, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	svc := server.NewControlService(store, nil)
	listener.SetHandler(svc)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	go func() { _ = listener.Serve(rootCtx) }()

	// 真实 Agent 连接并注册一个会话：serveConn goroutine 进入 wg（活跃会话）。
	conn, err := net.Dial("tcp", listener.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	regReq, err := proto.Marshal(&agentv1.RegisterRequest{AgentId: "arch017-agent", GameId: "game-1", Env: "dev"})
	require.NoError(t, err)
	frame := protocol.NewMessageBody(protocol.MsgRegisterRequest, 1, regReq)
	wrapped := make([]byte, 4+len(frame))
	wrapped[0] = byte(len(frame) >> 24)
	wrapped[1] = byte(len(frame) >> 16)
	wrapped[2] = byte(len(frame) >> 8)
	wrapped[3] = byte(len(frame))
	copy(wrapped[4:], frame)
	_, err = conn.Write(wrapped)
	require.NoError(t, err)

	// 会话入 store ⇒ serveConn goroutine 已被 wg 追踪（活跃会话）。
	require.Eventually(t, func() bool { return listener.SessionStore().Count() == 1 }, 2*time.Second, 10*time.Millisecond)

	// 有界预算：若 Close 卡在活跃会话上（旧顺序），此调用将耗尽 5s 超时。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	runGracefulShutdown(ctx, rootCancel, &shutdownResources{tcpListener: listener})
	elapsed := time.Since(start)

	assert.True(t, listener.IsClosed(), "listener must be closed after shutdown")
	assert.Less(t, elapsed, 5*time.Second, "shutdown must not block on a live session")
	assert.Equal(t, 0, listener.SessionStore().Count(), "session must be drained on shutdown")
}

// ARCH-017: 全 nil 资源时 shutdown 序列不 panic（对应「组件未启动」装配）。
func TestRunGracefulShutdownNilResourcesV9(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	assert.NotPanics(t, func() {
		runGracefulShutdown(ctx, rootCancel, &shutdownResources{})
	})
	assert.ErrorIs(t, rootCtx.Err(), context.Canceled, "root context must be canceled first")
}

// ARCH-017: 停机收尾必须调用 ServiceContext.Close 冲刷执行日志留痕——root
// 此前从不调用 Close，退出时 writer 队列未冲刷（留痕丢）。runGracefulShutdown
// 经 svcCtx.Close → ExecutionLogWriter.Stop 落库未写批（FlushInterval 设 10s+
// 证明冲刷由 Stop 完成而非定时器）。
func TestRunGracefulShutdownFlushesExecutionLogsV9(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExecutionLog{}))

	w := executionlog.NewWriter(db, executionlog.Config{
		Enabled:       true,
		FlushInterval: 10 * time.Second, // 冲刷必须由 Stop 完成，不能靠定时器
		QueueSize:     16,
		BatchSize:     8,
	})
	ctx, cancel := context.WithCancel(context.Background())
	w.Run(ctx)
	t.Cleanup(cancel)

	svcCtx := &svc.ServiceContext{ExecutionLogWriter: w}
	w.Log(executionlog.Entry{
		GameID: "g1", Env: "dev", Source: "server", FunctionID: "f1",
		PageKey: "p1", Actor: "admin", Route: "/api/x", Status: "ok", DurationMs: 5,
		Request: map[string]string{"a": "1"}, Response: map[string]string{"r": "2"},
	})

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	runGracefulShutdown(shutdownCtx, func() {}, &shutdownResources{svcCtx: svcCtx})

	var n int64
	require.NoError(t, db.Model(&model.ExecutionLog{}).Count(&n).Error)
	assert.Equal(t, int64(1), n, "execution log must be flushed on shutdown")
}
