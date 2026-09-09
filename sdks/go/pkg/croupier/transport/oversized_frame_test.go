package transport

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// receiveLoop 收到超过 maxFrameBytes 的帧长度头时应立即退出并触发 onClose，
// 而不是尝试分配超大缓冲。
func TestReceiveLoopOversizedFrameClosesConnection(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer func() { _ = serverSide.Close() }()

	c := &TCPClient{
		conn:      clientSide,
		config:    &Config{},
		pending:   make(map[uint32]chan responseTuple),
		nextReqID: 1,
		closing:   make(chan struct{}),
	}

	closed := make(chan struct{}, 1)
	c.SetOnClose(func(err error) {
		select {
		case closed <- struct{}{}:
		default:
		}
	})

	c.readLoopWg.Add(1)
	go c.receiveLoop()

	// 写入伪造的超大帧头（maxFrameBytes + 1）。
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, maxFrameBytes+1)
	writeDone := make(chan error, 1)
	go func() {
		_, err := serverSide.Write(header)
		writeDone <- err
	}()

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("write oversized header: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("write oversized header timed out")
	}

	select {
	case <-closed:
		// onClose 已触发：读循环因超大帧退出。
	case <-time.After(2 * time.Second):
		t.Fatal("expected onClose to fire after oversized frame")
	}

	c.readLoopWg.Wait()
}
