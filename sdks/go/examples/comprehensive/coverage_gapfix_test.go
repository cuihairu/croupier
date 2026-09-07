package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// ---------------------------------------------------------------------------
// Invoker 桩：按 croupier.Invoker 接口注入故障/成功结果
// ---------------------------------------------------------------------------

type gapfixInvokerStub struct {
	connectErr   error
	setSchemaErr error
	invokeResult string
	streamErr    error
	closeErr     error

	connectCalled   atomic.Bool
	setSchemaCalled atomic.Bool
	invokeCalled    atomic.Bool
	streamCalled    atomic.Bool
	closeCalled     atomic.Bool
}

func (s *gapfixInvokerStub) Connect(ctx context.Context) error {
	s.connectCalled.Store(true)
	return s.connectErr
}
func (s *gapfixInvokerStub) Invoke(ctx context.Context, functionID, payload string, options croupier.InvokeOptions) (string, error) {
	s.invokeCalled.Store(true)
	return s.invokeResult, nil
}
func (s *gapfixInvokerStub) StartTask(ctx context.Context, functionID, payload string, options croupier.InvokeOptions) (string, error) {
	return "task-gapfix", nil
}
func (s *gapfixInvokerStub) StreamTask(ctx context.Context, taskID string) (<-chan croupier.TaskEvent, error) {
	s.streamCalled.Store(true)
	return nil, s.streamErr
}
func (s *gapfixInvokerStub) CancelTask(ctx context.Context, taskID string) error { return nil }
func (s *gapfixInvokerStub) GetTaskStatus(ctx context.Context, taskID string) (*croupier.TaskStatus, error) {
	return &croupier.TaskStatus{TaskID: taskID, Status: "RUNNING", Progress: 1}, nil
}
func (s *gapfixInvokerStub) SetSchema(functionID string, schema map[string]interface{}) error {
	s.setSchemaCalled.Store(true)
	return s.setSchemaErr
}
func (s *gapfixInvokerStub) Close() error {
	s.closeCalled.Store(true)
	return s.closeErr
}

func gapfixInjectInvoker(t *testing.T, stub croupier.Invoker) {
	t.Helper()
	orig := newInvoker
	newInvoker = func(*croupier.InvokerConfig) croupier.Invoker { return stub }
	t.Cleanup(func() { newInvoker = orig })
}

// ---------------------------------------------------------------------------
// Client 桩：向 main() 注入 Close/RegisterFunction 故障
// ---------------------------------------------------------------------------

type gapfixClientStub struct {
	registerErr error
	closeErr    error

	registerCalled atomic.Bool
	closeCalled    atomic.Bool
}

func (s *gapfixClientStub) RegisterFunction(desc croupier.FunctionDescriptor, handler croupier.FunctionHandler) error {
	s.registerCalled.Store(true)
	return s.registerErr
}
func (s *gapfixClientStub) Connect(ctx context.Context) error { return nil }
func (s *gapfixClientStub) Serve(ctx context.Context) error   { return nil }
func (s *gapfixClientStub) Stop() error                       { return nil }
func (s *gapfixClientStub) Close() error {
	s.closeCalled.Store(true)
	return s.closeErr
}

// ---------------------------------------------------------------------------
// demonstrateInvokerInterface 缺口分支
// ---------------------------------------------------------------------------

// SetSchema 失败必须让 demonstrateInvokerInterface 返回 "设置schema失败" 错误。
func TestComprehensiveInvokerGapfixSetSchemaFailure(t *testing.T) {
	stub := &gapfixInvokerStub{setSchemaErr: errors.New("gapfix schema rejected")}
	gapfixInjectInvoker(t, stub)

	err := demonstrateInvokerInterface(context.Background())
	if err == nil {
		t.Fatal("expected SetSchema failure to surface")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("设置schema失败")) || !bytes.Contains([]byte(err.Error()), []byte("gapfix schema rejected")) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stub.connectCalled.Load() || !stub.setSchemaCalled.Load() {
		t.Fatalf("connect=%v setSchema=%v; both must be exercised", stub.connectCalled.Load(), stub.setSchemaCalled.Load())
	}
}

// 同步调用成功 + 流式监听失败（仅告警）+ 收尾 Close 失败（返回错误）。
func TestComprehensiveInvokerGapfixInvokeSuccessStreamAndCloseFailure(t *testing.T) {
	stub := &gapfixInvokerStub{
		invokeResult: `{"banned":true}`,
		streamErr:    errors.New("gapfix stream down"),
		closeErr:     errors.New("gapfix close failed"),
	}
	gapfixInjectInvoker(t, stub)

	err := demonstrateInvokerInterface(context.Background())
	if err == nil {
		t.Fatal("expected Close failure to surface")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("关闭调用器失败")) || !bytes.Contains([]byte(err.Error()), []byte("gapfix close failed")) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stub.invokeCalled.Load() || !stub.streamCalled.Load() || !stub.closeCalled.Load() {
		t.Fatalf("invoke=%v stream=%v close=%v; success/failure chain must run",
			stub.invokeCalled.Load(), stub.streamCalled.Load(), stub.closeCalled.Load())
	}
}

// ---------------------------------------------------------------------------
// main() 缺口分支
// ---------------------------------------------------------------------------

// 延迟 Close 失败只记录日志，main 仍完整运行返回。
func TestComprehensiveMainGapfixDeferredCloseErrorLogs(t *testing.T) {
	origClient := newClient
	clientStub := &gapfixClientStub{closeErr: errors.New("gapfix client close failed")}
	newClient = func(*croupier.ClientConfig) croupier.Client { return clientStub }
	t.Cleanup(func() { newClient = origClient })

	invokerStub := &gapfixInvokerStub{connectErr: errors.New("gapfix connect refused")}
	gapfixInjectInvoker(t, invokerStub)

	done := make(chan struct{})
	go func() {
		main()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("main did not complete")
	}
	if !clientStub.registerCalled.Load() || !clientStub.closeCalled.Load() {
		t.Fatalf("register=%v close=%v; deferred Close must run after registration",
			clientStub.registerCalled.Load(), clientStub.closeCalled.Load())
	}
}

// 配置管理演示失败路径：log.Fatalf → os.Exit，只能在子进程中验证。
func TestComprehensiveMainGapfixConfigVariationsFailureExits(t *testing.T) {
	if os.Getenv("EXAMPLE_COMP_CFG_CHILD") == "1" {
		demonstrateConfigurationVariationsFn = func() error {
			return errors.New("gapfix config boom")
		}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestComprehensiveMainGapfixConfigVariationsFailureExits")
	cmd.Env = append(os.Environ(), "EXAMPLE_COMP_CFG_CHILD=1")
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("expected child exit status 1, got %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("配置演示失败")) {
		t.Fatalf("unexpected child output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("gapfix config boom")) {
		t.Fatalf("injected error must be reported:\n%s", out)
	}
}

// 函数注册演示失败路径：log.Fatalf → os.Exit，只能在子进程中验证。
func TestComprehensiveMainGapfixRegistrationFailureExits(t *testing.T) {
	if os.Getenv("EXAMPLE_COMP_REG_CHILD") == "1" {
		newClient = func(*croupier.ClientConfig) croupier.Client {
			return &gapfixClientStub{registerErr: errors.New("gapfix register rejected")}
		}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestComprehensiveMainGapfixRegistrationFailureExits")
	cmd.Env = append(os.Environ(), "EXAMPLE_COMP_REG_CHILD=1")
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("expected child exit status 1, got %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("函数注册演示失败")) {
		t.Fatalf("unexpected child output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("gapfix register rejected")) {
		t.Fatalf("injected error must be reported:\n%s", out)
	}
}
