// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"sync"
	"testing"
	"time"
)

func TestGetDispatcher_coverage(t *testing.T) {
	t.Run("returns non-nil dispatcher", func(t *testing.T) {
		dispatcher := GetDispatcher()
		if dispatcher == nil {
			t.Error("GetDispatcher() should not return nil")
		}
	})
}

func TestResetDispatcher_coverage(t *testing.T) {
	t.Run("reset does not panic", func(t *testing.T) {
		ResetDispatcher()
	})
}

func TestMainThreadDispatcher_Initialize_coverage(t *testing.T) {
	dispatcher := GetDispatcher()
	dispatcher.Initialize()
	if !dispatcher.IsInitialized() {
		t.Error("Dispatcher should be initialized")
	}
}

func TestMainThreadDispatcher_Enqueue_coverage(t *testing.T) {
	dispatcher := GetDispatcher()
	dispatcher.Enqueue(func() {})
	dispatcher.Enqueue(nil)
}

func TestMainThreadDispatcher_ProcessQueue_coverage(t *testing.T) {
	ResetDispatcher()
	dispatcher := GetDispatcher()
	processed := dispatcher.ProcessQueue()
	if processed < 0 {
		t.Errorf("ProcessQueue() returned negative: %d", processed)
	}
}

func TestMainThreadDispatcher_ProcessQueueWithLimit_coverage(t *testing.T) {
	dispatcher := GetDispatcher()
	for i := 0; i < 10; i++ {
		dispatcher.Enqueue(func() {})
	}
	
	limits := []int{0, 5, 100, -1}
	for _, limit := range limits {
		processed := dispatcher.ProcessQueueWithLimit(limit)
		t.Logf("ProcessQueueWithLimit(%d) = %d", limit, processed)
	}
}

func TestMainThreadDispatcher_GetPendingCount_coverage(t *testing.T) {
	ResetDispatcher()
	dispatcher := GetDispatcher()
	
	initial := dispatcher.GetPendingCount()
	if initial != 0 {
		t.Errorf("Initial pending count should be 0, got %d", initial)
	}
	
	dispatcher.Enqueue(func() {})
	after := dispatcher.GetPendingCount()
	t.Logf("Pending count after enqueue: %d", after)
}

func TestMainThreadDispatcher_IsMainGoroutine_coverage(t *testing.T) {
	dispatcher := GetDispatcher()
	isMain := dispatcher.IsMainGoroutine()
	t.Logf("IsMainGoroutine: %v", isMain)
	
	done := make(chan bool)
	go func() {
		isMain := dispatcher.IsMainGoroutine()
		t.Logf("IsMainGoroutine from goroutine: %v", isMain)
		done <- true
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Goroutine did not complete")
	}
}

func TestMainThreadDispatcher_SetMaxProcessPerFrame_coverage(t *testing.T) {
	dispatcher := GetDispatcher()
	
	values := []int{-1, 0, 1, 10, 100, 1000}
	for _, val := range values {
		dispatcher.SetMaxProcessPerFrame(val)
	}
}

func TestMainThreadDispatcher_Clear_coverage(t *testing.T) {
	ResetDispatcher()
	dispatcher := GetDispatcher()
	
	for i := 0; i < 10; i++ {
		dispatcher.Enqueue(func() {})
	}
	
	before := dispatcher.GetPendingCount()
	dispatcher.Clear()
	after := dispatcher.GetPendingCount()
	
	t.Logf("Before clear: %d, after clear: %d", before, after)
	
	if after != 0 {
		t.Errorf("After clear should be 0, got %d", after)
	}
}

func TestMainThreadDispatcher_Concurrency_coverage(t *testing.T) {
	ResetDispatcher()
	dispatcher := GetDispatcher()
	
	const numGoroutines = 10
	const operationsPerGoroutine = 50
	
	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				dispatcher.Enqueue(func() {})
				dispatcher.GetPendingCount()
			}
		}()
	}
	
	wg.Wait()
	t.Logf("Pending count after concurrent enqueues: %d", dispatcher.GetPendingCount())
}

func TestGetGoroutineID_coverage(t *testing.T) {
	id := getGoroutineID()
	t.Logf("Current goroutine ID: %d", id)
	
	ids := make(map[int64]bool)
	const numGoroutines = 10
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := getGoroutineID()
			mu.Lock()
			ids[id] = true
			mu.Unlock()
		}()
	}
	
	wg.Wait()
	t.Logf("Unique goroutine IDs: %d", len(ids))
}
