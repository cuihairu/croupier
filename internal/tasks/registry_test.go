// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package tasks

import (
	"context"
	"sync"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if reg.items == nil {
		t.Error("NewRegistry() items map is nil")
	}
}

func TestRegistrySetAndGet(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()
	runtime := &Runtime{
		taskID: "task-123",
		ctx:    ctx,
	}

	// Get before Set should return not found
	_, ok := reg.Get("task-123")
	if ok {
		t.Error("Get() on empty registry found item, want not found")
	}

	// Set the runtime
	reg.Set("task-123", runtime)

	// Get should now return the runtime
	got, ok := reg.Get("task-123")
	if !ok {
		t.Error("Get() after Set() returned not found")
	}
	if got != runtime {
		t.Errorf("Get() = %v, want %v", got, runtime)
	}

	// Check taskID
	if got.TaskID() != "task-123" {
		t.Errorf("TaskID = %v, want task-123", got.TaskID())
	}
}

func TestRegistryDelete(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()
	runtime := &Runtime{
		taskID: "task-456",
		ctx:    ctx,
	}

	// Set then delete
	reg.Set("task-456", runtime)
	reg.Delete("task-456")

	// Get should return not found
	_, ok := reg.Get("task-456")
	if ok {
		t.Error("Get() after Delete() found item, want not found")
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			taskID := string(rune('a'+n%26)) + "-task"
			runtime := &Runtime{
				taskID: taskID,
				ctx:    ctx,
			}
			reg.Set(taskID, runtime)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			taskID := string(rune('a'+n%26)) + "-task"
			reg.Get(taskID)
		}(i)
	}

	// Concurrent deletes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			taskID := string(rune('a'+n%26)) + "-task"
			reg.Delete(taskID)
		}(i)
	}

	wg.Wait()

	// Registry should still be functional
	runtime := &Runtime{
		taskID: "final-task",
		ctx:    ctx,
	}
	reg.Set("final-task", runtime)

	got, ok := reg.Get("final-task")
	if !ok {
		t.Error("Get() after concurrent operations returned not found")
	}
	if got.TaskID() != "final-task" {
		t.Errorf("TaskID = %v, want final-task", got.TaskID())
	}
}

func TestRegistryUpdate(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()

	runtime1 := &Runtime{
		taskID: "task-789",
		ctx:    ctx,
	}
	reg.Set("task-789", runtime1)

	// Update with new runtime
	ctx2, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime2 := &Runtime{
		taskID: "task-789",
		ctx:    ctx2,
		cancel: cancel,
	}
	reg.Set("task-789", runtime2)

	// Get should return the updated runtime
	got, ok := reg.Get("task-789")
	if !ok {
		t.Fatal("Get() returned not found")
	}
	if got == runtime1 {
		t.Error("Get() returned old runtime, want updated runtime")
	}
	if got == runtime2 {
		// Expected - they're the same pointer
		return
	}
	t.Errorf("Get() returned unexpected runtime")
}
