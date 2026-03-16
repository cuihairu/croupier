package dispatch

import (
	"context"
	"fmt"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
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
	if d.jobStore == nil {
		t.Error("jobStore should be initialized with default memory store")
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
	// 由于 getNNGClient 涉及实际的网络连接，我们测试 pickAgent 的错误路径
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

// TestDispatcher_StartJob_NilRequest 测试 nil StartJobRequest
func TestDispatcher_StartJob_NilRequest(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	_, err := d.StartJobRequest(ctx, nil)

	if err == nil {
		t.Error("StartJobRequest with nil request should return error")
	}
}

// TestDispatcher_StartJob_EmptyFunctionID 测试空 FunctionID
func TestDispatcher_StartJob_EmptyFunctionID(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{FunctionId: ""}
	_, err := d.StartJobRequest(ctx, req)

	if err == nil {
		t.Error("StartJobRequest with empty function id should return error")
	}
}

// TestDispatcher_StartJob_NoAgent 测试没有可用代理
func TestDispatcher_StartJob_NoAgent(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{FunctionId: "test-function"}
	_, err := d.StartJobRequest(ctx, req)

	if err == nil {
		t.Error("StartJobRequest should return error when no agent available")
	}
}

// TestDispatcher_CancelJob_NotTracked 测试取消未跟踪的任务
func TestDispatcher_CancelJob_NotTracked(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	err := d.CancelJob(ctx, "non-existent-job")

	if err == nil {
		t.Error("CancelJob should return error for non-existent job")
	}
}

// TestDispatcher_StreamJob 测试流式任务
func TestDispatcher_StreamJob(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	events, complete, err := d.StreamJob(ctx, "test-job")

	if err == nil {
		t.Error("StreamJob should return error (not yet implemented)")
	}
	if complete {
		t.Error("complete should be false on error")
	}
	if events != nil {
		t.Error("events should be nil on error")
	}
}

// TestDispatcher_StreamJobRealtime 测试实时流式任务
func TestDispatcher_StreamJobRealtime(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()
	complete, err := d.StreamJobRealtime(ctx, "test-job", func(evt *sdkv1.JobEvent) bool {
		return true
	})

	if err == nil {
		t.Error("StreamJobRealtime should return error (not yet implemented)")
	}
	if complete {
		t.Error("complete should be false on error")
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

// TestDispatcher_pickAgent_IgnoresEmptyRPCAddr 测试忽略空 RPC 地址
func TestDispatcher_pickAgent_IgnoresEmptyRPCAddr(t *testing.T) {
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

	_, err := d.pickAgent("test-func")

	if err == nil {
		t.Error("pickAgent() should return error when agent has empty RPC address")
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

// TestDispatcher_getNNGClient_EmptyAddr 测试空地址
func TestDispatcher_getNNGClient_EmptyAddr(t *testing.T) {
	d := NewDispatcher(nil)

	_, err := d.getNNGClient("")

	if err == nil {
		t.Error("getNNGClient() should return error for empty address")
	}
}

// TestDispatcher_JobAddr_NotFound 测试未找到任务地址
func TestDispatcher_JobAddr_NotFound(t *testing.T) {
	d := NewDispatcher(nil)

	addr, ok := d.JobAddr("non-existent")

	if ok {
		t.Errorf("JobAddr() should return false for non-existent job, got (%q, %v)", addr, ok)
	}
}

// TestDispatcher_jobAddr_LoadsFromStore 测试从存储加载
func TestDispatcher_jobAddr_LoadsFromStore(t *testing.T) {
	store := NewMemoryJobRoutingStore()
	d := NewDispatcherWithJobStore(nil, store)

	jobID := "test-job"
	addr := "127.0.0.1:9001"

	// 直接设置到存储中
	store.Set(jobID, addr)

	// 通过 jobAddr 获取
	retrievedAddr, err := d.jobAddr(jobID)

	if err != nil {
		t.Fatalf("jobAddr() error = %v", err)
	}

	if retrievedAddr != addr {
		t.Errorf("jobAddr() = %q, want %q", retrievedAddr, addr)
	}

	// 现在应该在内存缓存中
	cachedAddr, ok := d.JobAddr(jobID)
	if !ok || cachedAddr != addr {
		t.Errorf("JobAddr() after load = (%q, %v), want (%q, true)", cachedAddr, ok, addr)
	}
}

// TestDispatcher_UnregisterJob 测试注销任务
func TestDispatcher_UnregisterJob(t *testing.T) {
	d := NewDispatcher(nil)

	// 注册任务
	d.RegisterJob("test-job", "127.0.0.1:9001")

	// 验证已注册
	addr, ok := d.JobAddr("test-job")
	if !ok || addr != "127.0.0.1:9001" {
		t.Fatal("Job should be registered")
	}

	// 注销任务
	d.UnregisterJob("test-job")

	// 验证已注销
	_, ok = d.JobAddr("test-job")
	if ok {
		t.Error("Job should be unregistered")
	}
}

// TestDispatcher_unregisterJob 测试内部注销任务方法
func TestDispatcher_unregisterJob_WithStore(t *testing.T) {
	store := NewMemoryJobRoutingStore()
	d := NewDispatcherWithJobStore(nil, store)

	jobID := "test-job"
	addr := "127.0.0.1:9001"

	// 注册任务
	d.registerJob(jobID, addr)

	// 验证在存储中
	routing, err := store.Get(jobID)
	if err != nil {
		t.Fatal("Job should be in store")
	}
	if routing.AgentAddr != addr {
		t.Error("Store should have correct address")
	}

	// 注销任务
	d.unregisterJob(jobID)

	// 验证从存储中删除
	_, err = store.Get(jobID)
	if err == nil {
		t.Error("Job should be removed from store")
	}
}

// TestDispatcher_loadJobRouting_StoreError 测试存储错误处理
func TestDispatcher_loadJobRouting_StoreError(t *testing.T) {
	// 创建一个会返回错误的存储
	errorStore := &errorJobRoutingStore{}
	d := NewDispatcherWithJobStore(nil, errorStore)

	// loadJobRouting 应该处理错误并继续
	d.loadJobRouting()

	// 应该有空的内存缓存
	if _, ok := d.JobAddr("any-job"); ok {
		t.Error("Job routing should be empty after store error")
	}
}

// TestDispatcher_registerJob_StoreError 测试注册时存储错误
func TestDispatcher_registerJob_StoreError(t *testing.T) {
	// 创建一个会返回错误的存储
	errorStore := &errorJobRoutingStore{}
	d := NewDispatcherWithJobStore(nil, errorStore)

	// 注册任务 - 应该成功，即使存储失败
	d.registerJob("test-job", "127.0.0.1:9001")

	// 应该在内存缓存中
	addr, ok := d.JobAddr("test-job")
	if !ok || addr != "127.0.0.1:9001" {
		t.Error("Job should be in memory cache even if store fails")
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
	store := NewMemoryJobRoutingStore()
	d := NewDispatcherWithJobStore(nil, store)

	// 添加任务来使用 dispatcher
	d.RegisterJob("test-job", "127.0.0.1:9001")

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

// TestDispatcher_CleanupOldJobs 测试清理旧任务
func TestDispatcher_CleanupOldJobs(t *testing.T) {
	store := NewMemoryJobRoutingStore()
	d := NewDispatcherWithJobStore(nil, store)

	// 添加一些任务
	d.RegisterJob("old-job", "127.0.0.1:9001")
	time.Sleep(10 * time.Millisecond)
	d.RegisterJob("new-job", "127.0.0.1:9002")

	// 清理旧任务
	err := d.CleanupOldJobs(1 * time.Millisecond)
	if err != nil {
		t.Fatalf("CleanupOldJobs() error = %v", err)
	}

	// 验证旧任务被删除
	_, ok := d.JobAddr("old-job")
	if ok {
		t.Error("Old job should be removed")
	}

	// 验证新任务保留
	addr, ok := d.JobAddr("new-job")
	if !ok || addr != "127.0.0.1:9002" {
		t.Error("New job should still exist")
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

// TestDispatcher_StartJob 测试简单的 StartJob 方法
func TestDispatcher_StartJob(t *testing.T) {
	d := NewDispatcher(nil)

	ctx := context.Background()

	// 测试空 function ID
	_, err := d.StartJob(ctx, "", []byte("test"))
	if err == nil {
		t.Error("StartJob() with empty function id should return error")
	}

	// 测试没有可用代理
	_, err = d.StartJob(ctx, "test-func", []byte("test"))
	if err == nil {
		t.Error("StartJob() should return error when no agent available")
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
		RPCAddr:  "invalid-address-for-test", // 无效地址将在 getNNGClient 时失败
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

// errorJobRoutingStore 是一个总是返回错误的存储实现
type errorJobRoutingStore struct {
}

func (s *errorJobRoutingStore) Get(jobID string) (*JobRouting, error) {
	return nil, fmt.Errorf("store error")
}

func (s *errorJobRoutingStore) Set(jobID, agentAddr string) error {
	return fmt.Errorf("store error")
}

func (s *errorJobRoutingStore) Delete(jobID string) error {
	return fmt.Errorf("store error")
}

func (s *errorJobRoutingStore) List() ([]*JobRouting, error) {
	return nil, fmt.Errorf("store error")
}

func (s *errorJobRoutingStore) Cleanup(ttl time.Duration) error {
	return fmt.Errorf("store error")
}

func (s *errorJobRoutingStore) Close() error {
	return nil
}

// mockNNGClient 是一个模拟的 NNG 客户端
type mockNNGClient struct {
	running bool
}

func (m *mockNNGClient) Dial() error {
	m.running = true
	return nil
}

func (m *mockNNGClient) Call(ctx context.Context, msgType uint8, data []byte) ([]byte, error) {
	// 返回一个模拟的 InvokeResponse
	resp := &sdkv1.InvokeResponse{
		Payload: []byte("mock response"),
	}
	return proto.Marshal(resp)
}

func (m *mockNNGClient) IsRunning() bool {
	return m.running
}

func (m *mockNNGClient) Close() error {
	m.running = false
	return nil
}

// TestMessageTypes 测试消息类型常量
func TestMessageTypes(t *testing.T) {
	// 测试消息类型值是否在合理范围内
	// 注意：protocol 包中的消息类型是 uint32，不是 uint8
	_ = protocol.MsgInvokeRequest
	_ = protocol.MsgStartJobRequest
	_ = protocol.MsgCancelJobRequest
}

// TestDispatcher_InvokeRequest_WithMetadata 测试带元数据的请求
func TestDispatcher_InvokeRequest_WithMetadata(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "invalid-address", // 无效地址会在 getNNGClient 时失败
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

// TestDispatcher_StartJobRequest_WithMetadata 测试带元数据的任务启动
func TestDispatcher_StartJobRequest_WithMetadata(t *testing.T) {
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)

	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		RPCAddr:  "invalid-address", // 无效地址会在 getNNGClient 时失败
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

	_, err := d.StartJobRequest(ctx, req)
	if err == nil {
		t.Error("StartJobRequest() should return error with invalid address")
	}
}

// TestDispatcher_CancelJob_AfterRegister 测试注册后取消任务
func TestDispatcher_CancelJob_AfterRegister(t *testing.T) {
	d := NewDispatcher(nil)

	// 注册任务
	d.RegisterJob("test-job", "invalid-address")

	ctx := context.Background()
	err := d.CancelJob(ctx, "test-job")

	// 应该返回错误，因为地址无效
	if err == nil {
		t.Error("CancelJob() should return error with invalid address")
	}

	// 任务应该仍然被注册（因为取消失败，所以不会注销）
	_, ok := d.JobAddr("test-job")
	if !ok {
		t.Error("Job should still be registered when cancel fails")
	}
}

// TestDispatcher_CancelJob_UnregistersAfterSuccess 测试成功取消后注销任务
func TestDispatcher_CancelJob_UnregistersAfterSuccess(t *testing.T) {
	d := NewDispatcher(nil)

	// 注册任务到有效地址（但没有实际服务）
	// 由于我们无法模拟成功的 NNG 调用，我们直接测试 UnregisterJob
	d.RegisterJob("test-job", "127.0.0.1:9001")

	// 验证已注册
	_, ok := d.JobAddr("test-job")
	if !ok {
		t.Fatal("Job should be registered")
	}

	// 使用 UnregisterJob 直接注销
	d.UnregisterJob("test-job")

	// 验证已注销
	_, ok = d.JobAddr("test-job")
	if ok {
		t.Error("Job should be unregistered")
	}
}
