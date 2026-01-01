package edge

import (
	"context"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	jobv1 "github.com/cuihairu/croupier/pkg/pb/croupier/edge/job/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestNew 测试创建 App
func TestNew(t *testing.T) {
	registry := reg.NewStore()
	app := New(registry)

	if app == nil {
		t.Fatal("New() should return non-nil app")
	}

	if app.ctrl == nil {
		t.Error("ctrl should not be nil")
	}

	if app.dispatcher == nil {
		t.Error("dispatcher should not be nil")
	}

	if app.tunnelStats == nil {
		t.Error("tunnelStats should be initialized")
	}
}

// TestNewWithNilRegistry 测试 nil registry
func TestNewWithNilRegistry(t *testing.T) {
	app := New(nil)

	if app == nil {
		t.Fatal("New(nil) should return non-nil app")
	}

	if app.ctrl == nil {
		t.Error("ctrl should not be nil even with nil registry")
	}
}

// TestNewWithJobStore 测试带 JobStore 创建
func TestNewWithJobStore(t *testing.T) {
	registry := reg.NewStore()
	app := NewWithJobStore(registry, nil)

	if app == nil {
		t.Fatal("NewWithJobStore() should return non-nil app")
	}

	if app.ctrl == nil {
		t.Error("ctrl should not be nil")
	}

	if app.dispatcher == nil {
		t.Error("dispatcher should not be nil")
	}
}

// TestApp_Dispatcher 测试获取 Dispatcher
func TestApp_Dispatcher(t *testing.T) {
	registry := reg.NewStore()
	app := New(registry)

	dispatcher := app.Dispatcher()

	if dispatcher == nil {
		t.Error("Dispatcher() should return non-nil dispatcher")
	}

	if dispatcher != app.dispatcher {
		t.Error("Dispatcher() should return app's dispatcher")
	}
}

// TestApp_Dispatcher_NilApp 测试 nil app 的 Dispatcher
func TestApp_Dispatcher_NilApp(t *testing.T) {
	var app *App
	dispatcher := app.Dispatcher()

	if dispatcher != nil {
		t.Error("Dispatcher() on nil app should return nil")
	}
}

// TestApp_SetUpstreamControlClient 测试设置上游控制客户端
func TestApp_SetUpstreamControlClient(t *testing.T) {
	registry := reg.NewStore()
	app := New(registry)

	// nil 客户端不应该 panic
	app.SetUpstreamControlClient(nil)

	// nil app 不应该 panic
	var nilApp *App
	nilApp.SetUpstreamControlClient(nil)
}

// TestApp_CleanupOldJobs 测试清理旧作业
func TestApp_CleanupOldJobs(t *testing.T) {
	registry := reg.NewStore()
	app := New(registry)

	// 清理作业不应该返回错误
	err := app.CleanupOldJobs(time.Hour)
	if err != nil {
		t.Errorf("CleanupOldJobs() should not return error, got %v", err)
	}
}

// TestApp_CleanupOldJobs_NilApp 测试 nil app 的清理
func TestApp_CleanupOldJobs_NilApp(t *testing.T) {
	var app *App
	err := app.CleanupOldJobs(time.Hour)

	if err != nil {
		t.Errorf("CleanupOldJobs() on nil app should return nil, got %v", err)
	}
}

// TestApp_MetricsMap 测试获取指标
func TestApp_MetricsMap(t *testing.T) {
	registry := reg.NewStore()
	app := New(registry)

	metrics := app.MetricsMap()

	if metrics == nil {
		t.Fatal("MetricsMap() should return non-nil map")
	}

	// 检查默认值
	if reconnects, ok := metrics["tunnel_reconnects"]; !ok {
		t.Error("MetricsMap should contain 'tunnel_reconnects'")
	} else if reconnects != int64(0) {
		t.Errorf("Default tunnel_reconnects should be 0, got %v", reconnects)
	}

	if agents, ok := metrics["active_agents"]; !ok {
		t.Error("MetricsMap should contain 'active_agents'")
	} else if agents != 0 {
		t.Errorf("Default active_agents should be 0, got %v", agents)
	}
}

// TestFunctionServer_Invoke 测试函数调用
func TestFunctionServer_Invoke(t *testing.T) {
	registry := reg.NewStore()
	app := New(registry)
	server := &FunctionServer{dispatcher: app.dispatcher}

	ctx := context.Background()
	req := &functionv1.InvokeRequest{
		FunctionId: "test-func",
		Payload:    []byte("test payload"),
	}

	// 由于没有实际的 agent，预期会失败
	_, err := server.Invoke(ctx, req)

	if err == nil {
		t.Error("Invoke() should return error when no agents available")
	}

	// 错误可能是任何类型，只验证它存在
	t.Logf("Expected error received: %v", err)
}

// TestFunctionServer_Invoke_NilDispatcher 测试 nil dispatcher
func TestFunctionServer_Invoke_NilDispatcher(t *testing.T) {
	server := &FunctionServer{dispatcher: nil}

	ctx := context.Background()
	req := &functionv1.InvokeRequest{}

	_, err := server.Invoke(ctx, req)

	if err == nil {
		t.Error("Invoke() with nil dispatcher should return error")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unavailable {
		t.Errorf("Error should be Unavailable, got %v", err)
	}
}

// TestFunctionServer_StartJob 测试启动作业
func TestFunctionServer_StartJob(t *testing.T) {
	registry := reg.NewStore()
	app := New(registry)
	server := &FunctionServer{dispatcher: app.dispatcher}

	ctx := context.Background()
	req := &functionv1.InvokeRequest{
		FunctionId: "test-func",
		Payload:    []byte("test payload"),
	}

	// 由于没有实际的 agent，预期会失败
	_, err := server.StartJob(ctx, req)

	if err == nil {
		t.Error("StartJob() should return error when no agents available")
	}
}

// TestFunctionServer_CancelJob 测试取消作业
func TestFunctionServer_CancelJob(t *testing.T) {
	registry := reg.NewStore()
	app := New(registry)
	server := &FunctionServer{dispatcher: app.dispatcher}

	ctx := context.Background()
	req := &functionv1.CancelJobRequest{
		JobId: "nonexistent-job",
	}

	resp, err := server.CancelJob(ctx, req)

	// 由于作业不存在，预期会失败
	if err == nil {
		t.Error("CancelJob() should return error for nonexistent job")
	}

	if resp != nil && resp.JobId != req.JobId {
		t.Errorf("Response JobId should match request, got %q", resp.JobId)
	}
}

// TestJobServer_GetJobResult 测试获取作业结果
func TestJobServer_GetJobResult(t *testing.T) {
	registry := reg.NewStore()
	app := New(registry)
	server := &JobServer{dispatcher: app.dispatcher}

	ctx := context.Background()
	req := &jobv1.GetJobResultRequest{
		JobId: "nonexistent-job",
	}

	// 由于作业不存在，预期会失败
	_, err := server.GetJobResult(ctx, req)

	if err == nil {
		t.Error("GetJobResult() should return error for nonexistent job")
	}
}

// TestJobServer_GetJobResult_NilDispatcher 测试 nil dispatcher
func TestJobServer_GetJobResult_NilDispatcher(t *testing.T) {
	server := &JobServer{dispatcher: nil}

	ctx := context.Background()
	req := &jobv1.GetJobResultRequest{}

	_, err := server.GetJobResult(ctx, req)

	if err == nil {
		t.Error("GetJobResult() with nil dispatcher should return error")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unavailable {
		t.Errorf("Error should be Unavailable, got %v", err)
	}
}

// TestCloneInvokeRequest 测试克隆请求
func TestCloneInvokeRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *functionv1.InvokeRequest
	}{
		{
			name: "完整请求",
			req: &functionv1.InvokeRequest{
				FunctionId:     "test-func",
				IdempotencyKey: "key-123",
				Payload:        []byte("payload"),
				Metadata:       map[string]string{"key": "value"},
			},
		},
		{
			name: "空请求",
			req:  &functionv1.InvokeRequest{},
		},
		{
			name: "nil 请求",
			req:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloned := cloneInvokeRequest(tt.req)

			if cloned == nil {
				t.Error("cloneInvokeRequest() should never return nil")
			}

			if tt.req != nil && tt.req.FunctionId != "" {
				if cloned.FunctionId != tt.req.FunctionId {
					t.Errorf("FunctionId = %q, want %q", cloned.FunctionId, tt.req.FunctionId)
				}
			}
		})
	}
}

// TestJobState 测试作业状态
func TestJobState(t *testing.T) {
	tests := []struct {
		name     string
		events   []*functionv1.JobEvent
		done     bool
		expected string
	}{
		{
			name:     "运行中",
			events:   []*functionv1.JobEvent{},
			done:     false,
			expected: "running",
		},
		{
			name:     "已完成",
			events:   []*functionv1.JobEvent{},
			done:     true,
			expected: "completed",
		},
		{
			name: "成功状态",
			events: []*functionv1.JobEvent{
				{Type: "DONE"},
			},
			done:     true,
			expected: "done",
		},
		{
			name: "错误状态",
			events: []*functionv1.JobEvent{
				{Type: "ERROR"},
			},
			done:     true,
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := jobState(tt.events, tt.done)
			if state != tt.expected {
				t.Errorf("jobState() = %q, want %q", state, tt.expected)
			}
		})
	}
}

// TestLastPayload 测试获取最后一个 payload
func TestLastPayload(t *testing.T) {
	tests := []struct {
		name     string
		events   []*functionv1.JobEvent
		expected []byte
	}{
		{
			name:     "空事件",
			events:   []*functionv1.JobEvent{},
			expected: nil,
		},
		{
			name: "有 payload",
			events: []*functionv1.JobEvent{
				{Payload: []byte("first")},
				{Payload: []byte("last")},
			},
			expected: []byte("last"),
		},
		{
			name: "中间空 payload",
			events: []*functionv1.JobEvent{
				{Payload: []byte("first")},
				{Payload: nil},
				{Payload: []byte("last")},
			},
			expected: []byte("last"),
		},
		{
			name: "只有空 payload",
			events: []*functionv1.JobEvent{
				{Payload: nil},
				{Payload: []byte{}},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := lastPayload(tt.events)
			if string(payload) != string(tt.expected) {
				t.Errorf("lastPayload() = %q, want %q", payload, tt.expected)
			}
		})
	}
}

// TestToJobFrame 测试转换为 JobEventFrame
func TestToJobFrame(t *testing.T) {
	tests := []struct {
		name  string
		jobID string
		evt   *functionv1.JobEvent
	}{
		{
			name:  "完整事件",
			jobID: "job-123",
			evt: &functionv1.JobEvent{
				Type:     "PROGRESS",
				Message:  "processing",
				Progress: 50,
				Payload:  []byte("data"),
			},
		},
		{
			name:  "nil 事件",
			jobID: "job-456",
			evt:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := toJobFrame(tt.jobID, tt.evt)

			if tt.evt == nil {
				if frame != nil {
					t.Error("toJobFrame() with nil event should return nil")
				}
				return
			}

			if frame == nil {
				t.Fatal("toJobFrame() should return non-nil frame")
			}

			if frame.JobId != tt.jobID {
				t.Errorf("JobId = %q, want %q", frame.JobId, tt.jobID)
			}

			if frame.Type != tt.evt.Type {
				t.Errorf("Type = %q, want %q", frame.Type, tt.evt.Type)
			}
		})
	}
}

// BenchmarkNew 性能基准测试
func BenchmarkNew(b *testing.B) {
	registry := reg.NewStore()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New(registry)
	}
}

// BenchmarkMetricsMap 性能基准测试
func BenchmarkMetricsMap(b *testing.B) {
	registry := reg.NewStore()
	app := New(registry)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.MetricsMap()
	}
}

// BenchmarkCloneInvokeRequest 性能基准测试
func BenchmarkCloneInvokeRequest(b *testing.B) {
	req := &functionv1.InvokeRequest{
		FunctionId:     "test-func",
		IdempotencyKey: "key-123",
		Payload:        make([]byte, 1024),
		Metadata:       map[string]string{"key": "value"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloneInvokeRequest(req)
	}
}
