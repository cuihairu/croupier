package mocks

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMockFunctionStore_AddAndGet(t *testing.T) {
	store := NewMockFunctionStore()

	info := &FunctionInfo{
		FunctionID:  "player.ban",
		DisplayName: "Ban Player",
		Tags:        []string{"player", "moderation"},
		InputSchema: `{"type":"object"}`,
	}

	store.AddFunction(info)

	got, err := store.GetFunction("player.ban")
	if err != nil {
		t.Errorf("GetFunction() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetFunction() returned nil")
	}
	if got.FunctionID != info.FunctionID {
		t.Errorf("FunctionID = %v, want %v", got.FunctionID, info.FunctionID)
	}
	if got.DisplayName != info.DisplayName {
		t.Errorf("DisplayName = %v, want %v", got.DisplayName, info.DisplayName)
	}
}

func TestMockFunctionStore_GetNonExistent(t *testing.T) {
	store := NewMockFunctionStore()

	got, err := store.GetFunction("nonexistent")
	if err != nil {
		t.Errorf("GetFunction() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetFunction() = %v, want nil", got)
	}
}

func TestMockFunctionStore_SetError(t *testing.T) {
	store := NewMockFunctionStore()
	expectedErr := errors.New("database error")
	store.SetError(expectedErr)

	_, err := store.GetFunction("any")
	if err != expectedErr {
		t.Errorf("GetFunction() error = %v, want %v", err, expectedErr)
	}

	_, err = store.ListFunctions()
	if err != expectedErr {
		t.Errorf("ListFunctions() error = %v, want %v", err, expectedErr)
	}
}

func TestMockFunctionStore_ListFunctions(t *testing.T) {
	store := NewMockFunctionStore()

	store.AddFunction(&FunctionInfo{FunctionID: "func1"})
	store.AddFunction(&FunctionInfo{FunctionID: "func2"})
	store.AddFunction(&FunctionInfo{FunctionID: "func3"})

	list, err := store.ListFunctions()
	if err != nil {
		t.Errorf("ListFunctions() error = %v", err)
	}
	if len(list) != 3 {
		t.Errorf("ListFunctions() returned %d items, want 3", len(list))
	}
}

func TestMockFunctionStore_Clear(t *testing.T) {
	store := NewMockFunctionStore()

	store.AddFunction(&FunctionInfo{FunctionID: "func1"})
	store.SetError(errors.New("error"))
	store.Clear()

	list, err := store.ListFunctions()
	if err != nil {
		t.Errorf("ListFunctions() after Clear() error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("ListFunctions() after Clear() = %d, want 0", len(list))
	}
}

func TestMockFunctionStore_ConcurrentAccess(t *testing.T) {
	store := NewMockFunctionStore()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			store.AddFunction(&FunctionInfo{
				FunctionID: "func" + string(rune('0'+idx%10)),
			})
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.ListFunctions()
		}()
	}

	wg.Wait()
}

func TestMockServiceContext_Basic(t *testing.T) {
	ctx := NewMockServiceContext()

	if !ctx.IsRunning() {
		t.Error("IsRunning() = false, want true")
	}

	ctx.SetRunning(false)
	if ctx.IsRunning() {
		t.Error("IsRunning() after SetRunning(false) = true, want false")
	}
}

func TestMockServiceContext_ActiveTasks(t *testing.T) {
	ctx := NewMockServiceContext()

	if ctx.GetActiveTasks() != 0 {
		t.Errorf("GetActiveTasks() = %d, want 0", ctx.GetActiveTasks())
	}

	ctx.SetActiveTasks(5)
	if ctx.GetActiveTasks() != 5 {
		t.Errorf("GetActiveTasks() = %d, want 5", ctx.GetActiveTasks())
	}
}

func TestMockServiceContext_Functions(t *testing.T) {
	ctx := NewMockServiceContext()

	ctx.AddFunction("func1", "handler1")
	ctx.AddFunction("func2", "handler2")

	if ctx.FunctionCount() != 2 {
		t.Errorf("FunctionCount() = %d, want 2", ctx.FunctionCount())
	}

	funcs := ctx.GetFunctions()
	if len(funcs) != 2 {
		t.Errorf("GetFunctions() = %d items, want 2", len(funcs))
	}
}

func TestMockServiceContext_Timestamps(t *testing.T) {
	ctx := NewMockServiceContext()

	now := time.Now()
	ctx.SetRegisteredAt(now)
	ctx.SetLastHeartbeat(now.Add(time.Minute))

	if !ctx.GetRegisteredAt().Equal(now) {
		t.Errorf("GetRegisteredAt() = %v, want %v", ctx.GetRegisteredAt(), now)
	}

	expected := now.Add(time.Minute)
	if !ctx.GetLastHeartbeat().Equal(expected) {
		t.Errorf("GetLastHeartbeat() = %v, want %v", ctx.GetLastHeartbeat(), expected)
	}
}

func TestMockServiceContext_Uptime(t *testing.T) {
	ctx := NewMockServiceContext()

	// Set start time to 1 hour ago
	ctx.SetStartTime(time.Now().Add(-time.Hour))

	uptime := ctx.Uptime()
	if uptime < time.Hour-time.Second || uptime > time.Hour+time.Second {
		t.Errorf("Uptime() = %v, want approximately 1 hour", uptime)
	}
}

func TestMockServiceContext_ConcurrentAccess(t *testing.T) {
	ctx := NewMockServiceContext()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx.SetActiveTasks(idx)
			ctx.SetLastHeartbeat(time.Now())
			_ = ctx.GetActiveTasks()
			_ = ctx.GetLastHeartbeat()
		}(i)
	}

	wg.Wait()
}
