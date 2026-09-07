package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// gapfixClientStub 实现最小 Client 契约，用于向 main() 注入故障。
type gapfixClientStub struct {
	registerErr error
	closeErr    error

	registerCalled atomic.Bool
	serveCalled    atomic.Bool
	closeCalled    atomic.Bool
}

func (s *gapfixClientStub) RegisterFunction(desc croupier.FunctionDescriptor, handler croupier.FunctionHandler) error {
	s.registerCalled.Store(true)
	return s.registerErr
}
func (s *gapfixClientStub) Connect(ctx context.Context) error { return nil }
func (s *gapfixClientStub) Serve(ctx context.Context) error {
	s.serveCalled.Store(true)
	<-ctx.Done()
	return nil
}
func (s *gapfixClientStub) Stop() error { return nil }
func (s *gapfixClientStub) Close() error {
	s.closeCalled.Store(true)
	return s.closeErr
}

// main() 的注册失败路径：log.Fatalf → os.Exit，只能在子进程中验证。
func TestBasicMainRegisterFailureExits(t *testing.T) {
	if os.Getenv("EXAMPLE_BASIC_REG_CHILD") == "1" {
		newClient = func(*croupier.ClientConfig) croupier.Client {
			return &gapfixClientStub{registerErr: errors.New("gapfix register rejected")}
		}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestBasicMainRegisterFailureExits")
	cmd.Env = append(os.Environ(), "EXAMPLE_BASIC_REG_CHILD=1")
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("expected child exit status 1, got %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("Failed to register functions")) {
		t.Fatalf("unexpected child output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("gapfix register rejected")) {
		t.Fatalf("injected error must be reported:\n%s", out)
	}
}

// main() 的 Close 失败路径：只记录日志，main 仍正常返回。
func TestBasicMainCloseErrorLogsAndCompletes(t *testing.T) {
	orig := newClient
	stub := &gapfixClientStub{closeErr: errors.New("gapfix close failed")}
	newClient = func(*croupier.ClientConfig) croupier.Client { return stub }
	t.Cleanup(func() { newClient = orig })

	done := make(chan struct{})
	go func() {
		main()
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("main did not return after SIGINT")
	}
	if !stub.registerCalled.Load() || !stub.serveCalled.Load() || !stub.closeCalled.Load() {
		t.Fatalf("register=%v serve=%v close=%v; full lifecycle must run",
			stub.registerCalled.Load(), stub.serveCalled.Load(), stub.closeCalled.Load())
	}
}
