package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

const probeProg = "e2e-agent-probe"

// ── 线协议帧读写（与 internal/transport/tcp 的 4 字节长度前缀一致）───────

type frame struct {
	msgID uint32
	reqID uint32
	body  []byte
}

func writeFrameTo(w io.Writer, f frame) error {
	payload := protocol.NewMessageBody(f.msgID, f.reqID, f.body)
	buf := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(payload)))
	copy(buf[4:], payload)
	for len(buf) > 0 {
		n, err := w.Write(buf)
		if err != nil {
			return err
		}
		buf = buf[n:]
	}
	return nil
}

func readFrameFrom(r io.Reader) (frame, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return frame{}, err
	}
	payload := make([]byte, binary.BigEndian.Uint32(hdr[:]))
	if _, err := io.ReadFull(r, payload); err != nil {
		return frame{}, err
	}
	_, msgID, reqID, body, err := protocol.ParseMessageFromBody(payload)
	if err != nil {
		return frame{}, err
	}
	return frame{msgID: msgID, reqID: reqID, body: body}, nil
}

func mustMarshal(t *testing.T, m proto.Message) []byte {
	t.Helper()
	body, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("marshal %T: %v", m, err)
	}
	return body
}

// startMock 启动一个本地 TCP 桩；handle 决定该连接的行为，不能使用 *testing.T
// （连接 goroutine 的生命周期可能长于测试函数本身）。
func startMock(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				handle(conn)
			}()
		}
	}()
	return ln.Addr().String()
}

// handshakeMock 应答 Register 与 Heartbeat（respondHeartbeat=false 时忽略心跳），
// 之后阻塞读到对端关闭。
func handshakeMock(regBody []byte, respondHeartbeat bool) func(net.Conn) {
	return func(conn net.Conn) {
		f, err := readFrameFrom(conn)
		if err != nil || f.msgID != protocol.MsgRegisterRequest {
			return
		}
		_ = writeFrameTo(conn, frame{msgID: protocol.MsgRegisterResponse, reqID: f.reqID, body: regBody})
		if !respondHeartbeat {
			_, _ = io.Copy(io.Discard, conn)
			return
		}
		f, err = readFrameFrom(conn)
		if err != nil || f.msgID != protocol.MsgHeartbeatRequest {
			return
		}
		_ = writeFrameTo(conn, frame{msgID: protocol.MsgHeartbeatResponse, reqID: f.reqID})
		_, _ = io.Copy(io.Discard, conn)
	}
}

func registerBody(t *testing.T, session string) []byte {
	t.Helper()
	return mustMarshal(t, &agentv1.RegisterResponse{SessionId: session})
}

// ── probeHandler 的事件记录器（send 注入点）─────────────────────────────

type eventRecorder struct {
	mu     sync.Mutex
	events []*sdkv1.TaskEvent
	err    error
}

func (r *eventRecorder) send(ctx context.Context, msgID uint32, body []byte) error {
	if msgID != protocol.MsgTaskEvent {
		return errors.New("unexpected msg id")
	}
	if r.err != nil {
		return r.err
	}
	ev := &sdkv1.TaskEvent{}
	if err := proto.Unmarshal(body, ev); err != nil {
		return err
	}
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
	return nil
}

func (r *eventRecorder) snapshot() []*sdkv1.TaskEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*sdkv1.TaskEvent, len(r.events))
	copy(out, r.events)
	return out
}

func eventTypes(events []*sdkv1.TaskEvent) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		out = append(out, ev.GetType())
	}
	return out
}

func newTestHandler(stepMs int) (*probeHandler, *eventRecorder) {
	rec := &eventRecorder{}
	h := &probeHandler{
		stepMs:      stepMs,
		cancelFuncs: make(map[string]context.CancelFunc),
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		send:        rec.send,
	}
	return h, rec
}

func waitFor(t *testing.T, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", desc)
}

func invokeBody(t *testing.T, taskID string) []byte {
	t.Helper()
	req := &sdkv1.InvokeRequest{FunctionId: "e2e.echo"}
	if taskID != "" {
		req.Metadata = map[string]string{"taskId": taskID}
	}
	return mustMarshal(t, req)
}

func cancelBody(t *testing.T, taskID string) []byte {
	t.Helper()
	return mustMarshal(t, &sdkv1.CancelTaskRequest{TaskId: taskID})
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// ── flag / 校验 / 拨号 ──────────────────────────────────────────────────

func TestRunMainFlagErrors(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		var out bytes.Buffer
		if code := runMain(probeProg, []string{"-h"}, io.Discard, &out); code != 0 {
			t.Fatalf("exit code = %d, want 0 (out=%q)", code, out.String())
		}
		for _, want := range []string{"Usage of e2e-agent-probe:", "-agent-id", "-mock-task", "-step-ms"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("usage output missing %q, got %q", want, out.String())
			}
		}
	})
	t.Run("unknown flag", func(t *testing.T) {
		var out bytes.Buffer
		if code := runMain(probeProg, []string{"-nope"}, io.Discard, &out); code != 2 {
			t.Fatalf("exit code = %d, want 2 (out=%q)", code, out.String())
		}
		if !strings.Contains(out.String(), "flag provided but not defined: -nope") {
			t.Fatalf("missing flag error, got %q", out.String())
		}
	})
}

func TestRunMainMissingAgentID(t *testing.T) {
	var errOut bytes.Buffer
	if code := runMain(probeProg, nil, io.Discard, &errOut); code != 1 {
		t.Fatalf("exit code = %d, want 1 (out=%q)", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "FAIL — -agent-id is required") {
		t.Fatalf("missing diagnostic, got %q", errOut.String())
	}
}

func TestRunMainDialFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	var errOut bytes.Buffer
	code := runMain(probeProg, []string{"-addr", addr, "-agent-id", "probe-1"}, io.Discard, &errOut)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (out=%q)", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "FAIL — dial "+addr) {
		t.Fatalf("missing dial diagnostic, got %q", errOut.String())
	}
}

// ── call() ─────────────────────────────────────────────────────────────

func newTestMux(t *testing.T, addr string) *tcp.MuxConn {
	t.Helper()
	conn, err := tcp.Dial(&tcp.Config{Address: addr, Insecure: true, ConnectTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	handler, _ := newTestHandler(0)
	mux := tcp.NewMuxConn(conn, &tcp.Config{RecvTimeout: 5 * time.Second, SendTimeout: 5 * time.Second}, handler)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mux.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("mux.Run did not return after cancel")
		}
		_ = mux.Close()
	})
	return mux
}

func TestCallMarshalError(t *testing.T) {
	mux := newTestMux(t, startMock(t, func(conn net.Conn) { _, _ = io.Copy(io.Discard, conn) }))
	_, err := call(mux, time.Second, protocol.MsgRegisterRequest,
		&agentv1.RegisterRequest{AgentId: "\xff"}, protocol.MsgRegisterResponse)
	if err == nil {
		t.Fatal("expected marshal error for invalid UTF-8, got nil")
	}
	if !strings.Contains(err.Error(), "marshal RegisterRequest") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallTimeout(t *testing.T) {
	mux := newTestMux(t, startMock(t, func(conn net.Conn) { _, _ = io.Copy(io.Discard, conn) }))
	_, err := call(mux, 200*time.Millisecond, protocol.MsgRegisterRequest,
		&agentv1.RegisterRequest{AgentId: "probe-1"}, protocol.MsgRegisterResponse)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "call RegisterRequest: context deadline exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallResponseIDMismatch(t *testing.T) {
	mux := newTestMux(t, startMock(t, func(conn net.Conn) {
		f, err := readFrameFrom(conn)
		if err != nil {
			return
		}
		_ = writeFrameTo(conn, frame{msgID: protocol.MsgHeartbeatResponse, reqID: f.reqID})
	}))
	_, err := call(mux, time.Second, protocol.MsgRegisterRequest,
		&agentv1.RegisterRequest{AgentId: "probe-1"}, protocol.MsgRegisterResponse)
	if err == nil {
		t.Fatal("expected msgID mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "expected RegisterResponse (0x010102), got 0x010104") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── probeHandler ───────────────────────────────────────────────────────

func TestHandleStartTask(t *testing.T) {
	h, rec := newTestHandler(0)
	onTaskCalls := 0
	h.onTask = func() { onTaskCalls++ }

	resp, err := h.Handle(testCtx(t), protocol.MsgStartTaskRequest, 7, invokeBody(t, "task-1"))
	if err != nil {
		t.Fatalf("Handle(StartTask) = %v", err)
	}
	started := &sdkv1.StartTaskResponse{}
	if err := proto.Unmarshal(resp, started); err != nil {
		t.Fatalf("unmarshal StartTaskResponse: %v", err)
	}
	if started.GetTaskId() != "task-1" {
		t.Fatalf("task id = %q, want task-1", started.GetTaskId())
	}
	if onTaskCalls != 1 {
		t.Fatalf("onTask calls = %d, want 1", onTaskCalls)
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.waitIdle(waitCtx)

	events := rec.snapshot()
	wantTypes := []string{"started", "progress", "progress", "progress", "progress", "progress", "completed"}
	gotTypes := eventTypes(events)
	if len(gotTypes) != len(wantTypes) {
		t.Fatalf("event types = %v, want %v", gotTypes, wantTypes)
	}
	for i := range wantTypes {
		if gotTypes[i] != wantTypes[i] {
			t.Fatalf("event types = %v, want %v", gotTypes, wantTypes)
		}
	}
	var progresses []int32
	for _, ev := range events {
		if ev.GetType() == "progress" {
			progresses = append(progresses, ev.GetProgress())
		}
	}
	if len(progresses) != 5 || progresses[0] != 20 || progresses[4] != 100 {
		t.Fatalf("progress events = %v, want [20 40 60 80 100]", progresses)
	}
	if !strings.Contains(string(events[len(events)-1].GetPayload()), "e2e-probe") {
		t.Fatalf("completed payload = %q, want e2e-probe marker", events[len(events)-1].GetPayload())
	}
}

func TestHandleStartTaskGeneratedIDWithoutOnTask(t *testing.T) {
	h, _ := newTestHandler(0)
	// h.onTask 为 nil：覆盖 onTask 缺省分支。
	resp, err := h.Handle(testCtx(t), protocol.MsgStartTaskRequest, 1, invokeBody(t, ""))
	if err != nil {
		t.Fatalf("Handle(StartTask) = %v", err)
	}
	started := &sdkv1.StartTaskResponse{}
	if err := proto.Unmarshal(resp, started); err != nil {
		t.Fatalf("unmarshal StartTaskResponse: %v", err)
	}
	if !strings.HasPrefix(started.GetTaskId(), "probe-") {
		t.Fatalf("generated task id = %q, want probe- prefix", started.GetTaskId())
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.waitIdle(waitCtx)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.tasksHandled != 1 {
		t.Fatalf("tasksHandled = %d, want 1", h.tasksHandled)
	}
}

func TestHandleStartTaskBadBody(t *testing.T) {
	h, _ := newTestHandler(0)
	// 长度前缀声明 8 字节但只有 1 字节：截断的 proto。
	_, err := h.Handle(testCtx(t), protocol.MsgStartTaskRequest, 1, []byte{0x0a, 0x08, 'x'})
	if err == nil {
		t.Fatal("expected unmarshal error, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal InvokeRequest") {
		t.Fatalf("unexpected error: %v", err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.tasksHandled != 0 {
		t.Fatalf("tasksHandled = %d, want 0", h.tasksHandled)
	}
}

func TestHandleCancelTaskUnknownTask(t *testing.T) {
	h, rec := newTestHandler(0)
	resp, err := h.Handle(testCtx(t), protocol.MsgCancelTaskRequest, 2, cancelBody(t, "ghost"))
	if err != nil {
		t.Fatalf("Handle(CancelTask) = %v", err)
	}
	ack := &sdkv1.StartTaskResponse{}
	if err := proto.Unmarshal(resp, ack); err != nil {
		t.Fatalf("unmarshal ack: %v", err)
	}
	if ack.GetTaskId() != "" {
		t.Fatalf("ack task id = %q, want empty", ack.GetTaskId())
	}
	events := rec.snapshot()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].GetType() != "cancelled" || events[0].GetMessage() != "task already finished" {
		t.Fatalf("event = %s/%q, want cancelled/task already finished", events[0].GetType(), events[0].GetMessage())
	}
	if events[0].GetTaskId() != "ghost" {
		t.Fatalf("event task id = %q, want ghost", events[0].GetTaskId())
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelsHandled != 1 {
		t.Fatalf("cancelsHandled = %d, want 1", h.cancelsHandled)
	}
}

func TestHandleCancelTaskRunningTask(t *testing.T) {
	h, rec := newTestHandler(200)
	if _, err := h.Handle(testCtx(t), protocol.MsgStartTaskRequest, 1, invokeBody(t, "task-run")); err != nil {
		t.Fatalf("Handle(StartTask) = %v", err)
	}
	waitFor(t, "cancel func registration", func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		_, ok := h.cancelFuncs["task-run"]
		return ok
	})

	if _, err := h.Handle(testCtx(t), protocol.MsgCancelTaskRequest, 2, cancelBody(t, "task-run")); err != nil {
		t.Fatalf("Handle(CancelTask) = %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.waitIdle(waitCtx)

	events := rec.snapshot()
	last := events[len(events)-1]
	if last.GetType() != "cancelled" || last.GetMessage() != "cancelled by request" {
		t.Fatalf("last event = %s/%q, want cancelled/cancelled by request", last.GetType(), last.GetMessage())
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelsHandled != 1 {
		t.Fatalf("cancelsHandled = %d, want 1", h.cancelsHandled)
	}
	if len(h.cancelFuncs) != 0 {
		t.Fatalf("cancelFuncs not cleaned up: %d entries", len(h.cancelFuncs))
	}
}

func TestHandleCancelTaskBadBody(t *testing.T) {
	h, _ := newTestHandler(0)
	_, err := h.Handle(testCtx(t), protocol.MsgCancelTaskRequest, 1, []byte{0x0a, 0x08, 'x'})
	if err == nil {
		t.Fatal("expected unmarshal error, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal CancelTaskRequest") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleUnsupportedMsgID(t *testing.T) {
	h, _ := newTestHandler(0)
	_, err := h.Handle(testCtx(t), protocol.MsgInvokeRequest, 1, nil)
	if err == nil {
		t.Fatal("expected unsupported msgID error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported msgID") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendEventMarshalFailure(t *testing.T) {
	h, rec := newTestHandler(0)
	// TaskEvent.message 是 proto3 string：非法 UTF-8 使 Marshal 失败。
	h.sendEvent("task-1", "started", "\xff", 0, nil)
	if got := len(rec.snapshot()); got != 0 {
		t.Fatalf("events = %d, want 0 (marshal must fail)", got)
	}
}

func TestSendEventWarnsOnSendFailure(t *testing.T) {
	var logs bytes.Buffer
	h, _ := newTestHandler(0)
	h.logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	h.send = func(ctx context.Context, msgID uint32, body []byte) error {
		return errors.New("connection closed")
	}
	h.sendEvent("task-1", "started", "mock task started", 0, nil)
	if !strings.Contains(logs.String(), "send task event failed") {
		t.Fatalf("missing warn log, got %q", logs.String())
	}
	if !strings.Contains(logs.String(), "task_id=task-1") {
		t.Fatalf("missing task_id in log, got %q", logs.String())
	}
}

func TestWaitIdleRespectsContext(t *testing.T) {
	h, _ := newTestHandler(1000)
	if _, err := h.Handle(testCtx(t), protocol.MsgStartTaskRequest, 1, invokeBody(t, "task-slow")); err != nil {
		t.Fatalf("Handle(StartTask) = %v", err)
	}

	shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	h.waitIdle(shortCtx)
	cancel()
	h.mu.Lock()
	running := len(h.cancelFuncs) == 1
	h.mu.Unlock()
	if !running {
		t.Fatal("task should still be inflight after waitIdle ctx expired")
	}

	if _, err := h.Handle(testCtx(t), protocol.MsgCancelTaskRequest, 2, cancelBody(t, "task-slow")); err != nil {
		t.Fatalf("Handle(CancelTask) = %v", err)
	}
	waitCtx, cancelIdle := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelIdle()
	h.waitIdle(waitCtx)
}

// ── serveLoop ──────────────────────────────────────────────────────────

func runServeLoop(t *testing.T, h *probeHandler, taskCh <-chan struct{}, runErrCh <-chan error, exitAfter int, dur time.Duration) (error, time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- serveLoop(h, taskCh, runErrCh, exitAfter, dur) }()
	select {
	case err := <-done:
		return err, time.Since(start)
	case <-time.After(5 * time.Second):
		t.Fatal("serveLoop did not return within 5s")
		return nil, 0
	}
}

func TestServeLoopDeadline(t *testing.T) {
	h, _ := newTestHandler(0)
	err, elapsed := runServeLoop(t, h, make(chan struct{}), make(chan error), 0, 30*time.Millisecond)
	if err != nil {
		t.Fatalf("serveLoop = %v, want nil", err)
	}
	if elapsed < 25*time.Millisecond {
		t.Fatalf("serveLoop returned after %s, want to wait for the deadline", elapsed)
	}
}

func TestServeLoopExitAfterTasksReached(t *testing.T) {
	h, _ := newTestHandler(0)
	h.tasksHandled = 1
	taskCh := make(chan struct{}, 1)
	taskCh <- struct{}{}
	// 只有 taskCh 就绪：必须在这里退出，而不是等到 serve-duration（1min）。
	err, _ := runServeLoop(t, h, taskCh, make(chan error), 1, time.Minute)
	if err != nil {
		t.Fatalf("serveLoop = %v, want nil", err)
	}
}

func TestServeLoopKeepsServingBelowThreshold(t *testing.T) {
	cases := []struct {
		name         string
		tasksHandled int
		exitAfter    int
	}{
		{name: "zero threshold", tasksHandled: 5, exitAfter: 0},
		{name: "below threshold", tasksHandled: 1, exitAfter: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestHandler(0)
			h.tasksHandled = tc.tasksHandled
			taskCh := make(chan struct{}, 1)
			taskCh <- struct{}{}
			runErrCh := make(chan error, 1)
			go func() {
				time.Sleep(50 * time.Millisecond)
				runErrCh <- nil
			}()
			// serve-duration 取 3s：若 task 分支错误地 break，函数会立刻返回；
			// 正确行为是继续等待并消费 runErrCh（约 50ms）。
			err, elapsed := runServeLoop(t, h, taskCh, runErrCh, tc.exitAfter, 3*time.Second)
			if err != nil {
				t.Fatalf("serveLoop = %v, want nil", err)
			}
			if elapsed < 40*time.Millisecond {
				t.Fatalf("serveLoop returned after %s: task signal must not break the loop", elapsed)
			}
			if len(runErrCh) != 0 {
				t.Fatal("runErrCh value was not consumed: loop exited before runErrCh fired")
			}
		})
	}
}

func TestServeLoopRunErrNil(t *testing.T) {
	h, _ := newTestHandler(0)
	runErrCh := make(chan error, 1)
	runErrCh <- nil
	err, _ := runServeLoop(t, h, make(chan struct{}), runErrCh, 0, time.Minute)
	if err != nil {
		t.Fatalf("serveLoop = %v, want nil", err)
	}
}

func TestServeLoopRunErrNonNil(t *testing.T) {
	h, _ := newTestHandler(0)
	runErrCh := make(chan error, 1)
	runErrCh <- errors.New("boom")
	err, _ := runServeLoop(t, h, make(chan struct{}), runErrCh, 0, time.Minute)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("serveLoop = %v, want boom", err)
	}
}

// ── runMain 端到端（本地 TCP 桩）────────────────────────────────────────

func TestRunMainHandshake(t *testing.T) {
	regBody := registerBody(t, "sess-1")
	addr := startMock(t, handshakeMock(regBody, true))

	var stdout, stderr bytes.Buffer
	code := runMain(probeProg, []string{
		"-addr", addr, "-agent-id", "probe-1", "-game-id", "g1", "-env", "dev",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "e2e-agent-probe: PASS") {
		t.Fatalf("stdout missing PASS, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "registered agent=probe-1 session=sess-1 game=g1 env=dev functions=0") {
		t.Fatalf("stderr missing register line, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "heartbeat ok agent=probe-1") {
		t.Fatalf("stderr missing heartbeat line, got %q", stderr.String())
	}
}

func TestRunMainHandshakeSkipHeartbeat(t *testing.T) {
	regBody := registerBody(t, "sess-2")
	addr := startMock(t, handshakeMock(regBody, true))

	var stdout, stderr bytes.Buffer
	code := runMain(probeProg, []string{
		"-addr", addr, "-agent-id", "probe-2", "-skip-heartbeat", "-step-ms=-1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "e2e-agent-probe: PASS") {
		t.Fatalf("stdout missing PASS, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "heartbeat ok") {
		t.Fatalf("heartbeat must be skipped, got %q", stderr.String())
	}
}

func TestRunMainRegisterTimeout(t *testing.T) {
	addr := startMock(t, func(conn net.Conn) { _, _ = io.Copy(io.Discard, conn) })

	var stderr bytes.Buffer
	code := runMain(probeProg, []string{
		"-addr", addr, "-agent-id", "probe-1", "-timeout", "200ms",
	}, io.Discard, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "FAIL — register: call RegisterRequest: context deadline exceeded") {
		t.Fatalf("stderr missing register timeout, got %q", stderr.String())
	}
}

func TestRunMainRegisterBadResponse(t *testing.T) {
	addr := startMock(t, func(conn net.Conn) {
		f, err := readFrameFrom(conn)
		if err != nil || f.msgID != protocol.MsgRegisterRequest {
			return
		}
		// 字段 1（varint）后缺值：非法 wire format。
		_ = writeFrameTo(conn, frame{msgID: protocol.MsgRegisterResponse, reqID: f.reqID, body: []byte{0x08}})
	})

	var stderr bytes.Buffer
	code := runMain(probeProg, []string{"-addr", addr, "-agent-id", "probe-1", "-timeout", "2s"}, io.Discard, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "FAIL — unmarshal RegisterResponse:") {
		t.Fatalf("stderr missing unmarshal failure, got %q", stderr.String())
	}
}

func TestRunMainRegisterEmptySession(t *testing.T) {
	addr := startMock(t, func(conn net.Conn) {
		f, err := readFrameFrom(conn)
		if err != nil || f.msgID != protocol.MsgRegisterRequest {
			return
		}
		_ = writeFrameTo(conn, frame{msgID: protocol.MsgRegisterResponse, reqID: f.reqID})
	})

	var stderr bytes.Buffer
	code := runMain(probeProg, []string{"-addr", addr, "-agent-id", "probe-1", "-timeout", "2s"}, io.Discard, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "FAIL — register returned empty session id") {
		t.Fatalf("stderr missing empty session failure, got %q", stderr.String())
	}
}

func TestRunMainHeartbeatFailure(t *testing.T) {
	regBody := registerBody(t, "sess-3")
	addr := startMock(t, handshakeMock(regBody, false))

	var stderr bytes.Buffer
	code := runMain(probeProg, []string{
		"-addr", addr, "-agent-id", "probe-1", "-timeout", "200ms",
	}, io.Discard, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "FAIL — heartbeat: call HeartbeatRequest: context deadline exceeded") {
		t.Fatalf("stderr missing heartbeat failure, got %q", stderr.String())
	}
}

func TestRunMainServeTasks(t *testing.T) {
	regBody := registerBody(t, "sess-serve")
	startReqBody := invokeBody(t, "e2e-task-1")
	regReqCh := make(chan *agentv1.RegisterRequest, 1)
	events := make(chan *sdkv1.TaskEvent, 64)
	addr := startMock(t, func(conn net.Conn) {
		f, err := readFrameFrom(conn)
		if err != nil || f.msgID != protocol.MsgRegisterRequest {
			return
		}
		req := &agentv1.RegisterRequest{}
		_ = proto.Unmarshal(f.body, req)
		select {
		case regReqCh <- req:
		default:
		}
		_ = writeFrameTo(conn, frame{msgID: protocol.MsgRegisterResponse, reqID: f.reqID, body: regBody})
		f, err = readFrameFrom(conn)
		if err != nil || f.msgID != protocol.MsgHeartbeatRequest {
			return
		}
		_ = writeFrameTo(conn, frame{msgID: protocol.MsgHeartbeatResponse, reqID: f.reqID})
		_ = writeFrameTo(conn, frame{
			msgID: protocol.MsgStartTaskRequest,
			reqID: 9001,
			body:  startReqBody,
		})
		for {
			f, err := readFrameFrom(conn)
			if err != nil {
				return
			}
			if f.msgID != protocol.MsgTaskEvent {
				continue
			}
			ev := &sdkv1.TaskEvent{}
			if proto.Unmarshal(f.body, ev) != nil {
				continue
			}
			select {
			case events <- ev:
			default:
			}
		}
	})

	var stdout, stderr bytes.Buffer
	code := runMain(probeProg, []string{
		"-addr", addr, "-agent-id", "probe-serve", "-game-id", "g1", "-env", "dev",
		"-mock-task", "e2e.echo", "-exit-after-tasks", "1", "-serve-duration", "10s",
		"-step-ms=-1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "e2e-agent-probe: PASS") {
		t.Fatalf("stdout missing PASS, got %q", stdout.String())
	}
	// -step-ms=-1 归零后体现在 serving 行。
	if !strings.Contains(stderr.String(), "serving function=e2e.echo exit-after-tasks=1 max=10s step=0ms") {
		t.Fatalf("stderr missing serving line, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "served tasks=1 cancels=0") {
		t.Fatalf("stderr missing served summary, got %q", stderr.String())
	}

	select {
	case req := <-regReqCh:
		if len(req.GetFunctions()) != 1 || req.GetFunctions()[0].GetId() != "e2e.echo" {
			t.Fatalf("declared functions = %v, want [e2e.echo]", req.GetFunctions())
		}
	case <-time.After(time.Second):
		t.Fatal("register request never observed")
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			if ev.GetTaskId() != "e2e-task-1" {
				t.Fatalf("event task id = %q, want e2e-task-1", ev.GetTaskId())
			}
			if ev.GetType() == "completed" {
				return
			}
		case <-deadline:
			t.Fatal("no completed TaskEvent received")
		}
	}
}

func TestRunMainServeDeadlineWithoutTasks(t *testing.T) {
	regBody := registerBody(t, "sess-idle")
	addr := startMock(t, handshakeMock(regBody, true))

	var stdout, stderr bytes.Buffer
	code := runMain(probeProg, []string{
		"-addr", addr, "-agent-id", "probe-idle",
		"-mock-task", "e2e.echo", "-exit-after-tasks", "0", "-serve-duration", "100ms",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "served tasks=0 cancels=0") {
		t.Fatalf("stderr missing served summary, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "FAIL — serve mode: no tasks received") {
		t.Fatalf("stderr missing serve failure, got %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "PASS") {
		t.Fatalf("stdout must not report PASS, got %q", stdout.String())
	}
}

func TestRunMainServeMuxEndedEarly(t *testing.T) {
	regBody := registerBody(t, "sess-bye")
	addr := startMock(t, func(conn net.Conn) {
		f, err := readFrameFrom(conn)
		if err != nil || f.msgID != protocol.MsgRegisterRequest {
			return
		}
		_ = writeFrameTo(conn, frame{msgID: protocol.MsgRegisterResponse, reqID: f.reqID, body: regBody})
		f, err = readFrameFrom(conn)
		if err != nil || f.msgID != protocol.MsgHeartbeatRequest {
			return
		}
		_ = writeFrameTo(conn, frame{msgID: protocol.MsgHeartbeatResponse, reqID: f.reqID})
	})

	var stderr bytes.Buffer
	code := runMain(probeProg, []string{
		"-addr", addr, "-agent-id", "probe-bye",
		"-mock-task", "e2e.echo", "-exit-after-tasks", "1", "-serve-duration", "10s",
	}, io.Discard, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "FAIL — mux run ended early:") {
		t.Fatalf("stderr missing mux failure, got %q", stderr.String())
	}
}

// ── main 的退出出口 ────────────────────────────────────────────────────

func TestMainUsesExitHook(t *testing.T) {
	origArgs, origExit := os.Args, exit
	defer func() { os.Args, exit = origArgs, origExit }()

	regBody := registerBody(t, "sess-hook")
	addr := startMock(t, handshakeMock(regBody, true))

	cases := []struct {
		name string
		args []string
		want int
	}{
		{name: "missing id", args: []string{probeProg}, want: 1},
		{name: "handshake", args: []string{probeProg, "-addr", addr, "-agent-id", "probe-hook"}, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []int
			exit = func(code int) { got = append(got, code) }
			os.Args = tc.args
			main()
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("exit hook calls = %v, want [%d]", got, tc.want)
			}
		})
	}
}

// ── 子进程重入：验证真实 os.Exit 路径的退出码与输出 ────────────────────

const probeHelperEnv = "CROUPIER_E2E_AGENT_PROBE_MAIN"

// TestMainHelperProcess 只在子进程中按环境变量切换 os.Args 后调用 main()
// （main 经 exit 走真实 os.Exit，不再返回）。
func TestMainHelperProcess(t *testing.T) {
	switch os.Getenv(probeHelperEnv) {
	case "missing":
		os.Args = []string{os.Args[0]}
	case "handshake":
		os.Args = []string{
			os.Args[0],
			"-addr", os.Getenv(probeHelperEnv + "_ADDR"),
			"-agent-id", "sub-probe",
		}
	default:
		return
	}
	main()
}

func TestMainProcessExitCodes(t *testing.T) {
	addr := startMock(t, handshakeMock(registerBody(t, "sess-sub"), true))

	cases := []struct {
		name       string
		mode       string
		wantCode   int
		wantStderr string
		wantStdout string
	}{
		{name: "missing id", mode: "missing", wantCode: 1, wantStderr: "-agent-id is required"},
		{name: "handshake", mode: "handshake", wantCode: 0, wantStderr: "registered agent=sub-probe", wantStdout: "e2e-agent-probe: PASS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
			cmd.Env = append(os.Environ(), probeHelperEnv+"="+tc.mode, probeHelperEnv+"_ADDR="+addr)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			if tc.wantCode == 0 {
				if err != nil {
					t.Fatalf("child err = %v, want success (stderr=%q stdout=%q)", err, stderr.String(), stdout.String())
				}
			} else {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("child err = %v, want exit status %d (stderr=%q)", err, tc.wantCode, stderr.String())
				}
				if code := exitErr.ExitCode(); code != tc.wantCode {
					t.Fatalf("exit code = %d, want %d (stderr=%q)", code, tc.wantCode, stderr.String())
				}
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("stderr missing %q, got %q", tc.wantStderr, stderr.String())
			}
			if tc.wantStdout != "" && !strings.Contains(stdout.String(), tc.wantStdout) {
				t.Fatalf("stdout missing %q, got %q", tc.wantStdout, stdout.String())
			}
		})
	}
}
