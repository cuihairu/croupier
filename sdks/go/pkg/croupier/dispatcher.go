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

package croupier

import (
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// MainThreadDispatcher ensures callbacks execute on the main goroutine.
//
// Usage:
//  1. Call Initialize() once on the main goroutine at startup
//  2. Call ProcessQueue() in your main loop/game loop
//  3. Use Enqueue() from any goroutine to schedule callbacks
//
// Example:
//
//	// In main function
//	dispatcher := croupier.GetDispatcher()
//	dispatcher.Initialize()
//
//	// From any goroutine
//	dispatcher.Enqueue(func() {
//	    // This runs on the main goroutine
//	    updateUI()
//	})
//
//	// In game loop
//	for running {
//	    dispatcher.ProcessQueue()
//	    // ... game logic
//	}
type MainThreadDispatcher struct {
	mu                 sync.Mutex
	queue              []func()
	mainGoroutineID    int64
	initialized        atomic.Bool
	maxProcessPerFrame int
}

var (
	dispatcherInstance *MainThreadDispatcher
	dispatcherOnce     sync.Once
	dispatcherMu       sync.Mutex
)

// GetDispatcher returns the singleton instance of MainThreadDispatcher.
func GetDispatcher() *MainThreadDispatcher {
	dispatcherOnce.Do(func() {
		dispatcherInstance = &MainThreadDispatcher{
			queue:              make([]func(), 0, 64),
			maxProcessPerFrame: 1000,
			mainGoroutineID:    -1,
		}
	})
	return dispatcherInstance
}

// ResetDispatcher resets the singleton instance. Primarily for testing.
func ResetDispatcher() {
	dispatcherMu.Lock()
	defer dispatcherMu.Unlock()

	if dispatcherInstance != nil {
		dispatcherInstance.Clear()
		dispatcherInstance.initialized.Store(false)
		dispatcherInstance.mainGoroutineID = -1
	}
	dispatcherInstance = nil
	dispatcherOnce = sync.Once{}
}

// Initialize initializes the dispatcher. Must be called once on the main goroutine.
func (d *MainThreadDispatcher) Initialize() {
	d.mainGoroutineID = getGoroutineID()
	d.initialized.Store(true)
}

// IsInitialized returns true if the dispatcher has been initialized.
func (d *MainThreadDispatcher) IsInitialized() bool {
	return d.initialized.Load()
}

// Enqueue adds a callback to be executed on the main goroutine.
// If called from the main goroutine and initialized, executes immediately.
func (d *MainThreadDispatcher) Enqueue(callback func()) {
	if callback == nil {
		return
	}

	// If already on main goroutine and initialized, execute immediately
	if d.initialized.Load() && d.IsMainGoroutine() {
		func() {
			defer func() {
				recover() // Swallow panic to prevent crash
			}()
			callback()
		}()
		return
	}

	d.mu.Lock()
	d.queue = append(d.queue, callback)
	d.mu.Unlock()
}

// EnqueueWithData adds a callback with data to be executed on the main goroutine.
func EnqueueWithData[T any](d *MainThreadDispatcher, callback func(T), data T) {
	if callback == nil {
		return
	}
	d.Enqueue(func() {
		callback(data)
	})
}

// ProcessQueue processes queued callbacks on the main goroutine.
// Call this from your main loop. Returns the number of callbacks processed.
func (d *MainThreadDispatcher) ProcessQueue() int {
	return d.ProcessQueueWithLimit(d.maxProcessPerFrame)
}

// ProcessQueueWithLimit processes up to maxCount queued callbacks.
// Returns the number of callbacks processed.
func (d *MainThreadDispatcher) ProcessQueueWithLimit(maxCount int) int {
	if maxCount <= 0 {
		maxCount = d.maxProcessPerFrame
	}

	d.mu.Lock()
	if len(d.queue) == 0 {
		d.mu.Unlock()
		return 0
	}

	// Take at most maxCount items
	count := min(len(d.queue), maxCount)

	// Copy callbacks to process
	toProcess := make([]func(), count)
	copy(toProcess, d.queue[:count])

	// Remove processed items from queue
	d.queue = d.queue[count:]
	d.mu.Unlock()

	// Process callbacks outside the lock
	processed := 0
	for _, callback := range toProcess {
		func() {
			defer func() {
				recover() // Continue processing even if one callback panics
			}()
			callback()
		}()
		processed++
	}

	return processed
}

// GetPendingCount returns the number of pending callbacks in the queue.
func (d *MainThreadDispatcher) GetPendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.queue)
}

// IsMainGoroutine returns true if the current goroutine is the main goroutine.
func (d *MainThreadDispatcher) IsMainGoroutine() bool {
	if d.mainGoroutineID < 0 {
		return false
	}
	return getGoroutineID() == d.mainGoroutineID
}

// SetMaxProcessPerFrame sets the maximum number of callbacks to process per frame.
func (d *MainThreadDispatcher) SetMaxProcessPerFrame(max int) {
	if max > 0 {
		d.maxProcessPerFrame = max
	} else {
		d.maxProcessPerFrame = 1000
	}
}

// Clear removes all pending callbacks from the queue.
func (d *MainThreadDispatcher) Clear() {
	d.mu.Lock()
	d.queue = d.queue[:0]
	d.mu.Unlock()
}

// getGoroutineID returns a unique identifier for the current goroutine.
// This uses runtime stack trace parsing to extract the goroutine ID.
func getGoroutineID() int64 {
	buf := make([]byte, 64)
	n := runtime.Stack(buf, false)
	// Stack trace starts with "goroutine <id> ["
	idStr := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	if idx := strings.Index(idStr, " "); idx > 0 {
		idStr = idStr[:idx]
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return -1
	}
	return id
}
