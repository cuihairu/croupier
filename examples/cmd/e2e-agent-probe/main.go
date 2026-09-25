// Command e2e-agent-probe is a minimal mock Agent that backs the CI E2E.
//
// It has two modes:
//
//  1. Handshake mode (default): register against the Server control-plane TCP
//     listener, send one Heartbeat, then exit 0. Used by happy-path.sh to
//     validate the Agent→Server handshake (Register + Heartbeat).
//
//  2. Serve mode (-mock-task <function_id>): register while declaring the
//     function, then stay connected and act as a mock agent — receive
//     StartTaskRequest, run a fake task that streams started→progress→completed
//     TaskEvents, and honor CancelTaskRequest. Used by the task lifecycle E2E
//     (startTask→events→cancelTask).
//
// It reuses the production transport (internal/transport/tcp.MuxConn) rather
// than re-implementing the wire format, so a protocol drift between probe and
// server is impossible. Exit 0 on success, 1 on failure (diagnostic on stderr).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// exit 是 os.Exit 的测试注入点：main 只经由它退出，测试替换后可直接调用
// main 断言退出码而不终止测试进程；生产路径等价于 os.Exit(code)。
var exit = os.Exit

func main() {
	exit(runMain(os.Args[0], os.Args[1:], os.Stdout, os.Stderr))
}

// runMain 解析 flag 并执行探针，返回进程退出码：
// 0=成功，1=运行期失败（诊断已写 stderr），2=flag 解析错误（-h 为 0）。
func runMain(prog string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(prog, flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:19090", "server control-plane TCP address")
	agentID := fs.String("agent-id", "", "agent id (required)")
	gameID := fs.String("game-id", "", "game scope id")
	env := fs.String("env", "", "environment (e.g. dev/prod)")
	version := fs.String("version", "e2e-probe-1.0", "agent version reported at register")
	insecure := fs.Bool("insecure", true, "use plain TCP (skip TLS)")
	skipHeartbeat := fs.Bool("skip-heartbeat", false, "skip the post-register heartbeat probe")
	timeout := fs.Duration("timeout", 10*time.Second, "register/heartbeat timeout")
	ttlSeconds := fs.Uint("ttl-seconds", 0, "session ttl in seconds (0 = server default)")
	// Serve-mode flags.
	mockTask := fs.String("mock-task", "", "serve mode: function id to declare and serve")
	serveDuration := fs.Duration("serve-duration", 60*time.Second, "serve mode: max runtime")
	exitAfterTasks := fs.Int("exit-after-tasks", 0, "serve mode: exit once this many StartTask requests are received (0 = only serve-duration)")
	stepMs := fs.Int("step-ms", 60, "serve mode: milliseconds between progress events")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if *agentID == "" {
		return fail(stderr, "-agent-id is required")
	}
	if *stepMs < 0 {
		*stepMs = 0
	}

	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Dial and wrap in a MuxConn so we can both Call (Register/Heartbeat) and
	// receive inbound requests (StartTask/Cancel) on the same session.
	conn, err := tcp.Dial(&tcp.Config{
		Address:        *addr,
		Insecure:       *insecure,
		ConnectTimeout: 5 * time.Second,
	})
	if err != nil {
		return fail(stderr, "dial %s: %v", *addr, err)
	}

	taskCh := make(chan struct{}, 64)
	handler := &probeHandler{
		stepMs:      *stepMs,
		cancelFuncs: make(map[string]context.CancelFunc),
		onTask: func() {
			select {
			case taskCh <- struct{}{}:
			default:
			}
		},
		logger: logger,
	}
	mux := tcp.NewMuxConn(conn, &tcp.Config{RecvTimeout: 30 * time.Second, SendTimeout: 10 * time.Second}, handler)
	handler.send = mux.Send

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	runErrCh := make(chan error, 1)
	go func() { runErrCh <- mux.Run(runCtx) }()

	// 1. Register — must be the first frame on the session. In serve mode we
	//    declare the function so the dispatcher can route tasks to this probe.
	regReq := &agentv1.RegisterRequest{
		AgentId:    *agentID,
		Version:    *version,
		GameId:     *gameID,
		Env:        *env,
		TtlSeconds: uint32(*ttlSeconds),
	}
	if *mockTask != "" {
		// Function descriptor version must be valid semver — the server's
		// validateAndNormalizeFunctions skips non-semver versions, which would
		// leave the dispatcher with no agent for the function.
		regReq.Functions = []*agentv1.FunctionDescriptor{
			{Id: *mockTask, Version: "1.0.0", Enabled: true},
		}
	}
	regBody, err := call(mux, *timeout, protocol.MsgRegisterRequest, regReq, protocol.MsgRegisterResponse)
	if err != nil {
		return fail(stderr, "register: %v", err)
	}
	regResp := &agentv1.RegisterResponse{}
	if err := proto.Unmarshal(regBody, regResp); err != nil {
		return fail(stderr, "unmarshal RegisterResponse: %v", err)
	}
	if regResp.GetSessionId() == "" {
		return fail(stderr, "register returned empty session id")
	}
	fmt.Fprintf(stderr, "registered agent=%s session=%s game=%s env=%s functions=%d\n",
		*agentID, regResp.GetSessionId(), *gameID, *env, len(regReq.Functions))

	// 2. Heartbeat — validates the post-register state machine.
	if !*skipHeartbeat {
		hbReq := &agentv1.HeartbeatRequest{AgentId: *agentID}
		if _, err := call(mux, *timeout, protocol.MsgHeartbeatRequest, hbReq, protocol.MsgHeartbeatResponse); err != nil {
			return fail(stderr, "heartbeat: %v", err)
		}
		fmt.Fprintf(stderr, "heartbeat ok agent=%s\n", *agentID)
	}

	// 3. Handshake mode: done.
	if *mockTask == "" {
		runCancel()
		<-runErrCh
		fmt.Fprintln(stdout, "e2e-agent-probe: PASS")
		return 0
	}

	// 4. Serve mode: stay connected until exitAfterTasks reached or serveDuration.
	fmt.Fprintf(stderr, "serving function=%s exit-after-tasks=%d max=%s step=%dms\n",
		*mockTask, *exitAfterTasks, *serveDuration, *stepMs)

	if err := serveLoop(handler, taskCh, runErrCh, *exitAfterTasks, *serveDuration); err != nil {
		return fail(stderr, "mux run ended early: %v", err)
	}

	// Grace period then drain: wait for any in-flight cancel to arrive and be
	// handled (so the terminal cancelled event is emitted), then let inflight
	// tasks finish. Without the grace, a cancel landing just after the task
	// completes would hit a closing session and the cancelled event is lost.
	time.Sleep(2 * time.Second)
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 3*time.Second)
	handler.waitIdle(drainCtx)
	drainCancel()

	runCancel()
	<-runErrCh
	_ = mux.Close()

	handler.mu.Lock()
	tasks := handler.tasksHandled
	cancels := handler.cancelsHandled
	handler.mu.Unlock()
	fmt.Fprintf(stderr, "served tasks=%d cancels=%d\n", tasks, cancels)
	if tasks == 0 {
		return fail(stderr, "serve mode: no tasks received (dispatcher did not route to this probe)")
	}
	fmt.Fprintln(stdout, "e2e-agent-probe: PASS")
	return 0
}

// serveLoop 是 serve 模式的等待循环：serve-duration 到期、exit-after-tasks
// 达成或会话提前结束时退出。返回非 nil 表示 mux 会话异常结束（调用方 fail）。
func serveLoop(handler *probeHandler, taskCh <-chan struct{}, runErrCh <-chan error, exitAfterTasks int, serveDuration time.Duration) error {
	deadline := time.NewTimer(serveDuration)
	defer deadline.Stop()
servedLoop:
	for {
		select {
		case <-deadline.C:
			break servedLoop
		case <-taskCh:
			handler.mu.Lock()
			n := handler.tasksHandled
			handler.mu.Unlock()
			if exitAfterTasks > 0 && n >= exitAfterTasks {
				break servedLoop
			}
		case err := <-runErrCh:
			if err != nil {
				return err
			}
			break servedLoop
		}
	}
	return nil
}

// call marshals req, sends it as msgID, and asserts the response is respMsgID.
func call(mux *tcp.MuxConn, timeout time.Duration, msgID uint32, req proto.Message, respMsgID uint32) ([]byte, error) {
	body, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", protocol.MsgIDString(msgID), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	gotMsgID, respBody, err := mux.Call(ctx, msgID, body)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", protocol.MsgIDString(msgID), err)
	}
	if gotMsgID != respMsgID {
		return nil, fmt.Errorf("expected %s (0x%06X), got 0x%06X", protocol.MsgIDString(respMsgID), respMsgID, gotMsgID)
	}
	return respBody, nil
}

// probeHandler is the mock agent: it answers StartTask/Cancel and streams
// TaskEvents upstream via the shared MuxConn.
type probeHandler struct {
	stepMs         int
	mu             sync.Mutex
	tasksHandled   int
	cancelsHandled int
	cancelFuncs    map[string]context.CancelFunc
	inflight       sync.WaitGroup
	onTask         func()
	logger         *slog.Logger
	// send 是 TaskEvent 上行通道：生产实现为 mux.Send，测试注入记录器。
	send func(ctx context.Context, msgID uint32, body []byte) error
}

// Handle implements transport.Handler.
func (h *probeHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgStartTaskRequest:
		return h.handleStartTask(body)
	case protocol.MsgCancelTaskRequest:
		return h.handleCancelTask(body)
	default:
		return nil, fmt.Errorf("probe: unsupported msgID %s", protocol.MsgIDString(msgID))
	}
}

func (h *probeHandler) handleStartTask(body []byte) ([]byte, error) {
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeRequest: %w", err)
	}
	taskID := req.GetMetadata()["taskId"]
	if taskID == "" {
		taskID = fmt.Sprintf("probe-%d", time.Now().UnixNano())
	}

	h.mu.Lock()
	h.tasksHandled++
	h.mu.Unlock()
	h.inflight.Add(1)
	if h.onTask != nil {
		h.onTask()
	}

	// Run the mock task asynchronously; the StartTaskResponse is returned
	// synchronously to unblock the dispatcher's Call.
	go h.runMockTask(taskID)

	return proto.Marshal(&sdkv1.StartTaskResponse{TaskId: taskID})
}

func (h *probeHandler) handleCancelTask(body []byte) ([]byte, error) {
	req := &sdkv1.CancelTaskRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal CancelTaskRequest: %w", err)
	}
	h.mu.Lock()
	h.cancelsHandled++
	cancel, ok := h.cancelFuncs[req.GetTaskId()]
	h.mu.Unlock()
	if ok {
		// Cancelling triggers runMockTask's ctx.Done → it emits "cancelled".
		cancel()
	} else {
		// Task already finished; surface the cancellation anyway so the
		// observer sees the terminal state.
		h.sendEvent(req.GetTaskId(), "cancelled", "task already finished", 0, nil)
	}
	// Ack with an empty StartTaskResponse; the dispatcher ignores the body.
	return proto.Marshal(&sdkv1.StartTaskResponse{})
}

func (h *probeHandler) runMockTask(taskID string) {
	defer h.inflight.Done()
	ctx, cancel := context.WithCancel(context.Background())
	h.mu.Lock()
	h.cancelFuncs[taskID] = cancel
	h.mu.Unlock()
	defer func() {
		cancel()
		h.mu.Lock()
		delete(h.cancelFuncs, taskID)
		h.mu.Unlock()
	}()

	h.sendEvent(taskID, "started", "mock task started", 0, nil)
	step := time.Duration(h.stepMs) * time.Millisecond
	progress := 0
	for progress < 100 {
		select {
		case <-ctx.Done():
			h.sendEvent(taskID, "cancelled", "cancelled by request", int32(progress), nil)
			return
		case <-time.After(step):
		}
		// 步长固定 20、从 0 起：progress 恰好停在 100，原 `>100` clamp 为死代码。
		progress += 20
		h.sendEvent(taskID, "progress", "", int32(progress), nil)
	}
	result, _ := json.Marshal(map[string]any{"ok": true, "agent": "e2e-probe"})
	h.sendEvent(taskID, "completed", "mock task completed", 100, result)
}

func (h *probeHandler) sendEvent(taskID, etype, msg string, progress int32, payload []byte) {
	body, err := proto.Marshal(&sdkv1.TaskEvent{
		TaskId:   taskID,
		Type:     etype,
		Message:  msg,
		Progress: progress,
		Payload:  payload,
	})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.send(ctx, protocol.MsgTaskEvent, body); err != nil {
		h.logger.Warn("send task event failed", "task_id", taskID, "type", etype, "error", err)
	}
}

// waitIdle blocks until all inflight tasks finish or ctx expires.
func (h *probeHandler) waitIdle(ctx context.Context) {
	done := make(chan struct{})
	go func() { h.inflight.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// fail 打印诊断并返回退出码 1；真正的进程出口由 main 的 exit 执行。
func fail(stderr io.Writer, format string, args ...any) int {
	fmt.Fprintf(stderr, "e2e-agent-probe: FAIL — "+format+"\n", args...)
	return 1
}
