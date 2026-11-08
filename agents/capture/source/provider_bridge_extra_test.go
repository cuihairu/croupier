package source

// provider_bridge.go 补测：Discover/Snapshot 的能力缺失显式拒绝、Poll 的
// Open 失败透传、事件流关闭收口、发送阻塞下的 ctx 取消两条收敛路径、
// AckCheckpoint 的无桥接与非法位点守卫。
//
// 诚实跳过（结构性）：CheckPoint 无桥接分支已被既有 guards 用例覆盖；
// Poll 的白名单过滤与 bridgeEvent 错误路径同上。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/stretchr/testify/require"
)

// stubBridgeProvider 可注入 Open 失败与事件流桥接替身。
type stubBridgeProvider struct {
	openErr error
	events  chan ChangeEvent
	gotBM   Bookmark
}

func (p *stubBridgeProvider) Open(_ context.Context, bm Bookmark) (<-chan ChangeEvent, error) {
	if p.openErr != nil {
		return nil, p.openErr
	}
	p.gotBM = bm
	return p.events, nil
}

func (p *stubBridgeProvider) Bookmark() Bookmark { return p.gotBM }
func (p *stubBridgeProvider) Close() error       { return nil }

func TestProviderBridgeCapabilityGapsRefused(t *testing.T) {
	p := NewProviderConnector(&stubBridgeProvider{}, "mysql", nil)
	_, err := p.Discover(t.Context(), connector.DiscoveryRequest{})
	require.ErrorContains(t, err, "discovery unsupported for provider-bridged source")
	_, err = p.Snapshot(t.Context(), connector.SnapshotRequest{})
	require.ErrorIs(t, err, connector.ErrUnsupportedSnapshot)
}

func TestProviderBridgePollOpenError(t *testing.T) {
	p := NewProviderConnector(&stubBridgeProvider{openErr: errors.New("dial down")}, "mysql", nil)
	_, err := p.Poll(t.Context(), connector.PollRequest{From: connector.Position{Value: "g:1"}})
	require.ErrorContains(t, err, "dial down")
}

func TestProviderBridgePollClosesOnStreamEnd(t *testing.T) {
	ev := make(chan ChangeEvent, 1)
	p := NewProviderConnector(&stubBridgeProvider{events: ev}, "mysql", nil)
	p.cfg = connector.SourceConfig{SourceID: "src-1"}

	out, err := p.Poll(t.Context(), connector.PollRequest{})
	require.NoError(t, err)
	ev <- ChangeEvent{Database: "game", Table: "items", Op: OpInsert, GTID: "g:1"}
	close(ev)

	ce, ok := <-out
	require.True(t, ok, "bridged event must pass through before close")
	require.Equal(t, "src-1", ce.SourceID)
	require.Equal(t, "gtid", ce.Position.Kind)
	_, ok = <-out
	require.False(t, ok, "source stream end must close the contract stream")
}

func TestProviderBridgePollSendCancelledByCtx(t *testing.T) {
	// out 缓冲 64：65 条单事件把缓冲填满并卡在发送 select 上，随后取消
	// ctx——非缓冲 events 的第 65 次发送完成即证明桥接已阻塞在 out<-。
	ev := make(chan ChangeEvent)
	p := NewProviderConnector(&stubBridgeProvider{events: ev}, "mysql", nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	out, err := p.Poll(ctx, connector.PollRequest{})
	require.NoError(t, err)

	go func() {
		for i := 0; i < 65; i++ {
			ev <- ChangeEvent{Database: "game", Table: "items", Op: OpInsert}
		}
	}()
	time.Sleep(300 * time.Millisecond) // 前 64 条入缓冲，第 65 条阻塞发送
	cancel()
	waitClosed(t, out)
}

func TestProviderBridgePollCtxCancelsIdleStream(t *testing.T) {
	// 事件流安静未关闭 → 外层 select 的 ctx.Done 收敛
	ev := make(chan ChangeEvent)
	p := NewProviderConnector(&stubBridgeProvider{events: ev}, "mysql", nil)

	ctx, cancel := context.WithCancel(t.Context())
	out, err := p.Poll(ctx, connector.PollRequest{})
	require.NoError(t, err)
	cancel()
	waitClosed(t, out)
}

func TestProviderBridgeAckCheckpointGuards(t *testing.T) {
	// 无桥接
	p := &ProviderConnector{sourceType: "mysql", log: nil}
	require.ErrorContains(t, p.AckCheckpoint(t.Context(), connector.Position{Kind: "gtid", Value: "g:1"}),
		"no provider bridged")

	// 位点 kind 与源类型不符 → 拒绝；kind 在闭集但值为空属合法位点
	// （Valid 只校验 kind 闭集，不校验值形态）
	p = NewProviderConnector(&stubBridgeProvider{}, "mysql", nil)
	require.ErrorContains(t, p.AckCheckpoint(t.Context(), connector.Position{Kind: "lsn", Value: "g:1"}),
		"invalid mysql checkpoint")
	require.NoError(t, p.AckCheckpoint(t.Context(), connector.Position{Kind: "gtid", Value: ""}))
}

// waitClosed 轮询等待契约流关闭（桥接 goroutine 异步收口）。
func waitClosed(t *testing.T, out <-chan connector.ChangeEvent) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case _, ok := <-out:
			if !ok {
				return
			}
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatal("contract stream did not close in time")
}
