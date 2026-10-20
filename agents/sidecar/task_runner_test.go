package sidecar

import (
	"context"
	"sync"
	"testing"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// recordingReporter 捕获上报事件（随迁用例的最小替身；internal 侧另有完整
// mockTaskEventReporter 服务 LocalHandler 用例）。
type recordingReporter struct {
	mu     sync.Mutex
	events []*sdkv1.TaskEvent
}

func (m *recordingReporter) ReportTaskEvent(_ context.Context, event *sdkv1.TaskEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *recordingReporter) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func TestMarshalTaskInvokeV9(t *testing.T) {
	assert.Nil(t, marshalTaskInvoke(nil))

	req := &sdkv1.InvokeRequest{FunctionId: "fn.v9", Payload: []byte(`{}`)}
	data := marshalTaskInvoke(req)
	require.NotNil(t, data)

	decoded := &sdkv1.InvokeRequest{}
	require.NoError(t, proto.Unmarshal(data, decoded))
	assert.Equal(t, "fn.v9", decoded.GetFunctionId())
}

// 覆盖率补洞（group L）：task_runner.go marshalTaskInvoke 的 Marshal 失败分支
// （proto3 string 含无效 UTF-8 必败）。
func TestMarshalTaskInvoke_NilAndInvalidUTF8(t *testing.T) {
	assert.Nil(t, marshalTaskInvoke(nil))
	assert.Nil(t, marshalTaskInvoke(&sdkv1.InvokeRequest{
		FunctionId: "demo.ping",
		Metadata:   map[string]string{"k": "\xff\xfe"},
	}))

	data := marshalTaskInvoke(&sdkv1.InvokeRequest{FunctionId: "demo.ok"})
	require.NotNil(t, data)
	var back sdkv1.InvokeRequest
	require.NoError(t, proto.Unmarshal(data, &back))
	assert.Equal(t, "demo.ok", back.GetFunctionId())
}

func TestTaskRunnerReportEmptyPayloadV9(t *testing.T) {
	r := NewTaskRunner(func(context.Context, *sdkv1.InvokeRequest) ([]byte, error) {
		return nil, nil
	}, nil, nil)
	require.NoError(t, r.report(context.Background(), &sdkv1.TaskEvent{}))

	reporter := &recordingReporter{}
	r.SetReporter(reporter)
	event := &sdkv1.TaskEvent{TaskId: "t-v9"}
	require.NoError(t, r.report(context.Background(), event))
	assert.Equal(t, []byte("null"), event.Payload)
	assert.Equal(t, 1, reporter.count())
}
