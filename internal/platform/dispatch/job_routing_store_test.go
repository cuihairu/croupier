package dispatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

func TestFileJobRoutingStore_PersistsAcrossInstances(t *testing.T) {
	dataDir := t.TempDir()

	store1, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore: %v", err)
	}

	if err := store1.Set("job-1", "127.0.0.1:9001"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	store2, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore (second): %v", err)
	}

	routing, err := store2.Get("job-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got, want := routing.AgentAddr, "127.0.0.1:9001"; got != want {
		t.Fatalf("AgentAddr=%q want %q", got, want)
	}
}

func TestFileJobRoutingStore_Cleanup(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore: %v", err)
	}

	if err := store.Set("job-old", "127.0.0.1:9999"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := store.Cleanup(1 * time.Millisecond); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	routings, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := len(routings); got != 0 {
		t.Fatalf("len(List)=%d want 0", got)
	}
}

func TestDispatcher_LoadsJobRoutingFromStore(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore: %v", err)
	}

	if err := store.Set("job-2", "127.0.0.1:9002"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	d := NewDispatcherWithJobStore(nil, store)
	if got, ok := d.JobAddr("job-2"); !ok || got != "127.0.0.1:9002" {
		t.Fatalf("JobAddr(job-2)=(%q,%v) want (%q,true)", got, ok, "127.0.0.1:9002")
	}
}

func TestDispatcher_CleanupOldJobsClearsMemoryCache(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore: %v", err)
	}

	d := NewDispatcherWithJobStore(nil, store)

	d.RegisterJob("job-old", "127.0.0.1:9009")
	time.Sleep(10 * time.Millisecond)

	if err := d.CleanupOldJobs(1 * time.Millisecond); err != nil {
		t.Fatalf("CleanupOldJobs: %v", err)
	}

	if addr, ok := d.JobAddr("job-old"); ok {
		t.Fatalf("JobAddr(job-old)=(%q,%v) want (_,false)", addr, ok)
	}
}

// TestMemoryJobRoutingStore_BasicOperations 测试内存存储的基本操作
func TestMemoryJobRoutingStore_BasicOperations(t *testing.T) {
	store := NewMemoryJobRoutingStore()

	// 测试 Set 和 Get
	err := store.Set("job-1", "127.0.0.1:9001")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	routing, err := store.Get("job-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if routing.JobID != "job-1" {
		t.Errorf("JobID = %q, want 'job-1'", routing.JobID)
	}
	if routing.AgentAddr != "127.0.0.1:9001" {
		t.Errorf("AgentAddr = %q, want '127.0.0.1:9001'", routing.AgentAddr)
	}
}

// TestMemoryJobRoutingStore_GetNotFound 测试获取不存在的任务
func TestMemoryJobRoutingStore_GetNotFound(t *testing.T) {
	store := NewMemoryJobRoutingStore()

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("Get() should return error for nonexistent job")
	}
}

// TestMemoryJobRoutingStore_UpdatePreservesCreatedAt 测试更新保留创建时间
func TestMemoryJobRoutingStore_UpdatePreservesCreatedAt(t *testing.T) {
	store := NewMemoryJobRoutingStore()

	_ = store.Set("job-1", "127.0.0.1:9001")
	time.Sleep(10 * time.Millisecond)

	_ = store.Set("job-1", "127.0.0.1:9002")

	routing, _ := store.Get("job-1")
	originalCreatedAt := routing.CreatedAt

	time.Sleep(10 * time.Millisecond)
	_ = store.Set("job-1", "127.0.0.1:9003")

	routing, _ = store.Get("job-1")
	if routing.CreatedAt != originalCreatedAt {
		t.Error("CreatedAt should be preserved across updates")
	}
	if routing.AgentAddr != "127.0.0.1:9003" {
		t.Errorf("AgentAddr = %q, want '127.0.0.1:9003'", routing.AgentAddr)
	}
}

// TestMemoryJobRoutingStore_Delete 测试删除任务路由
func TestMemoryJobRoutingStore_Delete(t *testing.T) {
	store := NewMemoryJobRoutingStore()

	_ = store.Set("job-1", "127.0.0.1:9001")
	_, err := store.Get("job-1")
	if err != nil {
		t.Fatal("Job should exist before delete")
	}

	err = store.Delete("job-1")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Get("job-1")
	if err == nil {
		t.Error("Job should not exist after delete")
	}
}

// TestMemoryJobRoutingStore_List 测试列出所有任务路由
func TestMemoryJobRoutingStore_List(t *testing.T) {
	store := NewMemoryJobRoutingStore()

	_ = store.Set("job-1", "127.0.0.1:9001")
	_ = store.Set("job-2", "127.0.0.1:9002")
	_ = store.Set("job-3", "127.0.0.1:9003")

	list, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 3 {
		t.Errorf("List() returned %d items, want 3", len(list))
	}
}

// TestMemoryJobRoutingStore_ListEmpty 测试列出空存储
func TestMemoryJobRoutingStore_ListEmpty(t *testing.T) {
	store := NewMemoryJobRoutingStore()

	list, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 0 {
		t.Errorf("List() returned %d items, want 0", len(list))
	}
}

// TestMemoryJobRoutingStore_Cleanup 测试清理过期任务
func TestMemoryJobRoutingStore_Cleanup(t *testing.T) {
	store := NewMemoryJobRoutingStore()

	_ = store.Set("old-job", "127.0.0.1:9999")
	time.Sleep(10 * time.Millisecond)

	_ = store.Set("new-job", "127.0.0.1:9001")

	err := store.Cleanup(1 * time.Millisecond)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// old-job 应该被清理
	_, err = store.Get("old-job")
	if err == nil {
		t.Error("old-job should be cleaned up")
	}

	// new-job 应该仍然存在
	_, err = store.Get("new-job")
	if err != nil {
		t.Error("new-job should still exist")
	}
}

// TestMemoryJobRoutingStore_CleanupAll 测试清理所有任务
func TestMemoryJobRoutingStore_CleanupAll(t *testing.T) {
	store := NewMemoryJobRoutingStore()

	_ = store.Set("job-1", "127.0.0.1:9001")
	_ = store.Set("job-2", "127.0.0.1:9002")
	time.Sleep(10 * time.Millisecond)

	_ = store.Cleanup(1 * time.Millisecond)

	list, _ := store.List()
	if len(list) != 0 {
		t.Errorf("After cleanup, List() should return 0 items, got %d", len(list))
	}
}

// TestMemoryJobRoutingStore_Close 测试关闭存储
func TestMemoryJobRoutingStore_Close(t *testing.T) {
	store := NewMemoryJobRoutingStore()

	err := store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// 多次关闭应该是安全的
	err = store.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

// TestFileJobRoutingStore_EmptyDirectory 测试空目录
func TestFileJobRoutingStore_EmptyDirectory(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore() error = %v", err)
	}

	// 空目录应该创建空存储
	list, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 0 {
		t.Errorf("Empty store should have 0 items, got %d", len(list))
	}
}

// TestFileJobRoutingStore_GetNotFound 测试文件存储获取不存在的任务
func TestFileJobRoutingStore_GetNotFound(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore() error = %v", err)
	}

	_, err = store.Get("nonexistent")
	if err == nil {
		t.Error("Get() should return error for nonexistent job")
	}
}

// TestFileJobRoutingStore_Delete 测试文件存储删除
func TestFileJobRoutingStore_Delete(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore() error = %v", err)
	}

	_ = store.Set("job-1", "127.0.0.1:9001")
	err = store.Delete("job-1")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// 验证文件被删除
	filePath := filepath.Join(dataDir, "job_routing.json")
	if _, err := os.Stat(filePath); err == nil {
		// 文件可能存在但是空的（空 map）
		data, _ := os.ReadFile(filePath)
		if len(data) > 10 { // 至少有 "{}"
			t.Error("Job routing file should be empty or deleted")
		}
	}
}

// TestFileJobRoutingStore_CleanupNoOldEntries 测试清理时没有旧条目
func TestFileJobRoutingStore_CleanupNoOldEntries(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore() error = %v", err)
	}

	_ = store.Set("job-1", "127.0.0.1:9001")

	// 清理一个非常短的 TTL，应该删除所有条目
	time.Sleep(10 * time.Millisecond)
	err = store.Cleanup(1 * time.Millisecond)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	list, _ := store.List()
	if len(list) != 0 {
		t.Errorf("After cleanup, all entries should be removed, got %d", len(list))
	}
}

// TestFileJobRoutingStore_Close 测试关闭文件存储
func TestFileJobRoutingStore_Close(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore() error = %v", err)
	}

	err = store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestPickAgentByHash 测试哈希选择代理
func TestPickAgentByHash(t *testing.T) {
	agents := []*reg.AgentSession{
		{AgentID: "agent-1", RPCAddr: "127.0.0.1:9001"},
		{AgentID: "agent-2", RPCAddr: "127.0.0.1:9002"},
		{AgentID: "agent-3", RPCAddr: "127.0.0.1:9003"},
	}

	// 相同的 key 应该选择相同的 agent
	agent1 := pickAgentByHash(agents, "user-123")
	agent2 := pickAgentByHash(agents, "user-123")
	agent3 := pickAgentByHash(agents, "user-456")

	if agent1.AgentID != agent2.AgentID {
		t.Error("Same key should select same agent")
	}

	// 不同的 key 可能选择不同的 agent
	// 注意：哈希碰撞是可能的，所以这里不强制要求不同
	_ = agent3
}

// TestPickAgentByHash_EmptyCandidates 测试空候选列表
func TestPickAgentByHash_EmptyCandidates(t *testing.T) {
	var agents []*reg.AgentSession

	agent := pickAgentByHash(agents, "any-key")
	if agent != nil {
		t.Error("Should return nil for empty candidates")
	}
}

// TestPickAgentByHash_SingleCandidate 测试单个候选
func TestPickAgentByHash_SingleCandidate(t *testing.T) {
	agents := []*reg.AgentSession{
		{AgentID: "agent-1", RPCAddr: "127.0.0.1:9001"},
	}

	agent := pickAgentByHash(agents, "any-key")
	if agent == nil {
		t.Fatal("Should return agent for single candidate")
	}
	if agent.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want 'agent-1'", agent.AgentID)
	}
}

// TestPickAgentByHash_EmptyKey 测试空 key
func TestPickAgentByHash_EmptyKey(t *testing.T) {
	agents := []*reg.AgentSession{
		{AgentID: "agent-1", RPCAddr: "127.0.0.1:9001"},
		{AgentID: "agent-2", RPCAddr: "127.0.0.1:9002"},
	}

	agent := pickAgentByHash(agents, "")
	if agent == nil {
		t.Fatal("Should return first agent for empty key")
	}
	// 空 key 应该返回第一个
	if agent.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want 'agent-1'", agent.AgentID)
	}
}

// TestHostFromAddr 测试从地址提取主机
func TestHostFromAddr(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected string
	}{
		{"IPv4", "127.0.0.1:9001", "127.0.0.1"},
		{"IPv6", "[::1]:9001", "::1"},
		{"IPv6 full", "[2001:db8::1]:9001", "2001:db8::1"},
		{"只有主机", "localhost", "localhost"},
		{"只有端口号", ":9001", ""},
		{"空字符串", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hostFromAddr(tt.addr)
			if result != tt.expected {
				t.Errorf("hostFromAddr(%q) = %q, want %q", tt.addr, result, tt.expected)
			}
		})
	}
}

// TestIsTerminalEvent 测试判断事件是否为终止事件
func TestIsTerminalEvent(t *testing.T) {
	tests := []struct {
		name     string
		eventType string
		expected bool
	}{
		{"done", "done", true},
		{"completed", "completed", true},
		{"error", "error", true},
		{"failed", "failed", true},
		{"cancelled", "cancelled", true},
		{"canceled", "canceled", true},
		{"succeeded", "succeeded", true},
		{"success", "success", true},
		{"DONE", "DONE", true},
		{"progress", "progress", false},
		{"log", "log", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &functionv1.JobEvent{Type: tt.eventType}
			result := isTerminalEvent(evt)
			if result != tt.expected {
				t.Errorf("isTerminalEvent(%q) = %v, want %v", tt.eventType, result, tt.expected)
			}
		})
	}
}

// BenchmarkMemoryJobRoutingStore_Set 性能基准测试
func BenchmarkMemoryJobRoutingStore_Set(b *testing.B) {
	store := NewMemoryJobRoutingStore()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Set("job-id", "127.0.0.1:9001")
	}
}

// BenchmarkMemoryJobRoutingStore_Get 性能基准测试
func BenchmarkMemoryJobRoutingStore_Get(b *testing.B) {
	store := NewMemoryJobRoutingStore()
	_ = store.Set("job-id", "127.0.0.1:9001")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get("job-id")
	}
}

// BenchmarkPickAgentByHash 性能基准测试
func BenchmarkPickAgentByHash(b *testing.B) {
	agents := make([]*reg.AgentSession, 100)
	for i := 0; i < 100; i++ {
		agents[i] = &reg.AgentSession{
			AgentID: string(rune('a' + i)),
			RPCAddr: "127.0.0.1:9001",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pickAgentByHash(agents, "test-key")
	}
}

