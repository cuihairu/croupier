package dispatch

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// RegisterTask registers a task routing.
func (d *Dispatcher) RegisterTask(taskID, agentID string) {
	d.registerTask(taskID, agentID)
}

// ListTaskRoutings returns persisted task routing entries.
func (d *Dispatcher) ListTaskRoutings() ([]*TaskRouting, error) {
	if d == nil || d.taskStore == nil {
		return []*TaskRouting{}, nil
	}
	return d.taskStore.List()
}

// UnregisterTask unregisters a task routing.
func (d *Dispatcher) UnregisterTask(taskID string) {
	d.unregisterTask(taskID)
}

func (d *Dispatcher) taskAgentID(ctx context.Context, taskID string) (string, error) {
	d.mu.RLock()
	agentID, ok := d.taskRouting[taskID]
	d.mu.RUnlock()
	if !ok {
		if d.taskStore != nil {
			routing, err := d.taskStore.Get(taskID)
			if err == nil && routing != nil && routing.AgentID != "" {
				d.mu.Lock()
				d.taskRouting[taskID] = routing.AgentID
				d.mu.Unlock()
				return routing.AgentID, nil
			}
		}
		// 跨实例兜底：task routing 与本地 taskStore 都是 per-instance 的，
		// 取消请求落到非发起实例时在此 miss——task_runs 落共享库，从行内
		// AgentID 解析（发起实例 dispatch 时写入）。不回填内存 map：归属
		// 可能随任务终态失效，每次现查共享库更稳。
		d.mu.RLock()
		lookup := d.taskAgentLookup
		d.mu.RUnlock()
		if lookup != nil {
			if remoteAgentID, err := lookup.AgentForTask(ctx, taskID); err == nil && remoteAgentID != "" {
				return remoteAgentID, nil
			}
		}
		return "", fmt.Errorf("task %s not tracked", taskID)
	}
	return agentID, nil
}

func isTerminalEvent(evt *sdkv1.TaskEvent) bool {
	switch strings.ToLower(evt.GetType()) {
	case "done", "completed", "error", "failed", "cancelled", "canceled", "succeeded", "success":
		return true
	default:
		return false
	}
}

// loadTaskRouting loads task routing from persistent store.
func (d *Dispatcher) loadTaskRouting() {
	routings, err := d.taskStore.List()
	if err != nil {
		// Log error but continue with empty cache
		log.Printf("[dispatch] Warning: failed to load task routing from store: %v", err)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Rebuild in-memory cache.
	d.taskRouting = make(map[string]string, len(routings))
	for _, routing := range routings {
		d.taskRouting[routing.TaskID] = routing.AgentID
	}
}

// registerTask registers task routing to both memory cache and persistent store.
func (d *Dispatcher) registerTask(taskID, agentID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Update memory cache
	d.taskRouting[taskID] = agentID

	// Update persistent store
	if err := d.taskStore.Set(taskID, agentID); err != nil {
		// Log error but continue
		log.Printf("[dispatch] Warning: failed to persist task routing: %v", err)
	}
}

// unregisterTask removes task routing from both memory cache and persistent store.
func (d *Dispatcher) unregisterTask(taskID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Remove from memory cache
	delete(d.taskRouting, taskID)

	// Remove from persistent store
	if err := d.taskStore.Delete(taskID); err != nil {
		// Log error but continue
		log.Printf("[dispatch] Warning: failed to delete task routing: %v", err)
	}
}

// CleanupOldTasks removes old task routing entries.
func (d *Dispatcher) CleanupOldTasks(ttl time.Duration) error {
	// Cleanup persistent store
	if err := d.taskStore.Cleanup(ttl); err != nil {
		return err
	}

	// Reload cache to sync with persistent store
	d.loadTaskRouting()

	return nil
}

// Close closes the dispatcher and its resources
func (d *Dispatcher) Close() error {
	// Stop health tracker if enabled
	d.mu.Lock()
	if d.healthTracker != nil {
		d.healthTracker.Stop()
	}
	d.mu.Unlock()

	return d.taskStore.Close()
}
