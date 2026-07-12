package dispatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

func TestFileTaskRoutingStore_PersistsAcrossInstances(t *testing.T) {
	dataDir := t.TempDir()

	store1, err := NewFileTaskRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileTaskRoutingStore: %v", err)
	}

	if err := store1.Set("task-1", "agent-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	store2, err := NewFileTaskRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileTaskRoutingStore (second): %v", err)
	}

	routing, err := store2.Get("task-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got, want := routing.AgentID, "agent-1"; got != want {
		t.Fatalf("AgentID=%q want %q", got, want)
	}
}

func TestFileTaskRoutingStore_Cleanup(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewFileTaskRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileTaskRoutingStore: %v", err)
	}

	if err := store.Set("task-old", "agent-old"); err != nil {
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

func TestDispatcher_LoadsTaskRoutingFromStore(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewFileTaskRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileTaskRoutingStore: %v", err)
	}

	if err := store.Set("task-2", "agent-2"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	d := NewDispatcherWithTaskStore(nil, store, nil)
	if got, ok := d.TaskAgentID("task-2"); !ok || got != "agent-2" {
		t.Fatalf("TaskAgentID(task-2)=(%q,%v) want (%q,true)", got, ok, "agent-2")
	}
}

func TestDispatcher_CleanupOldTasksClearsMemoryCache(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewFileTaskRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileTaskRoutingStore: %v", err)
	}

	d := NewDispatcherWithTaskStore(nil, store, nil)

	d.RegisterTask("task-old", "agent-old")
	time.Sleep(10 * time.Millisecond)

	if err := d.CleanupOldTasks(1 * time.Millisecond); err != nil {
		t.Fatalf("CleanupOldTasks: %v", err)
	}

	if agentID, ok := d.TaskAgentID("task-old"); ok {
		t.Fatalf("TaskAgentID(task-old)=(%q,%v) want (_,false)", agentID, ok)
	}
}

// TestMemoryTaskRoutingStore_BasicOperations 测试内存存储的基本操作
func TestMemoryTaskRoutingStore_BasicOperations(t *testing.T) {
	store := NewMemoryTaskRoutingStore()

	// 测试 Set 和 Get
	err := store.Set("task-1", "agent-1")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	routing, err := store.Get("task-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if routing.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want 'task-1'", routing.TaskID)
	}
	if routing.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want 'agent-1'", routing.AgentID)
	}
}

// TestMemoryTaskRoutingStore_GetNotFound 测试获取不存在的任务
func TestMemoryTaskRoutingStore_GetNotFound(t *testing.T) {
	store := NewMemoryTaskRoutingStore()

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("Get() should return error for nonexistent task")
	}
}

// TestMemoryTaskRoutingStore_UpdatePreservesCreatedAt 测试更新保留创建时间
func TestMemoryTaskRoutingStore_UpdatePreservesCreatedAt(t *testing.T) {
	store := NewMemoryTaskRoutingStore()

	_ = store.Set("task-1", "agent-1")
	time.Sleep(10 * time.Millisecond)

	_ = store.Set("task-1", "agent-2")

	routing, _ := store.Get("task-1")
	originalCreatedAt := routing.CreatedAt

	time.Sleep(10 * time.Millisecond)
	_ = store.Set("task-1", "agent-3")

	routing, _ = store.Get("task-1")
	if routing.CreatedAt != originalCreatedAt {
		t.Error("CreatedAt should be preserved across updates")
	}
	if routing.AgentID != "agent-3" {
		t.Errorf("AgentID = %q, want 'agent-3'", routing.AgentID)
	}
}

// TestMemoryTaskRoutingStore_Delete 测试删除任务路由
func TestMemoryTaskRoutingStore_Delete(t *testing.T) {
	store := NewMemoryTaskRoutingStore()

	_ = store.Set("task-1", "agent-1")
	_, err := store.Get("task-1")
	if err != nil {
		t.Fatal("Task should exist before delete")
	}

	err = store.Delete("task-1")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Get("task-1")
	if err == nil {
		t.Error("Task should not exist after delete")
	}
}

// TestMemoryTaskRoutingStore_List 测试列出所有任务路由
func TestMemoryTaskRoutingStore_List(t *testing.T) {
	store := NewMemoryTaskRoutingStore()

	_ = store.Set("task-1", "agent-1")
	_ = store.Set("task-2", "agent-2")
	_ = store.Set("task-3", "agent-3")

	list, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 3 {
		t.Errorf("List() returned %d items, want 3", len(list))
	}
}

// TestMemoryTaskRoutingStore_ListEmpty 测试列出空存储
func TestMemoryTaskRoutingStore_ListEmpty(t *testing.T) {
	store := NewMemoryTaskRoutingStore()

	list, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 0 {
		t.Errorf("List() returned %d items, want 0", len(list))
	}
}

// TestMemoryTaskRoutingStore_Cleanup 测试清理过期任务
func TestMemoryTaskRoutingStore_Cleanup(t *testing.T) {
	store := NewMemoryTaskRoutingStore()

	_ = store.Set("old-task", "agent-old")
	time.Sleep(10 * time.Millisecond)

	_ = store.Set("new-task", "agent-new")

	err := store.Cleanup(1 * time.Millisecond)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// old-task 应该被清理
	_, err = store.Get("old-task")
	if err == nil {
		t.Error("old-task should be cleaned up")
	}

	// new-task 应该仍然存在
	_, err = store.Get("new-task")
	if err != nil {
		t.Error("new-task should still exist")
	}
}

// TestMemoryTaskRoutingStore_CleanupAll 测试清理所有任务
func TestMemoryTaskRoutingStore_CleanupAll(t *testing.T) {
	store := NewMemoryTaskRoutingStore()

	_ = store.Set("task-1", "agent-1")
	_ = store.Set("task-2", "agent-2")
	time.Sleep(10 * time.Millisecond)

	_ = store.Cleanup(1 * time.Millisecond)

	list, _ := store.List()
	if len(list) != 0 {
		t.Errorf("After cleanup, List() should return 0 items, got %d", len(list))
	}
}

// TestMemoryTaskRoutingStore_Close 测试关闭存储
func TestMemoryTaskRoutingStore_Close(t *testing.T) {
	store := NewMemoryTaskRoutingStore()

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

// TestFileTaskRoutingStore_EmptyDirectory 测试空目录
func TestFileTaskRoutingStore_EmptyDirectory(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewFileTaskRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileTaskRoutingStore() error = %v", err)
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

// TestFileTaskRoutingStore_GetNotFound 测试文件存储获取不存在的任务
func TestFileTaskRoutingStore_GetNotFound(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewFileTaskRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileTaskRoutingStore() error = %v", err)
	}

	_, err = store.Get("nonexistent")
	if err == nil {
		t.Error("Get() should return error for nonexistent task")
	}
}

// TestFileTaskRoutingStore_Delete 测试文件存储删除
func TestFileTaskRoutingStore_Delete(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewFileTaskRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileTaskRoutingStore() error = %v", err)
	}

	_ = store.Set("task-1", "agent-1")
	err = store.Delete("task-1")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// 验证文件被删除
	filePath := filepath.Join(dataDir, "task_routing.json")
	if _, err := os.Stat(filePath); err == nil {
		// 文件可能存在但是空的（空 map）
		data, _ := os.ReadFile(filePath)
		if len(data) > 10 { // 至少有 "{}"
			t.Error("Task routing file should be empty or deleted")
		}
	}
}

// TestFileTaskRoutingStore_CleanupNoOldEntries 测试清理时没有旧条目
func TestFileTaskRoutingStore_CleanupNoOldEntries(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewFileTaskRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileTaskRoutingStore() error = %v", err)
	}

	_ = store.Set("task-1", "agent-1")

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

// TestFileTaskRoutingStore_Close 测试关闭文件存储
func TestFileTaskRoutingStore_Close(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewFileTaskRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileTaskRoutingStore() error = %v", err)
	}

	err = store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestPickAgentByHash 测试哈希选择代理
func TestPickAgentByHash(t *testing.T) {
	agents := []*reg.AgentSession{
		{AgentID: "agent-1", Addr: "127.0.0.1:9001"},
		{AgentID: "agent-2", Addr: "127.0.0.1:9002"},
		{AgentID: "agent-3", Addr: "127.0.0.1:9003"},
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
		{AgentID: "agent-1", Addr: "127.0.0.1:9001"},
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
		{AgentID: "agent-1", Addr: "127.0.0.1:9001"},
		{AgentID: "agent-2", Addr: "127.0.0.1:9002"},
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

// TestIsTerminalEvent 测试判断事件是否为终止事件
func TestIsTerminalEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		expected  bool
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
			evt := &sdkv1.TaskEvent{Type: tt.eventType}
			result := isTerminalEvent(evt)
			if result != tt.expected {
				t.Errorf("isTerminalEvent(%q) = %v, want %v", tt.eventType, result, tt.expected)
			}
		})
	}
}

// BenchmarkMemoryTaskRoutingStore_Set 性能基准测试
func BenchmarkMemoryTaskRoutingStore_Set(b *testing.B) {
	store := NewMemoryTaskRoutingStore()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Set("task-id", "agent-1")
	}
}

// BenchmarkMemoryTaskRoutingStore_Get 性能基准测试
func BenchmarkMemoryTaskRoutingStore_Get(b *testing.B) {
	store := NewMemoryTaskRoutingStore()
	_ = store.Set("task-id", "agent-1")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get("task-id")
	}
}

// BenchmarkPickAgentByHash 性能基准测试
func BenchmarkPickAgentByHash(b *testing.B) {
	agents := make([]*reg.AgentSession, 100)
	for i := 0; i < 100; i++ {
		agents[i] = &reg.AgentSession{
			AgentID: string(rune('a' + i)),
			Addr:    "127.0.0.1:9001",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pickAgentByHash(agents, "test-key")
	}
}
