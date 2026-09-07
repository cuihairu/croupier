package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// gapfixInvokerStub 按 croupier.Invoker 接口注入故障结果。
type gapfixInvokerStub struct {
	connectErr   error
	setSchemaErr error
	invokeResult string
	streamErr    error
	closeErr     error

	connectCalled   bool
	setSchemaCalled bool
	invokeCalled    bool
	closeCalled     bool
}

func (s *gapfixInvokerStub) Connect(ctx context.Context) error {
	s.connectCalled = true
	return s.connectErr
}
func (s *gapfixInvokerStub) Invoke(ctx context.Context, functionID, payload string, options croupier.InvokeOptions) (string, error) {
	s.invokeCalled = true
	return s.invokeResult, nil
}
func (s *gapfixInvokerStub) StartTask(ctx context.Context, functionID, payload string, options croupier.InvokeOptions) (string, error) {
	return "task-gapfix", nil
}
func (s *gapfixInvokerStub) StreamTask(ctx context.Context, taskID string) (<-chan croupier.TaskEvent, error) {
	return nil, s.streamErr
}
func (s *gapfixInvokerStub) CancelTask(ctx context.Context, taskID string) error { return nil }
func (s *gapfixInvokerStub) GetTaskStatus(ctx context.Context, taskID string) (*croupier.TaskStatus, error) {
	return &croupier.TaskStatus{TaskID: taskID, Status: "RUNNING", Progress: 1}, nil
}
func (s *gapfixInvokerStub) SetSchema(functionID string, schema map[string]interface{}) error {
	s.setSchemaCalled = true
	return s.setSchemaErr
}
func (s *gapfixInvokerStub) Close() error {
	s.closeCalled = true
	return s.closeErr
}

func gapfixInjectInvoker(t *testing.T, stub croupier.Invoker) {
	t.Helper()
	orig := newInvoker
	newInvoker = func(*croupier.InvokerConfig) croupier.Invoker { return stub }
	t.Cleanup(func() { newInvoker = orig })
}

// runScenario 必须把 Connect 失败包装为 "connect:" 错误返回。
func TestRunScenarioGapfixConnectFailure(t *testing.T) {
	stub := &gapfixInvokerStub{connectErr: errors.New("dial gapfix refused")}
	gapfixInjectInvoker(t, stub)

	err := runScenario(&croupier.InvokerConfig{Address: "http://127.0.0.1:1/api/v1", TimeoutSeconds: 2})
	if err == nil {
		t.Fatal("expected connect failure to surface")
	}
	if !strings.Contains(err.Error(), "connect") || !strings.Contains(err.Error(), "dial gapfix refused") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stub.connectCalled || !stub.closeCalled {
		t.Fatalf("connect=%v close=%v; both must be exercised", stub.connectCalled, stub.closeCalled)
	}
}

// SetSchema 失败只告警不中断：流程继续并最终成功返回。
func TestRunScenarioGapfixSetSchemaWarningOnly(t *testing.T) {
	stub := &gapfixInvokerStub{setSchemaErr: errors.New("schema gapfix rejected"), invokeResult: `{"status":"success"}`}
	gapfixInjectInvoker(t, stub)

	if err := runScenario(&croupier.InvokerConfig{Address: "http://127.0.0.1:1/api/v1", TimeoutSeconds: 2}); err != nil {
		t.Fatalf("SetSchema failure must only warn, got %v", err)
	}
	if !stub.setSchemaCalled || !stub.invokeCalled {
		t.Fatalf("setSchema=%v invoke=%v; flow must continue past the warning", stub.setSchemaCalled, stub.invokeCalled)
	}
}

// payload 序列化失败必须返回 "marshal payload" 错误。
func TestRunScenarioGapfixMarshalPayloadFailure(t *testing.T) {
	stub := &gapfixInvokerStub{invokeResult: "unused"}
	gapfixInjectInvoker(t, stub)

	origMarshal := jsonMarshal
	jsonMarshal = func(v interface{}) ([]byte, error) { return nil, errors.New("marshal gapfix boom") }
	t.Cleanup(func() { jsonMarshal = origMarshal })

	err := runScenario(&croupier.InvokerConfig{Address: "http://127.0.0.1:1/api/v1", TimeoutSeconds: 2})
	if err == nil {
		t.Fatal("expected marshal failure to surface")
	}
	if !strings.Contains(err.Error(), "marshal payload") || !strings.Contains(err.Error(), "marshal gapfix boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
