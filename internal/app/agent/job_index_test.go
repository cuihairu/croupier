package agent

import (
	"sync"
	"testing"
)

// TestNewJobIndex 测试创建作业索引
func TestNewJobIndex(t *testing.T) {
	idx := newJobIndex()
	if idx == nil {
		t.Fatal("newJobIndex() should return non-nil index")
	}
	if idx.byID == nil {
		t.Error("byID map should be initialized")
	}
}

// TestJobIndex_Set_Get 测试设置和获取作业地址
func TestJobIndex_Set_Get(t *testing.T) {
	idx := newJobIndex()

	// 测试基本设置和获取
	idx.Set("job1", "addr1")
	addr, ok := idx.Get("job1")
	if !ok {
		t.Error("Get() should return true for existing job")
	}
	if addr != "addr1" {
		t.Errorf("Get() = %s, want 'addr1'", addr)
	}

	// 测试获取不存在的作业
	_, ok = idx.Get("nonexistent")
	if ok {
		t.Error("Get() should return false for non-existent job")
	}
}

// TestJobIndex_Set_EmptyValues 测试空值处理
func TestJobIndex_Set_EmptyValues(t *testing.T) {
	idx := newJobIndex()

	// 空 jobID 应该被忽略
	idx.Set("", "addr1")
	if idx.Len() != 0 {
		t.Error("Set with empty jobID should not add entry")
	}

	// 空 addr 应该被忽略
	idx.Set("job1", "")
	if idx.Len() != 0 {
		t.Error("Set with empty addr should not add entry")
	}

	// 两个都为空
	idx.Set("", "")
	if idx.Len() != 0 {
		t.Error("Set with both empty should not add entry")
	}
}

// TestJobIndex_Delete 测试删除作业
func TestJobIndex_Delete(t *testing.T) {
	idx := newJobIndex()

	// 添加一个作业
	idx.Set("job1", "addr1")
	if idx.Len() != 1 {
		t.Errorf("Len() = %d, want 1", idx.Len())
	}

	// 删除作业
	idx.Delete("job1")
	if idx.Len() != 0 {
		t.Errorf("Len() after Delete = %d, want 0", idx.Len())
	}

	// 验证作业已被删除
	_, ok := idx.Get("job1")
	if ok {
		t.Error("Get() should return false after Delete")
	}
}

// TestJobIndex_Delete_EmptyJobID 测试删除空 jobID
func TestJobIndex_Delete_EmptyJobID(t *testing.T) {
	idx := newJobIndex()

	// 添加一个作业
	idx.Set("job1", "addr1")

	// 删除空 jobID 应该是安全的（no-op）
	idx.Delete("")
	if idx.Len() != 1 {
		t.Error("Delete with empty jobID should not remove entries")
	}

	// 验证原作业仍然存在
	_, ok := idx.Get("job1")
	if !ok {
		t.Error("Original job should still exist after Delete(\"\")")
	}
}

// TestJobIndex_Delete_NonExistent 测试删除不存在的作业
func TestJobIndex_Delete_NonExistent(t *testing.T) {
	idx := newJobIndex()

	// 删除不存在的作业应该是安全的
	idx.Delete("nonexistent")
	if idx.Len() != 0 {
		t.Error("Delete non-existent job should be safe")
	}
}

// TestJobIndex_Len 测试长度计数
func TestJobIndex_Len(t *testing.T) {
	idx := newJobIndex()

	// 空索引
	if idx.Len() != 0 {
		t.Errorf("Len() of empty index = %d, want 0", idx.Len())
	}

	// 添加几个作业
	idx.Set("job1", "addr1")
	if idx.Len() != 1 {
		t.Errorf("Len() after 1 add = %d, want 1", idx.Len())
	}

	idx.Set("job2", "addr2")
	if idx.Len() != 2 {
		t.Errorf("Len() after 2 adds = %d, want 2", idx.Len())
	}

	idx.Set("job3", "addr3")
	if idx.Len() != 3 {
		t.Errorf("Len() after 3 adds = %d, want 3", idx.Len())
	}

	// 删除一个作业
	idx.Delete("job1")
	if idx.Len() != 2 {
		t.Errorf("Len() after delete = %d, want 2", idx.Len())
	}
}

// TestJobIndex_Overwrite 测试覆盖现有作业
func TestJobIndex_Overwrite(t *testing.T) {
	idx := newJobIndex()

	// 设置初始值
	idx.Set("job1", "addr1")
	addr, _ := idx.Get("job1")
	if addr != "addr1" {
		t.Errorf("Initial addr = %s, want 'addr1'", addr)
	}

	// 覆盖
	idx.Set("job1", "addr2")
	addr, _ = idx.Get("job1")
	if addr != "addr2" {
		t.Errorf("After overwrite addr = %s, want 'addr2'", addr)
	}

	// 长度应该仍然是 1
	if idx.Len() != 1 {
		t.Errorf("Len() after overwrite = %d, want 1", idx.Len())
	}
}

// TestJobIndex_ConcurrentAccess 测试并发访问
func TestJobIndex_ConcurrentAccess(t *testing.T) {
	idx := newJobIndex()
	var wg sync.WaitGroup

	// 并发写入
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			jobID := "job" + string(rune('0'+n%10))
			addr := "addr" + string(rune('0'+n%10))
			idx.Set(jobID, addr)
		}(i)
	}

	// 并发读取
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			jobID := "job" + string(rune('0'+n%10))
			idx.Get(jobID)
		}(i)
	}

	// 并发删除
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			jobID := "job" + string(rune('0'+n%10))
			idx.Delete(jobID)
		}(i)
	}

	wg.Wait()

	// 验证索引仍然可用
	idx.Set("test", "value")
	addr, ok := idx.Get("test")
	if !ok || addr != "value" {
		t.Error("Index should be functional after concurrent access")
	}
}

// TestJobIndex_MultipleOperations 测试多种操作组合
func TestJobIndex_MultipleOperations(t *testing.T) {
	idx := newJobIndex()

	// 添加多个作业
	jobs := []struct {
		jobID string
		addr  string
	}{
		{"job1", "addr1"},
		{"job2", "addr2"},
		{"job3", "addr3"},
		{"job4", "addr4"},
		{"job5", "addr5"},
	}

	for _, job := range jobs {
		idx.Set(job.jobID, job.addr)
	}

	// 验证所有作业都能被找到
	for _, job := range jobs {
		addr, ok := idx.Get(job.jobID)
		if !ok {
			t.Errorf("Get(%s) should return true", job.jobID)
		}
		if addr != job.addr {
			t.Errorf("Get(%s) = %s, want %s", job.jobID, addr, job.addr)
		}
	}

	// 验证长度
	if idx.Len() != len(jobs) {
		t.Errorf("Len() = %d, want %d", idx.Len(), len(jobs))
	}

	// 删除部分作业
	idx.Delete("job2")
	idx.Delete("job4")

	// 验证删除后的状态
	if idx.Len() != 3 {
		t.Errorf("Len() after deletes = %d, want 3", idx.Len())
	}

	_, ok := idx.Get("job2")
	if ok {
		t.Error("job2 should not exist after Delete")
	}

	// 验证未删除的作业仍然存在
	_, ok = idx.Get("job1")
	if !ok {
		t.Error("job1 should still exist")
	}
}
