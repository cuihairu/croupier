package jobs

import (
	"sync"
	"testing"
)

// TestNewRouter 测试创建 Router
func TestNewRouter(t *testing.T) {
	r := NewRouter()
	if r == nil {
		t.Fatal("NewRouter returned nil")
	}
	if r.m == nil {
		t.Error("Router map is not initialized")
	}
}

// TestRouter_SetAndGet 测试设置和获取
func TestRouter_SetAndGet(t *testing.T) {
	r := NewRouter()

	// 设置路由
	r.Set("job1", "agent1:8080")

	// 获取路由
	addr, ok := r.Get("job1")
	if !ok {
		t.Error("Get failed: job not found")
	}
	if addr != "agent1:8080" {
		t.Errorf("Get returned wrong address: got %s, want agent1:8080", addr)
	}
}

// TestRouter_GetNonExistent 测试获取不存在的任务
func TestRouter_GetNonExistent(t *testing.T) {
	r := NewRouter()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get should return false for non-existent job")
	}
}

// TestRouter_Update 测试更新路由
func TestRouter_Update(t *testing.T) {
	r := NewRouter()

	// 设置初始值
	r.Set("job1", "agent1:8080")

	// 更新值
	r.Set("job1", "agent2:9090")

	// 验证更新
	addr, ok := r.Get("job1")
	if !ok {
		t.Fatal("Get failed: job not found")
	}
	if addr != "agent2:9090" {
		t.Errorf("Get returned wrong address after update: got %s, want agent2:9090", addr)
	}
}

// TestRouter_MultipleJobs 测试多个任务
func TestRouter_MultipleJobs(t *testing.T) {
	r := NewRouter()

	jobs := []struct {
		id   string
		addr string
	}{
		{"job1", "agent1:8080"},
		{"job2", "agent2:8080"},
		{"job3", "agent3:8080"},
	}

	// 设置所有任务
	for _, job := range jobs {
		r.Set(job.id, job.addr)
	}

	// 验证所有任务
	for _, job := range jobs {
		addr, ok := r.Get(job.id)
		if !ok {
			t.Errorf("Job %s not found", job.id)
			continue
		}
		if addr != job.addr {
			t.Errorf("Job %s: got %s, want %s", job.id, addr, job.addr)
		}
	}
}

// TestRouter_ConcurrentAccess 测试并发访问
func TestRouter_ConcurrentAccess(t *testing.T) {
	r := NewRouter()
	var wg sync.WaitGroup

	numGoroutines := 100
	numOps := 100

	// 并发写入
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				jobID := "job" + string(rune(idx%10))
				addr := "agent" + string(rune(idx%10)) + ":8080"
				r.Set(jobID, addr)
			}
		}(i)
	}

	// 并发读取
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				jobID := "job" + string(rune(idx%10))
				r.Get(jobID)
			}
		}(i)
	}

	wg.Wait()

	// 验证数据一致性
	for i := 0; i < 10; i++ {
		jobID := "job" + string(rune(i))
		_, ok := r.Get(jobID)
		if !ok {
			t.Errorf("Job %s should exist", jobID)
		}
	}
}

// TestRouter_Overwrite 测试覆盖现有路由
func TestRouter_Overwrite(t *testing.T) {
	r := NewRouter()

	// 设置多个任务
	r.Set("job1", "agent1:8080")
	r.Set("job2", "agent2:8080")
	r.Set("job3", "agent3:8080")

	// 覆盖 job1
	r.Set("job1", "agent999:9999")

	// 验证 job1 被更新
	addr, ok := r.Get("job1")
	if !ok {
		t.Fatal("job1 not found")
	}
	if addr != "agent999:9999" {
		t.Errorf("job1: got %s, want agent999:9999", addr)
	}

	// 验证其他任务不受影响
	addr2, ok := r.Get("job2")
	if !ok || addr2 != "agent2:8080" {
		t.Errorf("job2 was affected: got %s, want agent2:8080", addr2)
	}
}

// TestRouter_EmptyKey 测试空键
func TestRouter_EmptyKey(t *testing.T) {
	r := NewRouter()

	// 设置空键
	r.Set("", "agent1:8080")

	// 获取空键
	addr, ok := r.Get("")
	if !ok {
		t.Error("Empty key should be stored")
	}
	if addr != "agent1:8080" {
		t.Errorf("Empty key: got %s, want agent1:8080", addr)
	}
}

// TestRouter_EmptyValue 测试空值
func TestRouter_EmptyValue(t *testing.T) {
	r := NewRouter()

	// 设置空值
	r.Set("job1", "")

	// 获取空值
	addr, ok := r.Get("job1")
	if !ok {
		t.Error("Job with empty value should exist")
	}
	if addr != "" {
		t.Errorf("Empty value: got %s, want empty string", addr)
	}
}

// TestRouter_SpecialCharacters 测试特殊字符
func TestRouter_SpecialCharacters(t *testing.T) {
	r := NewRouter()

	specialCases := []struct {
		id   string
		addr string
	}{
		{"job-with-dash", "agent-with-dash:8080"},
		{"job_with_underscore", "agent_with_underscore:8080"},
		{"job.with.dot", "agent.with.dot:8080"},
		{"job/with/slash", "agent/with/slash:8080"},
		{"job:with:colon", "agent:with:colon:8080"},
	}

	for _, tc := range specialCases {
		r.Set(tc.id, tc.addr)
		addr, ok := r.Get(tc.id)
		if !ok {
			t.Errorf("Job %s not found", tc.id)
			continue
		}
		if addr != tc.addr {
			t.Errorf("Job %s: got %s, want %s", tc.id, addr, tc.addr)
		}
	}
}

// TestRouter_LongStrings 测试长字符串
func TestRouter_LongStrings(t *testing.T) {
	r := NewRouter()

	longID := string(make([]byte, 10000))
	for i := range longID {
		longID = longID[:i] + "a" + longID[i+1:]
	}

	longAddr := string(make([]byte, 10000))
	for i := range longAddr {
		longAddr = longAddr[:i] + "b" + longAddr[i+1:]
	}

	r.Set(longID, longAddr)

	addr, ok := r.Get(longID)
	if !ok {
		t.Error("Long ID job not found")
	}
	if addr != longAddr {
		t.Error("Long address mismatch")
	}
}

// BenchmarkRouter_Set 设置操作基准测试
func BenchmarkRouter_Set(b *testing.B) {
	r := NewRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Set("job1", "agent1:8080")
	}
}

// BenchmarkRouter_Get 获取操作基准测试
func BenchmarkRouter_Get(b *testing.B) {
	r := NewRouter()
	r.Set("job1", "agent1:8080")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Get("job1")
	}
}

// BenchmarkRouter_ConcurrentSet 并发设置基准测试
func BenchmarkRouter_ConcurrentSet(b *testing.B) {
	r := NewRouter()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			r.Set("job"+string(rune(i%10)), "agent"+string(rune(i%10))+":8080")
			i++
		}
	})
}

// BenchmarkRouter_ConcurrentGet 并发获取基准测试
func BenchmarkRouter_ConcurrentGet(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 10; i++ {
		r.Set("job"+string(rune(i)), "agent"+string(rune(i))+":8080")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			r.Get("job" + string(rune(i%10)))
			i++
		}
	})
}

// BenchmarkRouter_ConcurrentMixed 并发混合操作基准测试
func BenchmarkRouter_ConcurrentMixed(b *testing.B) {
	r := NewRouter()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				r.Set("job"+string(rune(i%10)), "agent"+string(rune(i%10))+":8080")
			} else {
				r.Get("job" + string(rune(i%10)))
			}
			i++
		}
	})
}
