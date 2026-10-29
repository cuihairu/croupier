package sidecar

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// TaskExecutor runs a single task invocation and returns its result. This is
// the seam that decouples TaskRunner from the provider-call machinery: the
// agent wires it to its existing invoke path, while tests can inject a fake.
type TaskExecutor func(ctx context.Context, req *sdkv1.InvokeRequest) ([]byte, error)

// TaskEventReporter publishes task lifecycle events upstream. TaskRunner owns
// event ordering and payloads; the reporter just transports them.
type TaskEventReporter interface {
	ReportTaskEvent(ctx context.Context, event *sdkv1.TaskEvent) error
}

// taskSlot is an in-flight task registration. gen is the per-taskId generation:
// each Start for a taskId claims a fresh generation, and the finishing
// goroutine removes its slot only if the slot still carries its own generation.
// This is the ARCH-014 D1 ownership guard: an old goroutine's cleanup cannot
// delete the slot of a same-ID newer task (fixes the delete-on-finish
// corruption where the first goroutine removed the second task's registration).
type taskSlot struct {
	cancel context.CancelFunc
	gen    uint64
}

// eventRetryPolicy is the bounded-retry policy for upstream event delivery
// (ARCH-014 D2). maxAttempts is the total number of delivery attempts (>=1);
// baseBackoff is the initial delay between attempts, doubling per retry.
type eventRetryPolicy struct {
	maxAttempts int
	baseBackoff time.Duration
}

func defaultEventRetryPolicy() eventRetryPolicy {
	return eventRetryPolicy{maxAttempts: 3, baseBackoff: time.Second}
}

// TaskRunner owns the lifecycle of in-flight agent tasks: it tracks running
// tasks by ID for cancellation, executes them via an injected TaskExecutor,
// and reports started/completed/failed/cancelled events through a
// TaskEventReporter. Extracting this from LocalHandler keeps the handler a
// thin protocol decoder and makes task semantics independently testable.
type TaskRunner struct {
	executor TaskExecutor
	reporter TaskEventReporter
	logger   *slog.Logger

	mu         sync.RWMutex
	tasks      map[string]*taskSlot
	nextGen    uint64
	eventRetry eventRetryPolicy
}

// NewTaskRunner creates a TaskRunner. executor must be non-nil; reporter and
// logger are optional (events are dropped when reporter is nil).
func NewTaskRunner(executor TaskExecutor, reporter TaskEventReporter, logger *slog.Logger) *TaskRunner {
	if executor == nil {
		panic("TaskRunner requires a non-nil executor")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &TaskRunner{
		executor:   executor,
		reporter:   reporter,
		logger:     logger,
		tasks:      make(map[string]*taskSlot),
		eventRetry: defaultEventRetryPolicy(),
	}
}

// SetReporter swaps the event reporter at runtime. Safe for concurrent use.
func (r *TaskRunner) SetReporter(reporter TaskEventReporter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reporter = reporter
}

// Start launches a task asynchronously and returns the resolved task ID. The
// task ID is taken from req metadata (server-provided) when present, so events
// flow back to the correct task_runs row; otherwise a local ID is generated.
//
// ARCH-014 D1: a duplicate Start for a taskId already running is an idempotent
// no-op — it does not launch a second goroutine and does not overwrite the
// existing cancel registration. The taskId is the platform-level task identity
// (one taskId ⇒ one task_runs row ⇒ at most one intended execution), so a
// duplicate is a dispatch/retry artifact, not a new task. A same-ID task can
// only legitimately restart after Cancel removed its slot.
func (r *TaskRunner) Start(req *sdkv1.InvokeRequest) string {
	taskID := resolveTaskID(req)

	r.mu.Lock()
	if _, ok := r.tasks[taskID]; ok {
		r.mu.Unlock()
		r.logger.Warn("duplicate task start ignored: task already in flight", "task_id", taskID)
		return taskID
	}
	r.nextGen++
	gen := r.nextGen
	taskCtx, cancel := context.WithCancel(context.Background())
	r.tasks[taskID] = &taskSlot{cancel: cancel, gen: gen}
	r.mu.Unlock()

	go r.run(taskCtx, taskID, gen, cancel, req)
	return taskID
}

// Cancel requests cancellation of a running task. Returns true if a task with
// the given ID was found and its cancel function invoked. The Server records
// the cancellation intent before forwarding this request; TaskRunner reports
// only the eventual cancelled terminal event from run, so an async intent
// event cannot arrive late and overwrite that terminal state.
func (r *TaskRunner) Cancel(taskID string) bool {
	r.mu.RLock()
	slot, ok := r.tasks[taskID]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	slot.cancel()
	r.mu.Lock()
	// Remove only if the slot is still ours: a re-Start could have replaced it
	// between the read and the write (same generation guard as run's cleanup).
	if cur, ok := r.tasks[taskID]; ok && cur.gen == slot.gen {
		delete(r.tasks, taskID)
	}
	r.mu.Unlock()
	r.logger.Info("task cancel requested", "task_id", taskID)
	return true
}

// Count returns the number of in-flight tasks.
func (r *TaskRunner) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tasks)
}

// SetEventRetryPolicy configures the bounded retry for upstream event delivery
// (ARCH-014 D2). maxAttempts is clamped to >=1; a negative baseBackoff is
// clamped to 0. Safe for concurrent use; applies to events reported after the
// call returns.
func (r *TaskRunner) SetEventRetryPolicy(maxAttempts int, baseBackoff time.Duration) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if baseBackoff < 0 {
		baseBackoff = 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventRetry = eventRetryPolicy{maxAttempts: maxAttempts, baseBackoff: baseBackoff}
}

func (r *TaskRunner) run(ctx context.Context, taskID string, gen uint64, cancel context.CancelFunc, req *sdkv1.InvokeRequest) {
	defer func() {
		r.mu.Lock()
		// Ownership guard (D1): a finishing goroutine removes only its own slot,
		// so an old goroutine's cleanup cannot delete a same-ID newer task's
		// registration (which would leak its slot and make it uncancellable).
		if slot, ok := r.tasks[taskID]; ok && slot.gen == gen {
			delete(r.tasks, taskID)
		}
		r.mu.Unlock()
		cancel() // release the context to avoid leaks
	}()

	r.reportEvent(taskID, "started", "任务开始执行", 0, []byte("null"))

	result, err := r.executor(ctx, req)
	if err != nil {
		eventType := "failed"
		message := err.Error()
		if ctx.Err() != nil {
			eventType = "cancelled"
			message = "任务已取消"
		}
		r.reportEvent(taskID, eventType, message, 0, []byte("null"))
		return
	}

	r.reportEvent(taskID, "completed", "任务执行完成", 100, result)
}

// reportEvent delivers one lifecycle event with bounded retry (D2). A delivery
// failure that exhausts the window is surfaced as a warning, never silently
// dropped: the server reconciles the stuck row to unknown on agent restart
// (D3), so a completed task is never left "running" with its result undelivered
// and ignored.
func (r *TaskRunner) reportEvent(taskID, eventType, message string, progress int32, payload []byte) {
	event := &sdkv1.TaskEvent{
		TaskId:   taskID,
		Type:     eventType,
		Message:  message,
		Progress: progress,
		Payload:  payload,
	}
	if err := r.reportWithRetry(context.Background(), event); err != nil {
		r.logger.Warn("task event delivery failed after retries",
			"task_id", taskID, "event", eventType, "error", err)
	}
}

// reportWithRetry sends event via the reporter with bounded exponential
// backoff. It returns nil on the first success, or the last error when the
// retry window is exhausted. A nil reporter is a no-op success.
func (r *TaskRunner) reportWithRetry(ctx context.Context, event *sdkv1.TaskEvent) error {
	r.mu.RLock()
	policy := r.eventRetry
	r.mu.RUnlock()

	var lastErr error
	for attempt := 1; attempt <= policy.maxAttempts; attempt++ {
		if err := r.report(ctx, event); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == policy.maxAttempts {
			break
		}
		delay := policy.baseBackoff * time.Duration(1<<(attempt-1))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

func (r *TaskRunner) report(ctx context.Context, event *sdkv1.TaskEvent) error {
	if len(event.GetPayload()) == 0 {
		event.Payload = []byte("null")
	}
	r.mu.RLock()
	reporter := r.reporter
	r.mu.RUnlock()
	if reporter == nil || event == nil {
		return nil
	}
	return reporter.ReportTaskEvent(ctx, event)
}

// resolveTaskID returns the server-provided task ID from request metadata, or
// generates a local fallback when absent (backward compatibility).
func resolveTaskID(req *sdkv1.InvokeRequest) string {
	if req.GetMetadata() != nil {
		if id := strings.TrimSpace(req.Metadata["taskId"]); id != "" {
			return id
		}
	}
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

// marshalTaskInvoke is a small helper kept for symmetry with the previous
// inline path; executors that need to round-trip through handleInvoke can use it.
func marshalTaskInvoke(req *sdkv1.InvokeRequest) []byte {
	if req == nil {
		return nil
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return nil
	}
	return data
}
