package transport

import (
	"context"
	"net"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
)

// startDrainServer 起一个吸收式 TCP 服务端：读到的原始字节送 ch，不回写。
// worker 池测试只关心「客户端是否写出/是否阻塞」，不解析帧协议。
func startDrainServer(t *testing.T) (net.Listener, chan []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ch := make(chan []byte, 64)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if n > 0 {
						b := make([]byte, n)
						copy(b, buf[:n])
						select {
						case ch <- b:
						default:
						}
					}
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return ln, ch
}

// TestWorkerPoolDefaults 入站 worker 池默认参数：
// 队列容量 = workers×4（workers = NumCPU 下限 2）。
func TestWorkerPoolDefaults(t *testing.T) {
	ln, _ := startDrainServer(t)
	defer ln.Close()

	client, err := NewTCPClient(&Config{
		Address:     ln.Addr().String(),
		Insecure:    true,
		DialTimeout: 2 * time.Second,
		InboundHandler: func(ctx context.Context, msgID, reqID uint32, body []byte) ([]byte, error) {
			return body, nil
		},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	expectWorkers := runtime.NumCPU()
	if expectWorkers < 2 {
		expectWorkers = 2
	}
	if cap(client.inbox) != expectWorkers*4 {
		t.Fatalf("默认队列容量 = %d，期望 workers×4 = %d", cap(client.inbox), expectWorkers*4)
	}
}

// TestWorkerPoolNonBlockingDispatch 慢 handler 不阻塞投递方（读循环）：
// 单 worker + 阻塞 handler 下，连续投递立即返回。
func TestWorkerPoolNonBlockingDispatch(t *testing.T) {
	ln, _ := startDrainServer(t)
	defer ln.Close()

	block := make(chan struct{})
	client, err := NewTCPClient(&Config{
		Address:        ln.Addr().String(),
		Insecure:       true,
		DialTimeout:    2 * time.Second,
		InboundWorkers: 1,
		InboundQLen:    8,
		InboundHandler: func(ctx context.Context, msgID, reqID uint32, body []byte) ([]byte, error) {
			<-block
			return body, nil
		},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()
	defer close(block)

	done := make(chan struct{})
	go func() {
		defer close(done)
		client.handleInboundRequest(protocol.MsgInvokeRequest, 1, []byte(`{}`))
	}()
	select {
	case <-done:
		// 预期：投递非阻塞（读循环不被慢 handler 卡死）
	case <-time.After(500 * time.Millisecond):
		t.Fatal("投递阻塞：慢 handler 会卡死读循环（回归到同步处理）")
	}
}

// TestWorkerPoolQueueFullBusy 队列满时快速回 busy 错误帧（不排队积压内存）。
func TestWorkerPoolQueueFullBusy(t *testing.T) {
	ln, received := startDrainServer(t)
	defer ln.Close()

	block := make(chan struct{})
	client, err := NewTCPClient(&Config{
		Address:        ln.Addr().String(),
		Insecure:       true,
		DialTimeout:    2 * time.Second,
		InboundWorkers: 1,
		InboundQLen:    1, // 1 个在处理 + 1 个排队
		InboundHandler: func(ctx context.Context, msgID, reqID uint32, body []byte) ([]byte, error) {
			<-block
			return body, nil
		},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()
	defer close(block)

	// 占住 worker + 占满队列
	client.handleInboundRequest(protocol.MsgInvokeRequest, 1, []byte(`{}`))
	client.handleInboundRequest(protocol.MsgInvokeRequest, 2, []byte(`{}`))

	// 第 3 个：应立即走 busy 分支并写出错误帧
	wrote := make(chan struct{})
	go func() {
		defer close(wrote)
		client.handleInboundRequest(protocol.MsgInvokeRequest, 3, []byte(`{}`))
	}()
	select {
	case <-wrote:
		// 预期：队列满 → 非阻塞快速返回
	case <-time.After(500 * time.Millisecond):
		t.Fatal("队列满时投递阻塞：busy 快速失败路径失效（内存会积压）")
	}

	// busy 错误帧确实写出（服务端收到非空字节）
	select {
	case b := <-received:
		if len(b) == 0 {
			t.Fatal("busy 路径写出空帧")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("busy 错误帧未写出（Agent 会等到超时且无诊断）")
	}
}

// TestWorkerPoolHandlerErrorAnswers 错误 handler 仍写出响应帧（不吞错误、Agent 不空等）。
func TestWorkerPoolHandlerErrorAnswers(t *testing.T) {
	ln, received := startDrainServer(t)
	defer ln.Close()

	client, err := NewTCPClient(&Config{
		Address:        ln.Addr().String(),
		Insecure:       true,
		DialTimeout:    2 * time.Second,
		InboundWorkers: 1,
		InboundQLen:    4,
		InboundHandler: func(ctx context.Context, msgID, reqID uint32, body []byte) ([]byte, error) {
			return nil, context.DeadlineExceeded
		},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	client.handleInboundRequest(protocol.MsgInvokeRequest, 7, []byte(`{}`))
	select {
	case b := <-received:
		if len(b) == 0 {
			t.Fatal("错误响应写出空帧")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("handler 错误未回错误帧（Agent 会阻塞到超时）")
	}
}

// TestInboundWorkersExplicit 显式队列容量被采纳（不覆盖默认）。
func TestInboundWorkersExplicit(t *testing.T) {
	ln, _ := startDrainServer(t)
	defer ln.Close()

	client, err := NewTCPClient(&Config{
		Address:        ln.Addr().String(),
		Insecure:       true,
		DialTimeout:    2 * time.Second,
		InboundWorkers: 3,
		InboundQLen:    10,
		InboundHandler: func(ctx context.Context, msgID, reqID uint32, body []byte) ([]byte, error) {
			return body, nil
		},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	if cap(client.inbox) != 10 {
		t.Fatalf("显式队列容量被忽略：cap=%d 期望 10", cap(client.inbox))
	}
}

// InboundWorkers=1：单 worker 串行模式（单线程游戏服兼容）——两个任务
// 顺序执行、处理期间无第二个 worker 并发进入；队列容量按 1 缩放。
func TestWorkerPool_SingleWorkerSerial(t *testing.T) {
	var inside, maxInside, completed int32
	release := make(chan struct{})
	srv, _ := startDrainServer(t)
	cfg := &Config{
		Address:        srv.Addr().String(),
		Insecure:       true,
		InboundWorkers: 1,
		InboundHandler: func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
			cur := atomic.AddInt32(&inside, 1)
			for {
				old := atomic.LoadInt32(&maxInside)
				if cur <= old || atomic.CompareAndSwapInt32(&maxInside, old, cur) {
					break
				}
			}
			<-release
			atomic.AddInt32(&inside, -1)
			atomic.AddInt32(&completed, 1)
			return []byte(`{}`), nil
		},
	}
	client, err := NewTCPClient(cfg)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer client.Close()

	if cap(client.inbox) != 4 { // qlen = workers * 4 = 4
		t.Fatalf("inbox cap = %d, want 4", cap(client.inbox))
	}

	// 投递两个任务：第一个占住唯一 worker，第二个只能排队（不并发执行）
	client.inbox <- inboundTask{msgID: protocol.MsgInvokeRequest, reqID: 1}
	client.inbox <- inboundTask{msgID: protocol.MsgInvokeRequest, reqID: 2}
	close(release)

	// 等两个任务全部处理完（worker 空闲时阻塞在 channel 上，不能 Wait）
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&completed) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	client.Close()
	if got := atomic.LoadInt32(&maxInside); got != 1 {
		t.Fatalf("max concurrent handlers = %d, want 1 (serial)", got)
	}
	if n := atomic.LoadInt32(&completed); n != 2 {
		t.Fatalf("completed = %d, want 2", n)
	}
}
