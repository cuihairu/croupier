// Copyright 2025 Croupier Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package croupier

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func resetDispatcherForTest() {
	ResetDispatcher()
	GetDispatcher().Initialize()
}

func TestInitialize_SetsMainGoroutine(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	if !d.IsInitialized() {
		t.Error("expected dispatcher to be initialized")
	}
	if !d.IsMainGoroutine() {
		t.Error("expected to be on main goroutine after initialize")
	}
}

func TestEnqueue_FromMainGoroutine_ExecutesImmediately(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	executed := false

	d.Enqueue(func() {
		executed = true
	})

	if !executed {
		t.Error("expected callback to be executed immediately on main goroutine")
	}
}

func TestEnqueue_FromBackgroundGoroutine_QueuesForLater(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	executed := atomic.Bool{}
	done := make(chan struct{})

	go func() {
		d.Enqueue(func() {
			executed.Store(true)
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for background goroutine")
	}

	// Should not be executed yet
	if executed.Load() {
		t.Error("expected callback to NOT be executed yet")
	}
	if d.GetPendingCount() != 1 {
		t.Errorf("expected pending count 1, got %d", d.GetPendingCount())
	}

	// Process the queue
	processed := d.ProcessQueue()

	if processed != 1 {
		t.Errorf("expected 1 processed, got %d", processed)
	}
	if !executed.Load() {
		t.Error("expected callback to be executed after ProcessQueue")
	}
	if d.GetPendingCount() != 0 {
		t.Errorf("expected pending count 0, got %d", d.GetPendingCount())
	}
}

func TestEnqueueWithData_PassesDataCorrectly(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	var receivedData string
	done := make(chan struct{})

	go func() {
		EnqueueWithData(d, func(data string) {
			receivedData = data
		}, "test-data")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for background goroutine")
	}

	d.ProcessQueue()

	if receivedData != "test-data" {
		t.Errorf("expected 'test-data', got '%s'", receivedData)
	}
}

func TestProcessQueue_RespectsMaxCount(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	count := atomic.Int32{}
	var wg sync.WaitGroup

	// Enqueue 10 callbacks from background goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Enqueue(func() {
				count.Add(1)
			})
		}()
	}

	wg.Wait()

	if d.GetPendingCount() != 10 {
		t.Errorf("expected pending count 10, got %d", d.GetPendingCount())
	}

	// Process only 5
	processed := d.ProcessQueueWithLimit(5)

	if processed != 5 {
		t.Errorf("expected 5 processed, got %d", processed)
	}
	if count.Load() != 5 {
		t.Errorf("expected count 5, got %d", count.Load())
	}
	if d.GetPendingCount() != 5 {
		t.Errorf("expected pending count 5, got %d", d.GetPendingCount())
	}

	// Process remaining
	processed = d.ProcessQueue()
	if processed != 5 {
		t.Errorf("expected 5 processed, got %d", processed)
	}
	if count.Load() != 10 {
		t.Errorf("expected count 10, got %d", count.Load())
	}
}

func TestProcessQueue_HandlesPanics(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	results := make([]int, 0, 3)
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		d.Enqueue(func() {
			mu.Lock()
			results = append(results, 1)
			mu.Unlock()
		})
		d.Enqueue(func() {
			panic("test panic")
		})
		d.Enqueue(func() {
			mu.Lock()
			results = append(results, 3)
			mu.Unlock()
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for background goroutine")
	}

	// Should process all callbacks even with panic
	processed := d.ProcessQueue()

	if processed != 3 {
		t.Errorf("expected 3 processed, got %d", processed)
	}

	mu.Lock()
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	mu.Unlock()
}

func TestClear_RemovesAllPendingCallbacks(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Enqueue(func() {})
		}()
	}

	wg.Wait()

	if d.GetPendingCount() != 5 {
		t.Errorf("expected pending count 5, got %d", d.GetPendingCount())
	}

	d.Clear()

	if d.GetPendingCount() != 0 {
		t.Errorf("expected pending count 0, got %d", d.GetPendingCount())
	}
}

func TestSetMaxProcessPerFrame_LimitsProcessing(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	d.SetMaxProcessPerFrame(3)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Enqueue(func() {})
		}()
	}

	wg.Wait()

	processed := d.ProcessQueue()
	if processed != 3 {
		t.Errorf("expected 3 processed, got %d", processed)
	}

	// Reset to unlimited
	d.SetMaxProcessPerFrame(0)
}

func TestEnqueue_NilCallback_IsIgnored(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	initialCount := d.GetPendingCount()
	done := make(chan struct{})

	go func() {
		d.Enqueue(nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for background goroutine")
	}

	if d.GetPendingCount() != initialCount {
		t.Errorf("expected pending count %d, got %d", initialCount, d.GetPendingCount())
	}
}

func TestConcurrentEnqueue_IsThreadSafe(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	counter := atomic.Int32{}
	const goroutineCount = 10
	const enqueuesPerGoroutine = 100

	var wg sync.WaitGroup
	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < enqueuesPerGoroutine; j++ {
				d.Enqueue(func() {
					counter.Add(1)
				})
			}
		}()
	}

	wg.Wait()

	// Process all
	totalProcessed := 0
	for {
		processed := d.ProcessQueueWithLimit(100)
		if processed == 0 {
			break
		}
		totalProcessed += processed
	}

	expected := int32(goroutineCount * enqueuesPerGoroutine)
	if counter.Load() != expected {
		t.Errorf("expected counter %d, got %d", expected, counter.Load())
	}
	if totalProcessed != int(expected) {
		t.Errorf("expected total processed %d, got %d", expected, totalProcessed)
	}
}

func TestIsMainGoroutine_ReturnsFalse_OnBackgroundGoroutine(t *testing.T) {
	resetDispatcherForTest()
	defer ResetDispatcher()

	d := GetDispatcher()
	isMain := atomic.Bool{}
	isMain.Store(true)
	done := make(chan struct{})

	go func() {
		isMain.Store(d.IsMainGoroutine())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for background goroutine")
	}

	if isMain.Load() {
		t.Error("expected IsMainGoroutine to return false on background goroutine")
	}
}

func TestGetGoroutineID(t *testing.T) {
	// Test that getGoroutineID returns consistent values
	id1 := getGoroutineID()
	id2 := getGoroutineID()

	if id1 != id2 {
		t.Errorf("expected same goroutine ID, got %d and %d", id1, id2)
	}

	if id1 < 0 {
		t.Errorf("expected non-negative goroutine ID, got %d", id1)
	}

	// Test that different goroutines have different IDs
	var otherId int64
	done := make(chan struct{})
	go func() {
		otherId = getGoroutineID()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for background goroutine")
	}

	if id1 == otherId {
		t.Errorf("expected different goroutine IDs, but both were %d", id1)
	}
}
