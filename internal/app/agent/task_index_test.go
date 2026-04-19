package agent

import (
	"sync"
	"testing"
)

// TestNewTaskIndex 测试创建任务索引
func TestNewTaskIndex(t *testing.T) {
	idx := newTaskIndex()
	if idx == nil {
		t.Fatal("newTaskIndex() should return non-nil index")
	}
	if idx.byID == nil {
		t.Error("byID map should be initialized")
	}
}

// TestTaskIndex_Set_Get 测试设置和获取任务地址
func TestTaskIndex_Set_Get(t *testing.T) {
	idx := newTaskIndex()

	// 测试基本设置和获取
	idx.Set("task1", "addr1")
	addr, ok := idx.Get("task1")
	if !ok {
		t.Error("Get() should return true for existing task")
	}
	if addr != "addr1" {
		t.Errorf("Get() = %s, want 'addr1'", addr)
	}

	// 测试获取不存在的任务
	_, ok = idx.Get("nonexistent")
	if ok {
		t.Error("Get() should return false for non-existent task")
	}
}

// TestTaskIndex_Set_EmptyValues 测试空值处理
func TestTaskIndex_Set_EmptyValues(t *testing.T) {
	idx := newTaskIndex()

	// 空 taskID 应该被忽略
	idx.Set("", "addr1")
	if idx.Len() != 0 {
		t.Error("Set with empty taskID should not add entry")
	}

	// 空 addr 应该被忽略
	idx.Set("task1", "")
	if idx.Len() != 0 {
		t.Error("Set with empty addr should not add entry")
	}

	// 两个都为空
	idx.Set("", "")
	if idx.Len() != 0 {
		t.Error("Set with both empty should not add entry")
	}
}

// TestTaskIndex_Delete 测试删除任务
func TestTaskIndex_Delete(t *testing.T) {
	idx := newTaskIndex()

	// 添加一个任务
	idx.Set("task1", "addr1")
	if idx.Len() != 1 {
		t.Errorf("Len() = %d, want 1", idx.Len())
	}

	// 删除任务
	idx.Delete("task1")
	if idx.Len() != 0 {
		t.Errorf("Len() after Delete = %d, want 0", idx.Len())
	}

	// 验证任务已被删除
	_, ok := idx.Get("task1")
	if ok {
		t.Error("Get() should return false after Delete")
	}
}

// TestTaskIndex_Delete_EmptyTaskID 测试删除空 taskID
func TestTaskIndex_Delete_EmptyTaskID(t *testing.T) {
	idx := newTaskIndex()

	// 添加一个任务
	idx.Set("task1", "addr1")

	// 删除空 taskID 应该是安全的（no-op）
	idx.Delete("")
	if idx.Len() != 1 {
		t.Error("Delete with empty taskID should not remove entries")
	}

	// 验证原任务仍然存在
	_, ok := idx.Get("task1")
	if !ok {
		t.Error("Original task should still exist after Delete(\"\")")
	}
}

// TestTaskIndex_Delete_NonExistent 测试删除不存在的任务
func TestTaskIndex_Delete_NonExistent(t *testing.T) {
	idx := newTaskIndex()

	// 删除不存在的任务应该是安全的
	idx.Delete("nonexistent")
	if idx.Len() != 0 {
		t.Error("Delete non-existent task should be safe")
	}
}

// TestTaskIndex_Len 测试长度计数
func TestTaskIndex_Len(t *testing.T) {
	idx := newTaskIndex()

	// 空索引
	if idx.Len() != 0 {
		t.Errorf("Len() of empty index = %d, want 0", idx.Len())
	}

	// 添加几个任务
	idx.Set("task1", "addr1")
	if idx.Len() != 1 {
		t.Errorf("Len() after 1 add = %d, want 1", idx.Len())
	}

	idx.Set("task2", "addr2")
	if idx.Len() != 2 {
		t.Errorf("Len() after 2 adds = %d, want 2", idx.Len())
	}

	idx.Set("task3", "addr3")
	if idx.Len() != 3 {
		t.Errorf("Len() after 3 adds = %d, want 3", idx.Len())
	}

	// 删除一个任务
	idx.Delete("task1")
	if idx.Len() != 2 {
		t.Errorf("Len() after delete = %d, want 2", idx.Len())
	}
}

// TestTaskIndex_Overwrite 测试覆盖现有任务
func TestTaskIndex_Overwrite(t *testing.T) {
	idx := newTaskIndex()

	// 设置初始值
	idx.Set("task1", "addr1")
	addr, _ := idx.Get("task1")
	if addr != "addr1" {
		t.Errorf("Initial addr = %s, want 'addr1'", addr)
	}

	// 覆盖
	idx.Set("task1", "addr2")
	addr, _ = idx.Get("task1")
	if addr != "addr2" {
		t.Errorf("After overwrite addr = %s, want 'addr2'", addr)
	}

	// 长度应该仍然是 1
	if idx.Len() != 1 {
		t.Errorf("Len() after overwrite = %d, want 1", idx.Len())
	}
}

// TestTaskIndex_ConcurrentAccess 测试并发访问
func TestTaskIndex_ConcurrentAccess(t *testing.T) {
	idx := newTaskIndex()
	var wg sync.WaitGroup

	// 并发写入
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			taskID := "task" + string(rune('0'+n%10))
			addr := "addr" + string(rune('0'+n%10))
			idx.Set(taskID, addr)
		}(i)
	}

	// 并发读取
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			taskID := "task" + string(rune('0'+n%10))
			idx.Get(taskID)
		}(i)
	}

	// 并发删除
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			taskID := "task" + string(rune('0'+n%10))
			idx.Delete(taskID)
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

// TestTaskIndex_MultipleOperations 测试多种操作组合
func TestTaskIndex_MultipleOperations(t *testing.T) {
	idx := newTaskIndex()

	// 添加多个任务
	tasks := []struct {
		taskID string
		addr   string
	}{
		{"task1", "addr1"},
		{"task2", "addr2"},
		{"task3", "addr3"},
		{"task4", "addr4"},
		{"task5", "addr5"},
	}

	for _, task := range tasks {
		idx.Set(task.taskID, task.addr)
	}

	// 验证所有任务都能被找到
	for _, task := range tasks {
		addr, ok := idx.Get(task.taskID)
		if !ok {
			t.Errorf("Get(%s) should return true", task.taskID)
		}
		if addr != task.addr {
			t.Errorf("Get(%s) = %s, want %s", task.taskID, addr, task.addr)
		}
	}

	// 验证长度
	if idx.Len() != len(tasks) {
		t.Errorf("Len() = %d, want %d", idx.Len(), len(tasks))
	}

	// 删除部分任务
	idx.Delete("task2")
	idx.Delete("task4")

	// 验证删除后的状态
	if idx.Len() != 3 {
		t.Errorf("Len() after deletes = %d, want 3", idx.Len())
	}

	_, ok := idx.Get("task2")
	if ok {
		t.Error("task2 should not exist after Delete")
	}

	// 验证未删除的任务仍然存在
	_, ok = idx.Get("task1")
	if !ok {
		t.Error("task1 should still exist")
	}
}
