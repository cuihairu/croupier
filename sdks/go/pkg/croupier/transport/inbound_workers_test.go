package transport

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
)

// 入站请求由固定 worker 池并发处理：一个慢 handler 不得阻塞后续请求
// （串行处理曾造成 10s 头部阻塞——单个慢 invoke 卡住整条连接）。
func TestTCPClient_InboundWorkers_ConcurrentHandling(t *testing.T) {
	t.Parallel()

	var inFlight, maxInFlight int64
	var peakMu sync.Mutex

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	// 模拟 Agent：每请求停留 100ms（若串行处理，5 个并发 = 500ms+）。
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveSlowEcho(conn, &inFlight, &maxInFlight, &peakMu, 100*time.Millisecond)
		}
	}()

	client, err := NewTCPClient(&Config{
		Address:        listener.Addr().String(),
		Insecure:       true,
		DialTimeout:    5 * time.Second,
		InboundHandler: noopInbound,
		InboundWorkers: 4,
		InboundQLen:    16,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// 模拟 Agent 连发 5 个入站请求（invoke 响应帧回写给 client 的 Call）。
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			frame := buildRequestFrame(protocol.MsgInvokeRequest, 1000+i, []byte(`{}`))
			if _, err := client.conn.Write(frame); err != nil {
				t.Error(err)
			}
			time.Sleep(10 * time.Millisecond)
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	peakMu.Lock()
	peak := maxInFlight
	peakMu.Unlock()
	if peak < 2 {
		t.Fatalf("expected concurrent handling (peak >= 2), got %d — processing is serial", peak)
	}
	if elapsed > 400*time.Millisecond {
		t.Fatalf("5 concurrent 100ms requests took %v — head-of-line blocking", elapsed)
	}
}

// 队列满时立即回 busy 响应（InvokeRequest 错误格式），不排队积累。
func TestTCPClient_InboundQueueFull_ImmediateBusy(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	release := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// 吞下请求不回——模拟 handler 全部卡住，直到测试放行。
			go func(c net.Conn) {
				buf := make([]byte, 4096)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
					<-release
				}
			}(conn)
		}
	}()

	client, err := NewTCPClient(&Config{
		Address:        listener.Addr().String(),
		Insecure:       true,
		DialTimeout:    5 * time.Second,
		InboundHandler: noopInbound,
		InboundWorkers: 1,
		InboundQLen:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// 填满 worker+队列（1 worker 卡死 + 队列 1）后，下一个请求必须立即
	// 收到 busy 错误响应而不是排队。
	start := time.Now()
	frame := buildRequestFrame(protocol.MsgInvokeRequest, 2000, []byte(`{}`))
	for i := 0; i < 5; i++ {
		if _, err := client.conn.Write(frame); err != nil {
			t.Fatal(err)
		}
	}
	// 等待 busy 响应帧到达（读循环会把它转给 pending——我们直接读 conn
	// 前置 header 就行，简化：轮询 pending 不适用，改用原始 conn 读）。
	client.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	total := 0
	for total < 3 {
		n, err := client.conn.Read(buf[total:])
		if err != nil {
			t.Fatalf("no busy response within deadline: %v (read %d bytes in %v)", err, total, time.Since(start))
		}
		total += n
		if strings.Contains(string(buf[:total]), "inbound queue full") {
			break
		}
	}
	close(release)
}

func buildRequestFrame(msgID uint32, reqID uint32, body []byte) []byte {
	frameBody := protocol.NewMessageBody(msgID, reqID, body)
	frame := make([]byte, protocol.HeaderSize+len(frameBody))
	// Version + MsgID(3) + RequestID(4)
	frame[0] = protocol.Version1
	protocol.PutMsgID(frame[1:4], msgID)
	frame[4] = byte(reqID >> 24)
	frame[5] = byte(reqID >> 16)
	frame[6] = byte(reqID >> 8)
	frame[7] = byte(reqID)
	copy(frame[protocol.HeaderSize:], frameBody)
	// 长度前缀
	total := len(frame)
	frame[0] = byte(total >> 24) // 会覆盖 version——需按 framing 规范重排：
	// framing: [4B len][8B header][body]
	out := make([]byte, 4+len(frameBody))
	binary.PutUint32(out[:4], uint32(len(frameBody)))
	out[4] = protocol.Version1
	protocol.PutMsgID(out[5:8], msgID)
	out[8] = byte(reqID >> 24)
	out[9] = byte(reqID >> 16)
	out[10] = byte(reqID >> 8)
	out[11] = byte(reqID)
	copy(out[12:], frameBody)
	return out
}

func serveSlowEcho(conn net.Conn, inFlight, maxInFlight *int64, peakMu *sync.Mutex, delay time.Duration) {
	defer conn.Close()
	cur := atomic.AddInt64(inFlight, 1)
	peakMu.Lock()
	if cur > *maxInFlight {
		*maxInFlight = cur
	}
	peakMu.Unlock()
	time.Sleep(delay)
	atomic.AddInt64(inFlight, -1)
	buf := make([]byte, 4096)
	for {
		if _, err := conn.Read(buf); err != nil {
			return
		}
	}
}
