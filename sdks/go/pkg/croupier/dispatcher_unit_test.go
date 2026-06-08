package croupier

import (
	"sync"
	"testing"
)

func TestGetDispatcher_Singleton(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d1 := GetDispatcher()
	d2 := GetDispatcher()

	if d1 != d2 {
		t.Error("GetDispatcher should return singleton instance")
	}
}

func TestResetDispatcher(t *testing.T) {
	ResetDispatcher()

	d := GetDispatcher()
	d.Enqueue(func() {})

	ResetDispatcher()

	d2 := GetDispatcher()
	if d2.GetPendingCount() != 0 {
		t.Errorf("expected 0 pending after reset, got %d", d2.GetPendingCount())
	}
}

func TestMainThreadDispatcher_Initialize(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()

	if d.IsInitialized() {
		t.Error("should not be initialized before Initialize()")
	}

	d.Initialize()

	if !d.IsInitialized() {
		t.Error("should be initialized after Initialize()")
	}
}

func TestMainThreadDispatcher_Enqueue(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()
	d.Initialize()

	// When called from the same goroutine as Initialize(), Enqueue executes immediately
	// When called from a different goroutine, it queues
	called := false
	d.Enqueue(func() {
		called = true
	})

	// Since we're on the same goroutine as Initialize(), it should have executed immediately
	if !called {
		t.Error("expected callback to execute immediately on main goroutine")
	}
	if d.GetPendingCount() != 0 {
		t.Errorf("expected 0 pending (executed immediately), got %d", d.GetPendingCount())
	}
}

func TestMainThreadDispatcher_Enqueue_NilCallback(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()
	d.Initialize()

	// Should not panic
	d.Enqueue(nil)

	if d.GetPendingCount() != 0 {
		t.Error("nil callback should not be queued")
	}
}

func TestMainThreadDispatcher_ProcessQueue(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()
	// Initialize on a different goroutine so Enqueue queues instead of executing
	initDone := make(chan struct{})
	go func() {
		d.Initialize()
		close(initDone)
	}()
	<-initDone

	processed := 0
	for i := 0; i < 5; i++ {
		d.Enqueue(func() {
			processed++
		})
	}

	count := d.ProcessQueue()

	if count != 5 {
		t.Errorf("expected 5 processed, got %d", count)
	}
	if processed != 5 {
		t.Errorf("expected 5 callbacks executed, got %d", processed)
	}
	if d.GetPendingCount() != 0 {
		t.Errorf("expected 0 pending, got %d", d.GetPendingCount())
	}
}

func TestMainThreadDispatcher_ProcessQueue_Empty(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()
	d.Initialize()

	count := d.ProcessQueue()
	if count != 0 {
		t.Errorf("expected 0 processed, got %d", count)
	}
}

func TestMainThreadDispatcher_ProcessQueueWithLimit(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()
	initDone := make(chan struct{})
	go func() {
		d.Initialize()
		close(initDone)
	}()
	<-initDone

	processed := 0
	for i := 0; i < 10; i++ {
		d.Enqueue(func() {
			processed++
		})
	}

	count := d.ProcessQueueWithLimit(3)

	if count != 3 {
		t.Errorf("expected 3 processed, got %d", count)
	}
	if processed != 3 {
		t.Errorf("expected 3 callbacks executed, got %d", processed)
	}
	if d.GetPendingCount() != 7 {
		t.Errorf("expected 7 pending, got %d", d.GetPendingCount())
	}
}

func TestMainThreadDispatcher_ProcessQueueWithLimit_ZeroOrNegative(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()
	initDone := make(chan struct{})
	go func() {
		d.Initialize()
		close(initDone)
	}()
	<-initDone
	d.SetMaxProcessPerFrame(5)

	processed := 0
	for i := 0; i < 10; i++ {
		d.Enqueue(func() {
			processed++
		})
	}

	// Zero should use maxProcessPerFrame
	count := d.ProcessQueueWithLimit(0)
	if count != 5 {
		t.Errorf("expected 5 processed (using max), got %d", count)
	}
}

func TestMainThreadDispatcher_ProcessQueue_PanicRecovery(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()
	initDone := make(chan struct{})
	go func() {
		d.Initialize()
		close(initDone)
	}()
	<-initDone

	d.Enqueue(func() {
		panic("test panic")
	})

	d.Enqueue(func() {
		// This should still execute despite previous panic
	})

	count := d.ProcessQueue()
	if count != 2 {
		t.Errorf("expected 2 processed (including panicking one), got %d", count)
	}
}

func TestMainThreadDispatcher_GetPendingCount(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()

	if d.GetPendingCount() != 0 {
		t.Errorf("expected 0 pending initially, got %d", d.GetPendingCount())
	}

	d.Enqueue(func() {})
	d.Enqueue(func() {})

	if d.GetPendingCount() != 2 {
		t.Errorf("expected 2 pending, got %d", d.GetPendingCount())
	}
}

func TestMainThreadDispatcher_IsMainGoroutine(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()

	// Before initialization, should return false
	if d.IsMainGoroutine() {
		t.Error("should not be main goroutine before initialization")
	}

	d.Initialize()

	// In test goroutine, this might or might not be the main goroutine
	// Just verify it doesn't panic
	_ = d.IsMainGoroutine()
}

func TestMainThreadDispatcher_SetMaxProcessPerFrame(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()

	d.SetMaxProcessPerFrame(50)
	if d.maxProcessPerFrame != 50 {
		t.Errorf("expected maxProcessPerFrame=50, got %d", d.maxProcessPerFrame)
	}

	// Zero or negative should reset to default
	d.SetMaxProcessPerFrame(0)
	if d.maxProcessPerFrame != 1000 {
		t.Errorf("expected maxProcessPerFrame=1000, got %d", d.maxProcessPerFrame)
	}

	d.SetMaxProcessPerFrame(-1)
	if d.maxProcessPerFrame != 1000 {
		t.Errorf("expected maxProcessPerFrame=1000, got %d", d.maxProcessPerFrame)
	}
}

func TestMainThreadDispatcher_Clear(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()

	d.Enqueue(func() {})
	d.Enqueue(func() {})

	if d.GetPendingCount() != 2 {
		t.Errorf("expected 2 pending, got %d", d.GetPendingCount())
	}

	d.Clear()

	if d.GetPendingCount() != 0 {
		t.Errorf("expected 0 pending after clear, got %d", d.GetPendingCount())
	}
}

func TestEnqueueWithData(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()
	initDone := make(chan struct{})
	go func() {
		d.Initialize()
		close(initDone)
	}()
	<-initDone

	var result string
	EnqueueWithData(d, func(s string) {
		result = s
	}, "hello")

	d.ProcessQueue()

	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestEnqueueWithData_NilCallback(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()
	d.Initialize()

	// Should not panic
	EnqueueWithData(d, nil, "data")

	if d.GetPendingCount() != 0 {
		t.Error("nil callback should not be queued")
	}
}

func TestMainThreadDispatcher_ConcurrentEnqueue(t *testing.T) {
	ResetDispatcher()
	defer ResetDispatcher()

	d := GetDispatcher()
	d.Initialize()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Enqueue(func() {})
		}()
	}

	wg.Wait()

	if d.GetPendingCount() != 100 {
		t.Errorf("expected 100 pending, got %d", d.GetPendingCount())
	}
}

func TestGetGoroutineID(t *testing.T) {
	id := getGoroutineID()
	if id < 0 {
		t.Errorf("expected non-negative goroutine ID, got %d", id)
	}
}
