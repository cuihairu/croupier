package registry

import (
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// TestNewMetricsStore 测试创建 MetricsStore
func TestNewMetricsStore(t *testing.T) {
	store := NewMetricsStore()
	if store == nil {
		t.Fatal("NewMetricsStore() should return non-nil store")
	}
	if store.entries == nil {
		t.Error("entries should be initialized")
	}
	if store.byAgent == nil {
		t.Error("byAgent should be initialized")
	}
	if store.maxPerAgent != 100 {
		t.Errorf("maxPerAgent = %d, want 100", store.maxPerAgent)
	}
	if store.maxTotal != 10000 {
		t.Errorf("maxTotal = %d, want 10000", store.maxTotal)
	}
}

// TestMetricsStore_Add_GetLatest 测试添加和获取最新指标
func TestMetricsStore_Add_GetLatest(t *testing.T) {
	store := NewMetricsStore()

	// 创建测试报告
	report := &opsv1.MetricsReport{
		Cpu: &opsv1.CpuMetrics{UsagePercent: 50.0},
	}

	// 添加报告
	store.Add("agent1", report)

	// 获取最新报告
	entry, ok := store.GetLatest("agent1")
	if !ok {
		t.Fatal("GetLatest() should return true for existing agent")
	}
	if entry == nil {
		t.Fatal("GetLatest() should return non-nil entry")
	}
	if entry.AgentID != "agent1" {
		t.Errorf("AgentID = %s, want 'agent1'", entry.AgentID)
	}

	// 测试不存在的 agent
	_, ok = store.GetLatest("nonexistent")
	if ok {
		t.Error("GetLatest() should return false for non-existent agent")
	}
}

// TestMetricsStore_Add_EmptyValues 测试空值处理
func TestMetricsStore_Add(t *testing.T) {
	store := NewMetricsStore()
	report := &opsv1.MetricsReport{}

	// 空 agentID 应该被忽略
	store.Add("", report)
	if len(store.ListAgents()) != 0 {
		t.Error("Add with empty agentID should not add entry")
	}

	// nil report 应该被忽略
	store.Add("agent1", nil)
	if len(store.ListAgents()) != 0 {
		t.Error("Add with nil report should not add entry")
	}

	// 都为空
	store.Add("", nil)
	if len(store.ListAgents()) != 0 {
		t.Error("Add with both empty should not add entry")
	}

	// 测试空 agentID 获取
	_, ok := store.GetLatest("")
	if ok {
		t.Error("GetLatest with empty agentID should return false")
	}
}

// TestMetricsStore_GetAgentMetrics 测试获取 agent 的所有指标
func TestMetricsStore_GetAgentMetrics(t *testing.T) {
	store := NewMetricsStore()

	// 添加多个报告
	report1 := &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 10.0}}
	report2 := &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 20.0}}
	report3 := &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 30.0}}

	store.Add("agent1", report1)
	time.Sleep(5 * time.Millisecond) // 确保时间不同
	store.Add("agent1", report2)
	time.Sleep(5 * time.Millisecond)
	store.Add("agent1", report3)

	// 获取所有指标
	metrics := store.GetAgentMetrics("agent1", 0)
	if len(metrics) != 3 {
		t.Errorf("GetAgentMetrics() = %d entries, want 3", len(metrics))
	}

	// 测试 limit
	metrics = store.GetAgentMetrics("agent1", 2)
	if len(metrics) != 2 {
		t.Errorf("GetAgentMetrics(limit=2) = %d entries, want 2", len(metrics))
	}

	// 测试空 agentID
	metrics = store.GetAgentMetrics("", 0)
	if metrics != nil {
		t.Error("GetAgentMetrics with empty agentID should return nil")
	}

	// 测试不存在的 agent
	metrics = store.GetAgentMetrics("nonexistent", 0)
	if metrics != nil {
		t.Error("GetAgentMetrics for non-existent agent should return nil")
	}
}

// TestMetricsStore_GetAllMetrics 测试获取所有指标
func TestMetricsStore_GetAllMetrics(t *testing.T) {
	store := NewMetricsStore()

	report1 := &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 10.0}}
	report2 := &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 20.0}}

	store.Add("agent1", report1)
	store.Add("agent2", report2)

	// 获取所有指标 - limit=0 表示无限制
	metrics := store.GetAllMetrics(time.Time{}, 100) // 使用大的 limit 而不是 0
	if len(metrics) < 2 {
		t.Errorf("GetAllMetrics() = %d entries, want at least 2", len(metrics))
	}

	// 测试 limit
	metrics = store.GetAllMetrics(time.Time{}, 1)
	if len(metrics) != 1 {
		t.Errorf("GetAllMetrics(limit=1) = %d entries, want 1", len(metrics))
	}

	// 测试 since 过滤
	since := time.Now().Add(-time.Hour)
	metrics = store.GetAllMetrics(since, 100)
	if len(metrics) < 2 {
		t.Errorf("GetAllMetrics(since=1hour ago) = %d entries, want at least 2", len(metrics))
	}

	// 测试未来的 since（应该返回空）
	since = time.Now().Add(time.Hour)
	metrics = store.GetAllMetrics(since, 100)
	if len(metrics) != 0 {
		t.Error("GetAllMetrics with future since should return empty")
	}
}

// TestMetricsStore_ListAgents 测试列出所有 agent
func TestMetricsStore_ListAgents(t *testing.T) {
	store := NewMetricsStore()

	// 空 store
	agents := store.ListAgents()
	if len(agents) != 0 {
		t.Errorf("ListAgents() of empty store = %d, want 0", len(agents))
	}

	// 添加 agent
	store.Add("agent1", &opsv1.MetricsReport{})
	store.Add("agent2", &opsv1.MetricsReport{})
	store.Add("agent3", &opsv1.MetricsReport{})

	agents = store.ListAgents()
	if len(agents) != 3 {
		t.Errorf("ListAgents() = %d agents, want 3", len(agents))
	}

	// 验证包含所有 agent ID
	agentMap := make(map[string]bool)
	for _, agent := range agents {
		agentMap[agent] = true
	}
	if !agentMap["agent1"] || !agentMap["agent2"] || !agentMap["agent3"] {
		t.Error("ListAgents() should contain all agent IDs")
	}
}

// TestMetricsStore_Clear 测试清除 agent 指标
func TestMetricsStore_Clear(t *testing.T) {
	store := NewMetricsStore()

	// 添加指标
	store.Add("agent1", &opsv1.MetricsReport{})
	store.Add("agent1", &opsv1.MetricsReport{})
	store.Add("agent2", &opsv1.MetricsReport{})

	// 清除 agent1
	store.Clear("agent1")

	// 验证 agent1 已清除
	_, ok := store.GetLatest("agent1")
	if ok {
		t.Error("agent1 should be cleared")
	}

	// 验证 agent2 仍然存在
	_, ok = store.GetLatest("agent2")
	if !ok {
		t.Error("agent2 should still exist")
	}

	// 验证列表中不再包含 agent1
	agents := store.ListAgents()
	if len(agents) != 1 || agents[0] != "agent2" {
		t.Errorf("ListAgents() after Clear = %v, want [agent2]", agents)
	}

	// 测试清除空 agentID（应该是 no-op）
	store.Clear("")
	// 验证 agent2 仍然存在
	_, ok = store.GetLatest("agent2")
	if !ok {
		t.Error("Clear with empty agentID should be no-op")
	}

	// 测试清除不存在的 agent
	store.Clear("nonexistent")
	// 验证 agent2 仍然存在
	_, ok = store.GetLatest("agent2")
	if !ok {
		t.Error("Clear non-existent agent should be safe")
	}
}

// TestMetricsStore_Prune 测试修剪旧指标
func TestMetricsStore_Prune(t *testing.T) {
	store := NewMetricsStore()

	// 添加一些指标
	for i := 0; i < 5; i++ {
		report := &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: float64(i)}}
		store.Add("agent1", report)
	}

	// 等待一小段时间
	time.Sleep(10 * time.Millisecond)

	// 再添加一个指标
	report := &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 99.0}}
	store.Add("agent2", report)

	// 修剪 1ms 之前的所有指标
	store.Prune(1 * time.Millisecond)

	// agent1 的所有指标应该被清除（都在 10ms 前）
	_, ok := store.GetLatest("agent1")
	if ok {
		t.Error("agent1 metrics should be pruned")
	}

	// agent2 应该仍然存在（刚刚添加）
	_, ok = store.GetLatest("agent2")
	if !ok {
		t.Error("agent2 metrics should still exist after prune")
	}

	// 测试无效参数
	store.Prune(0)
	store.Prune(-1)
	// 验证 agent2 仍然存在
	_, ok = store.GetLatest("agent2")
	if !ok {
		t.Error("Prune with invalid duration should be safe")
	}
}

// TestNewSystemInfoCache 测试创建 SystemInfoCache
func TestNewSystemInfoCache(t *testing.T) {
	cache := NewSystemInfoCache()
	if cache == nil {
		t.Fatal("NewSystemInfoCache() should return non-nil cache")
	}
	if cache.infos == nil {
		t.Error("infos should be initialized")
	}
}

// TestSystemInfoCache_Set_Get 测试设置和获取系统信息
func TestSystemInfoCache_Set_Get(t *testing.T) {
	cache := NewSystemInfoCache()

	info := &opsv1.SystemInfo{
		Hostname:     "test-host",
		Os:           "linux",
		Arch:         "amd64",
		CpuCores:     4,
		AgentVersion: "1.0.0",
	}

	// 设置信息
	cache.Set("agent1", info)

	// 获取信息
	retrieved, ok := cache.Get("agent1")
	if !ok {
		t.Fatal("Get() should return true for existing agent")
	}
	if retrieved == nil {
		t.Fatal("Get() should return non-nil info")
	}
	if retrieved.Hostname != "test-host" {
		t.Errorf("Hostname = %s, want 'test-host'", retrieved.Hostname)
	}

	// 测试空 agentID
	cache.Set("", info)
	_, ok = cache.Get("")
	if ok {
		t.Error("Get/Set with empty agentID should not work")
	}

	// 测试 nil info
	cache.Set("agent2", nil)
	_, ok = cache.Get("agent2")
	if ok {
		t.Error("Set nil info should not add entry")
	}
}

// TestSystemInfoCache_List 测试列出所有系统信息
func TestSystemInfoCache_List(t *testing.T) {
	cache := NewSystemInfoCache()

	// 空 cache
	infos := cache.List()
	if len(infos) != 0 {
		t.Errorf("List() of empty cache = %d, want 0", len(infos))
	}

	// 添加多个 agent
	info1 := &opsv1.SystemInfo{Hostname: "host1"}
	info2 := &opsv1.SystemInfo{Hostname: "host2"}
	info3 := &opsv1.SystemInfo{Hostname: "host3"}

	cache.Set("agent1", info1)
	cache.Set("agent2", info2)
	cache.Set("agent3", info3)

	infos = cache.List()
	if len(infos) != 3 {
		t.Errorf("List() = %d infos, want 3", len(infos))
	}

	// 验证包含所有 agent
	if _, ok := infos["agent1"]; !ok {
		t.Error("List() should contain agent1")
	}
	if _, ok := infos["agent2"]; !ok {
		t.Error("List() should contain agent2")
	}
	if _, ok := infos["agent3"]; !ok {
		t.Error("List() should contain agent3")
	}
}

// TestSystemInfoCache_Remove 测试移除系统信息
func TestSystemInfoCache_Remove(t *testing.T) {
	cache := NewSystemInfoCache()

	info := &opsv1.SystemInfo{Hostname: "test"}
	cache.Set("agent1", info)

	// 移除
	cache.Remove("agent1")

	// 验证已移除
	_, ok := cache.Get("agent1")
	if ok {
		t.Error("agent1 should be removed")
	}

	// 测试移除空 agentID
	cache.Set("agent2", info)
	cache.Remove("")
	_, ok = cache.Get("agent2")
	if !ok {
		t.Error("Remove with empty agentID should be no-op")
	}

	// 测试移除不存在的 agent
	cache.Remove("nonexistent")
	// 验证 agent2 仍然存在
	_, ok = cache.Get("agent2")
	if !ok {
		t.Error("Remove non-existent agent should be safe")
	}
}

// TestSystemInfoCache_Prune 测试修剪系统信息缓存
func TestSystemInfoCache_Prune(t *testing.T) {
	cache := NewSystemInfoCache()

	// 添加一些信息
	info1 := &opsv1.SystemInfo{Hostname: "host1"}
	cache.Set("agent1", info1)

	// 等待一小段时间
	time.Sleep(10 * time.Millisecond)

	info2 := &opsv1.SystemInfo{Hostname: "host2"}
	cache.Set("agent2", info2)

	// 修剪 1ms 之前的所有条目
	cache.Prune(1 * time.Millisecond)

	// agent1 应该被清除
	_, ok := cache.Get("agent1")
	if ok {
		t.Error("agent1 should be pruned")
	}

	// agent2 应该仍然存在
	_, ok = cache.Get("agent2")
	if !ok {
		t.Error("agent2 should still exist after prune")
	}

	// 测试无效参数
	cache.Prune(0)
	cache.Prune(-1)
	_, ok = cache.Get("agent2")
	if !ok {
		t.Error("Prune with invalid duration should be safe")
	}
}
