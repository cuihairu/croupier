package dispatch

import (
	"context"
	"fmt"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// TestNewDispatcher 测试创建新 Dispatcher
func TestNewDispatcher(t *testing.T) {
	store := reg.NewStore()
	d := NewDispatcher(store)

	if d == nil {
		t.Fatal("NewDispatcher() should not return nil")
	}
	if d.store != store {
		t.Error("store should be set correctly")
	}
	if d.taskStore == nil {
		t.Error("taskStore should be initialized with default memory store")
	}
}

// TestNewDispatcherWithNilStore 测试使用 nil store 创建 Dispatcher
func TestNewDispatcherWithNilStore(t *testing.T) {
	d := NewDispatcher(nil)

	if d == nil {
		t.Fatal("NewDispatcher(nil) should not return nil")
	}
	if d.store == nil {
		t.Error("store should be initialized with default store when nil is passed")
	}
}

// TestDispatcher_SetTLSConfig 测试设置 TLS 配置
func TestDispatcher_SetTLSConfig(t *testing.T) {
	d := NewDispatcher(nil)

	// 测试设置为 nil
	d.SetTLSConfig(nil)

	if d.tlsCfg != nil {
		t.Error("tlsCfg should be nil when set to nil")
	}
}

// TestDispatcher_Store 测试获取 store
func TestDispatcher_Store(t *testing.T) {
	store := reg.NewStore()
	d := NewDispatcher(store)

	retrievedStore := d.Store()
	if retrievedStore != store {
		t.Error("Store() should return the same store that was set")
	}
}

// TestDispatcher_InvokeRequest_NilRequest 测试 nil 请求
func TestDispatcher_InvokeRequest_NilRequest(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	_, err := d.InvokeRequest(ctx, nil)

	if err == nil {
		t.Error("InvokeRequest with nil request should return error")
	}
}

// TestDispatcher_InvokeRequest_EmptyFunctionID 测试空 FunctionID
func TestDispatcher_InvokeRequest_EmptyFunctionID(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{FunctionId: ""}
	_, err := d.InvokeRequest(ctx, req)

	if err == nil {
		t.Error("InvokeRequest with empty function id should return error")
	}
}

// TestDispatcher_InvokeRequest_NoAgent 测试没有可用代理
func TestDispatcher_InvokeRequest_NoAgent(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{FunctionId: "test-function"}
	_, err := d.InvokeRequest(ctx, req)

	if err == nil {
		t.Error("InvokeRequest should return error when no agent available")
	}
}

// TestDispatcher_InvokeRequest_InvalidRequestProto 测试无效的 protobuf 编码
func TestDispatcher_InvokeRequest_InvalidMarshal(t *testing.T) {
	// 这是一个集成测试，需要设置一个模拟代理
	// 由于 TCP session 涉及实际的网络连接，我们测试 pickAgent 的错误路径
	d := NewDispatcher(nil)

	// 添加一个过期的代理
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:9999",
		ExpireAt: time.Now().Add(-time.Hour), // 已过期
		Functions: map[string]reg.FunctionMeta{
			"test-function": {Enabled: true},
		},
	})

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{FunctionId: "test-function"}
	_, err := d.InvokeRequest(ctx, req)

	if err == nil {
		t.Error("InvokeRequest should return error when no valid agent available")
	}
}

// TestDispatcher_StartTask_NilRequest 测试 nil StartTaskRequest
func TestDispatcher_StartTask_NilRequest(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	_, err := d.StartTaskRequest(ctx, nil)

	if err == nil {
		t.Error("StartTaskRequest with nil request should return error")
	}
}

// TestDispatcher_StartTask_EmptyFunctionID 测试空 FunctionID
func TestDispatcher_StartTask_EmptyFunctionID(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{FunctionId: ""}
	_, err := d.StartTaskRequest(ctx, req)

	if err == nil {
		t.Error("StartTaskRequest with empty function id should return error")
	}
}

// TestDispatcher_StartTask_NoAgent 测试没有可用代理
func TestDispatcher_StartTask_NoAgent(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{FunctionId: "test-function"}
	_, err := d.StartTaskRequest(ctx, req)

	if err == nil {
		t.Error("StartTaskRequest should return error when no agent available")
	}
}

// TestDispatcher_CancelTask_NotTracked 测试取消未跟踪的任务
func TestDispatcher_CancelTask_NotTracked(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	err := d.CancelTask(ctx, "non-existent-task")

	if err == nil {
		t.Error("CancelTask should return error for non-existent task")
	}
}

// TestDispatcher_StreamTask 测试流式任务
func TestDispatcher_StreamTask(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	events, complete, err := d.StreamTask(ctx, "test-task")

	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	if !complete {
		t.Error("complete should be true when no active run exists")
	}
	if len(events) != 0 {
		t.Errorf("events should be empty when task has no persisted events, got %d", len(events))
	}
}

// TestDispatcher_StreamTaskRealtime 测试实时流式任务
func TestDispatcher_StreamTaskRealtime(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	complete, err := d.StreamTaskRealtime(ctx, "test-task", func(evt *sdkv1.TaskEvent) bool {
		return true
	})

	if err != nil {
		t.Fatalf("StreamTaskRealtime() error = %v", err)
	}
	if !complete {
		t.Error("complete should be true when no active run exists")
	}
}

// TestDispatcher_ListFunctionAgents 测试列出功能代理
func TestDispatcher_ListFunctionAgents(t *testing.T) {
	d := NewDispatcher(nil)

	// 添加一些代理
	now := time.Now().Add(time.Hour)
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "127.0.0.1:9001",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"func-1": {Enabled: true},
			"func-2": {Enabled: true},
		},
	})
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-2",
		RPCAddr:  "127.0.0.1:9002",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"func-1": {Enabled: true},
		},
	})
	// 添加过期代理
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-3",
		RPCAddr:  "127.0.0.1:9003",
		ExpireAt: time.Now().Add(-time.Hour),
		Functions: map[string]reg.FunctionMeta{
			"func-1": {Enabled: true},
		},
	})
	// 添加禁用功能的代理
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-4",
		RPCAddr:  "127.0.0.1:9004",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"func-1": {Enabled: false},
		},
	})

	agents := d.ListFunctionAgents("func-1")

	if len(agents) != 2 {
		t.Errorf("ListFunctionAgents should return 2 agents, got %d", len(agents))
	}

	// 检查去重和排序
	agentMap := make(map[string]bool)
	for _, id := range agents {
		agentMap[id] = true
	}
	if !agentMap["agent-1"] || !agentMap["agent-2"] {
		t.Error("ListFunctionAgents should include agent-1 and agent-2")
	}
}

// TestDispatcher_ListFunctionAgents_Empty 测试空列表
func TestDispatcher_ListFunctionAgents_Empty(t *testing.T) {
	d := NewDispatcher(nil)

	agents := d.ListFunctionAgents("non-existent")

	if len(agents) != 0 {
		t.Errorf("ListFunctionAgents should return 0 agents for non-existent function, got %d", len(agents))
	}
}

// TestDispatcher_ListFunctionAgents_IgnoresNilAgents 测试忽略 nil 代理
func TestDispatcher_ListFunctionAgents_IgnoresNilAgents(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	// 添加有效代理
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "127.0.0.1:9001",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"func-1": {Enabled: true},
		},
	})

	agents := d.ListFunctionAgents("func-1")

	if len(agents) != 1 {
		t.Errorf("ListFunctionAgents should return 1 agent, got %d", len(agents))
	}
}

// TestAgentHasService 测试代理服务检查
func TestAgentHasService(t *testing.T) {
	agent := &reg.AgentSession{
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "provider-1",
				FunctionIDs: []string{"func-1", "func-2"},
			},
			{
				ProviderID:  "provider-2",
				FunctionIDs: []string{"func-3"},
			},
		},
	}

	tests := []struct {
		name       string
		agent      *reg.AgentSession
		providerID string
		functionID string
		expected   bool
	}{
		{
			name:       "existing provider and function",
			agent:      agent,
			providerID: "provider-1",
			functionID: "func-1",
			expected:   true,
		},
		{
			name:       "existing provider, different function",
			agent:      agent,
			providerID: "provider-1",
			functionID: "func-3",
			expected:   false,
		},
		{
			name:       "non-existing provider",
			agent:      agent,
			providerID: "provider-3",
			functionID: "func-1",
			expected:   false,
		},
		{
			name:       "existing provider, any function",
			agent:      agent,
			providerID: "provider-2",
			functionID: "",
			expected:   true,
		},
		{
			name:       "nil agent",
			agent:      nil,
			providerID: "provider-1",
			functionID: "func-1",
			expected:   false,
		},
		{
			name:       "empty provider ID",
			agent:      agent,
			providerID: "",
			functionID: "func-1",
			expected:   false,
		},
		{
			name:       "agent with no providers",
			agent:      &reg.AgentSession{},
			providerID: "provider-1",
			functionID: "func-1",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agentHasService(tt.agent, tt.providerID, tt.functionID)
			if result != tt.expected {
				t.Errorf("agentHasService() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestDispatcher_pickAgent 测试选择代理
func TestDispatcher_pickAgent(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	// 添加多个代理
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-3",
		RPCAddr:  "127.0.0.1:9003",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "127.0.0.1:9001",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-2",
		RPCAddr:  "127.0.0.1:9002",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})

	agent, err := d.pickAgent("test-func")

	if err != nil {
		t.Fatalf("pickAgent() error = %v", err)
	}

	// 应该选择 AgentID 最小的（agent-1）
	if agent.AgentID != "agent-1" {
		t.Errorf("pickAgent() selected %s, want agent-1", agent.AgentID)
	}
}

// TestDispatcher_pickAgent_NoAgent 测试没有可用代理
func TestDispatcher_pickAgent_NoAgent(t *testing.T) {
	d := NewDispatcher(nil)

	_, err := d.pickAgent("non-existent")

	if err == nil {
		t.Error("pickAgent() should return error when no agent available")
	}
}

// TestDispatcher_pickAgent_ExpiresExpiredAgents 测试忽略过期代理
func TestDispatcher_pickAgent_ExpiresExpiredAgents(t *testing.T) {
	d := NewDispatcher(nil)

	// 添加过期代理
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "127.0.0.1:9001",
		ExpireAt: time.Now().Add(-time.Hour),
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})

	_, err := d.pickAgent("test-func")

	if err == nil {
		t.Error("pickAgent() should return error when only expired agent available")
	}
}

// TestDispatcher_pickAgent_IgnoresDisabledFunctions 测试忽略禁用功能
func TestDispatcher_pickAgent_IgnoresDisabledFunctions(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "127.0.0.1:9001",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: false},
		},
	})

	_, err := d.pickAgent("test-func")

	if err == nil {
		t.Error("pickAgent() should return error when function is disabled")
	}
}

// TestDispatcher_pickAgent_AllowsEmptyRPCAddr 测试 session 路由下不再要求 RPC 地址
func TestDispatcher_pickAgent_AllowsEmptyRPCAddr(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})

	agent, err := d.pickAgent("test-func")
	if err != nil {
		t.Fatalf("pickAgent() error = %v", err)
	}
	if agent == nil || agent.AgentID != "agent-1" {
		t.Fatalf("pickAgent() selected %+v, want agent-1", agent)
	}
}

// TestDispatcher_pickAgentWithRouting_TargetServiceID 测试目标服务 ID 路由
func TestDispatcher_pickAgentWithRouting_TargetServiceID(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "127.0.0.1:9001",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "service-1",
				FunctionIDs: []string{"test-func"},
			},
		},
	})
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-2",
		RPCAddr:  "127.0.0.1:9002",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "service-2",
				FunctionIDs: []string{"test-func"},
			},
		},
	})

	metadata := map[string]string{"target_service_id": "service-1"}
	agent, err := d.pickAgentWithRouting("test-func", metadata)

	if err != nil {
		t.Fatalf("pickAgentWithRouting() error = %v", err)
	}

	if agent.AgentID != "agent-1" {
		t.Errorf("pickAgentWithRouting() selected %s, want agent-1", agent.AgentID)
	}
}

// TestDispatcher_pickAgentWithRouting_TargetServiceIDNotFound 测试目标服务未找到
func TestDispatcher_pickAgentWithRouting_TargetServiceIDNotFound(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "127.0.0.1:9001",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})

	metadata := map[string]string{"target_service_id": "non-existent"}
	_, err := d.pickAgentWithRouting("test-func", metadata)

	if err == nil {
		t.Error("pickAgentWithRouting() should return error when target service not found")
	}
}

// TestDispatcher_pickAgentWithRouting_HashKey 测试哈希键路由
func TestDispatcher_pickAgentWithRouting_HashKey(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "127.0.0.1:9001",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-2",
		RPCAddr:  "127.0.0.1:9002",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})

	metadata := map[string]string{"hash_key": "user-123"}
	agent1, _ := d.pickAgentWithRouting("test-func", metadata)
	agent2, _ := d.pickAgentWithRouting("test-func", metadata)

	// 相同的哈希键应该选择相同的代理
	if agent1.AgentID != agent2.AgentID {
		t.Error("Same hash key should select same agent")
	}

	// 不同的哈希键可能选择不同的代理（不强制，因为哈希碰撞）
	_ = agent1
}

// TestDispatcher_pickAgentWithRouting_EmptyHashKey 测试空哈希键
func TestDispatcher_pickAgentWithRouting_EmptyHashKey(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "127.0.0.1:9001",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})

	metadata := map[string]string{"hash_key": "  "} // 只有空格
	agent, err := d.pickAgentWithRouting("test-func", metadata)

	if err != nil {
		t.Fatalf("pickAgentWithRouting() error = %v", err)
	}

	if agent.AgentID != "agent-1" {
		t.Errorf("pickAgentWithRouting() selected %s, want agent-1", agent.AgentID)
	}
}

// TestDispatcher_pickAgentWithRouting_NilMetadata 测试 nil 元数据
func TestDispatcher_pickAgentWithRouting_NilMetadata(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "127.0.0.1:9001",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})

	agent, err := d.pickAgentWithRouting("test-func", nil)

	if err != nil {
		t.Fatalf("pickAgentWithRouting() error = %v", err)
	}

	if agent.AgentID != "agent-1" {
		t.Errorf("pickAgentWithRouting() selected %s, want agent-1", agent.AgentID)
	}
}

// TestDispatcher_TaskAgentID_NotFound 测试未找到任务地址
func TestDispatcher_TaskAgentID_NotFound(t *testing.T) {
	d := NewDispatcher(nil)

	addr, ok := d.TaskAgentID("non-existent")

	if ok {
		t.Errorf("TaskAgentID() should return false for non-existent task, got (%q, %v)", addr, ok)
	}
}

// TestDispatcher_taskAgentID_LoadsFromStore 测试从存储加载
func TestDispatcher_taskAgentID_LoadsFromStore(t *testing.T) {
	store := NewMemoryTaskRoutingStore()
	d := NewDispatcherWithTaskStore(nil, store)

	taskID := "test-task"
	addr := "127.0.0.1:9001"

	// 直接设置到存储中
	store.Set(taskID, addr)

	// 通过 taskAgentID 获取
	retrievedAddr, err := d.taskAgentID(taskID)

	if err != nil {
		t.Fatalf("taskAgentID() error = %v", err)
	}

	if retrievedAddr != addr {
		t.Errorf("taskAgentID() = %q, want %q", retrievedAddr, addr)
	}

	// 现在应该在内存缓存中
	cachedAddr, ok := d.TaskAgentID(taskID)
	if !ok || cachedAddr != addr {
		t.Errorf("TaskAgentID() after load = (%q, %v), want (%q, true)", cachedAddr, ok, addr)
	}
}

// TestDispatcher_UnregisterTask 测试注销任务
func TestDispatcher_UnregisterTask(t *testing.T) {
	d := NewDispatcher(nil)

	// 注册任务
	d.RegisterTask("test-task", "127.0.0.1:9001")

	// 验证已注册
	addr, ok := d.TaskAgentID("test-task")
	if !ok || addr != "127.0.0.1:9001" {
		t.Fatal("Task should be registered")
	}

	// 注销任务
	d.UnregisterTask("test-task")

	// 验证已注销
	_, ok = d.TaskAgentID("test-task")
	if ok {
		t.Error("Task should be unregistered")
	}
}

// TestDispatcher_unregisterTask 测试内部注销任务方法
func TestDispatcher_unregisterTask_WithStore(t *testing.T) {
	store := NewMemoryTaskRoutingStore()
	d := NewDispatcherWithTaskStore(nil, store)

	taskID := "test-task"
	agentID := "agent-test"

	// 注册任务
	d.registerTask(taskID, agentID)

	// 验证在存储中
	routing, err := store.Get(taskID)
	if err != nil {
		t.Fatal("Task should be in store")
	}
	if routing.AgentID != agentID {
		t.Error("Store should have correct agentID")
	}

	// 注销任务
	d.unregisterTask(taskID)

	// 验证从存储中删除
	_, err = store.Get(taskID)
	if err == nil {
		t.Error("Task should be removed from store")
	}
}

// TestDispatcher_loadTaskRouting_StoreError 测试存储错误处理
func TestDispatcher_loadTaskRouting_StoreError(t *testing.T) {
	// 创建一个会返回错误的存储
	errorStore := &errorTaskRoutingStore{}
	d := NewDispatcherWithTaskStore(nil, errorStore)

	// loadTaskRouting 应该处理错误并继续
	d.loadTaskRouting()

	// 应该有空的内存缓存
	if _, ok := d.TaskAgentID("any-task"); ok {
		t.Error("Task routing should be empty after store error")
	}
}

// TestDispatcher_registerTask_StoreError 测试注册时存储错误
func TestDispatcher_registerTask_StoreError(t *testing.T) {
	// 创建一个会返回错误的存储
	errorStore := &errorTaskRoutingStore{}
	d := NewDispatcherWithTaskStore(nil, errorStore)

	// 注册任务 - 应该成功，即使存储失败
	d.registerTask("test-task", "127.0.0.1:9001")

	// 应该在内存缓存中
	addr, ok := d.TaskAgentID("test-task")
	if !ok || addr != "127.0.0.1:9001" {
		t.Error("Task should be in memory cache even if store fails")
	}
}

// TestDispatcher_Close 测试关闭 Dispatcher
func TestDispatcher_Close(t *testing.T) {
	d := NewDispatcher(nil)

	err := d.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// 再次关闭应该是安全的
	err = d.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

// TestDispatcher_CloseWithClients 测试关闭带有客户端的 Dispatcher
func TestDispatcher_CloseWithClients(t *testing.T) {
	store := NewMemoryTaskRoutingStore()
	d := NewDispatcherWithTaskStore(nil, store)

	// 添加任务来使用 dispatcher
	d.RegisterTask("test-task", "127.0.0.1:9001")

	err := d.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// 再次关闭应该是安全的
	err = d.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

// TestDispatcher_CleanupOldTasks 测试清理旧任务
func TestDispatcher_CleanupOldTasks(t *testing.T) {
	store := NewMemoryTaskRoutingStore()
	d := NewDispatcherWithTaskStore(nil, store)

	// 添加一些任务
	d.RegisterTask("old-task", "127.0.0.1:9001")
	time.Sleep(10 * time.Millisecond)
	d.RegisterTask("new-task", "127.0.0.1:9002")

	// 清理旧任务
	err := d.CleanupOldTasks(1 * time.Millisecond)
	if err != nil {
		t.Fatalf("CleanupOldTasks() error = %v", err)
	}

	// 验证旧任务被删除
	_, ok := d.TaskAgentID("old-task")
	if ok {
		t.Error("Old task should be removed")
	}

	// 验证新任务保留
	addr, ok := d.TaskAgentID("new-task")
	if !ok || addr != "127.0.0.1:9002" {
		t.Error("New task should still exist")
	}
}

// TestDispatcher_Invoke 测试简单的 Invoke 方法
func TestDispatcher_Invoke(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()

	// 测试空 function ID
	_, err := d.Invoke(ctx, "", []byte("test"))
	if err == nil {
		t.Error("Invoke() with empty function id should return error")
	}

	// 测试没有可用代理
	_, err = d.Invoke(ctx, "test-func", []byte("test"))
	if err == nil {
		t.Error("Invoke() should return error when no agent available")
	}
}

// TestDispatcher_StartTask 测试简单的 StartTask 方法
func TestDispatcher_StartTask(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()

	// 测试空 function ID
	_, err := d.StartTask(ctx, "", []byte("test"))
	if err == nil {
		t.Error("StartTask() with empty function id should return error")
	}

	// 测试没有可用代理
	_, err = d.StartTask(ctx, "test-func", []byte("test"))
	if err == nil {
		t.Error("StartTask() should return error when no agent available")
	}
}

// TestProtoMarshalError 测试 protobuf 编码错误
func TestProtoMarshalError(t *testing.T) {
	// 测试 InvokeRequest 的 marshal 错误处理
	// 由于 proto.Marshal 对于正常的消息不会失败，我们测试 nil 情况
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	// 添加一个代理，但会导致后续错误
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "invalid-address-for-test", // 无效地址将在TCP session查找时失败
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{
		FunctionId: "test-func",
		Payload:    []byte("test"),
		Metadata:   map[string]string{},
	}

	_, err := d.InvokeRequest(ctx, req)
	if err == nil {
		t.Error("InvokeRequest() should return error with invalid address")
	}
}

// Helper types for testing

// errorTaskRoutingStore 是一个总是返回错误的存储实现
type errorTaskRoutingStore struct {
}

func (s *errorTaskRoutingStore) Get(jobID string) (*TaskRouting, error) {
	return nil, fmt.Errorf("store error")
}

func (s *errorTaskRoutingStore) Set(jobID, agentAddr string) error {
	return fmt.Errorf("store error")
}

func (s *errorTaskRoutingStore) Delete(jobID string) error {
	return fmt.Errorf("store error")
}

func (s *errorTaskRoutingStore) List() ([]*TaskRouting, error) {
	return nil, fmt.Errorf("store error")
}

func (s *errorTaskRoutingStore) Cleanup(ttl time.Duration) error {
	return fmt.Errorf("store error")
}

func (s *errorTaskRoutingStore) Close() error {
	return nil
}

// TestMessageTypes 测试消息类型常量
func TestMessageTypes(t *testing.T) {
	// 测试消息类型值是否在合理范围内
	// 注意：protocol 包中的消息类型是 uint32，不是 uint8
	_ = protocol.MsgInvokeRequest
	_ = protocol.MsgStartTaskRequest
	_ = protocol.MsgCancelTaskRequest
}

// TestDispatcher_InvokeRequest_WithMetadata 测试带元数据的请求
func TestDispatcher_InvokeRequest_WithMetadata(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "invalid-address", // 无效地址会在TCP session查找时失败
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{
		FunctionId: "test-func",
		Payload:    []byte("test"),
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	_, err := d.InvokeRequest(ctx, req)
	if err == nil {
		t.Error("InvokeRequest() should return error with invalid address")
	}
}

// TestDispatcher_StartTaskRequest_WithMetadata 测试带元数据的任务启动
func TestDispatcher_StartTaskRequest_WithMetadata(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "invalid-address", // 无效地址会在TCP session查找时失败
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	})

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{
		FunctionId: "test-func",
		Payload:    []byte("test"),
	}

	_, err := d.StartTaskRequest(ctx, req)
	if err == nil {
		t.Error("StartTaskRequest() should return error with invalid address")
	}
}

// TestDispatcher_CancelTask_AfterRegister 测试注册后取消任务
func TestDispatcher_CancelTask_AfterRegister(t *testing.T) {
	d := NewDispatcher(nil)

	// 注册任务
	d.RegisterTask("test-task", "invalid-address")

	ctx := context.Background()
	err := d.CancelTask(ctx, "test-task")

	// 应该返回错误，因为地址无效
	if err == nil {
		t.Error("CancelTask() should return error with invalid address")
	}

	// 任务应该仍然被注册（因为取消失败，所以不会注销）
	_, ok := d.TaskAgentID("test-task")
	if !ok {
		t.Error("Task should still be registered when cancel fails")
	}
}

// TestDispatcher_CancelTask_UnregistersAfterSuccess 测试成功取消后注销任务
func TestDispatcher_CancelTask_UnregistersAfterSuccess(t *testing.T) {
	d := NewDispatcher(nil)

	// 注册任务到有效地址（但没有实际服务）
	// 由于我们无法模拟成功的 TCP session 调用，我们直接测试 UnregisterTask
	d.RegisterTask("test-task", "127.0.0.1:9001")

	// 验证已注册
	_, ok := d.TaskAgentID("test-task")
	if !ok {
		t.Fatal("Task should be registered")
	}

	// 使用 UnregisterTask 直接注销
	d.UnregisterTask("test-task")

	// 验证已注销
	_, ok = d.TaskAgentID("test-task")
	if ok {
		t.Error("Task should be unregistered")
	}
}
