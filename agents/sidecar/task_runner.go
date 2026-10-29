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
// event ordering and payloads; the reporter just transports them. Per the
// ARCH-014 protocol (D2), a nil return from ReportTaskEvent IS the Server's
// ACK: the Server persists the event (or intentionally drops a late terminal
// event) before responding.
type TaskEventReporter interface {
	ReportTaskEvent(ctx context.Context, event *sdkv1.TaskEvent) error
}

// taskEntry is the per-task tracking record. generation is monotonically
// assigned by Start; run's cleanup removes an entry only when its own
// generation still matches, so a winding-down goroutine can never delete a
// newer same-ID task's tracking (the ARCH-015 "old cleanup must not delete a
// new same-ID task" invariant).
type taskEntry struct {
	generation uint64
	cancel     context.CancelFunc
}

const (
	// defaultMaxEventRetries bounds the terminal-event retry loop (D2). A
	// terminal event is retried until the Server ACKs or this many attempts
	// elapse; beyond that the terminal event is dropped and the row is left to
	// the server-side reaper (D3).
	defaultMaxEventRetries = 5
	// defaultEventRetryBase is the initial backoff between terminal-event
	// retries; it doubles per attempt up to maxEventRetryBackoff.
	defaultEventRetryBase = 500 * time.Millisecond
	// maxEventRetryBackoff caps both the per-attempt RPC context timeout and
	// the inter-attempt backoff.
	maxEventRetryBackoff = 5 * time.Second
)

// TaskRunner owns the lifecycle of in-flight agent tasks: it tracks running
// tasks by ID for cancellation, executes them via an injected TaskExecutor,
// and reports started/completed/failed/cancelled events through a
// TaskEventReporter. Extracting this from LocalHandler keeps the handler a
// thin protocol decoder and makes task semantics independently testable.
type TaskRunner struct {
	executor TaskExecutor
	reporter TaskEventReporter
	logger   *slog.Logger

	mu      sync.RWMutex
	tasks   map[string]*taskEntry
	nextGen uint64

	// Terminal-event retry policy (D2). Guarded by mu; settable via
	// SetEventRetryPolicy (tests shorten it).
	maxEventRetries int
	eventRetryBase  time.Duration
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
		executor:        executor,
		reporter:        reporter,
		logger:          logger,
		tasks:           make(map[string]*taskEntry),
		maxEventRetries: defaultMaxEventRetries,
		eventRetryBase:  defaultEventRetryBase,
	}
}

// SetReporter swaps the event reporter at runtime. Safe for concurrent use.
func (r *TaskRunner) SetReporter(reporter TaskEventReporter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reporter = reporter
}

// SetEventRetryPolicy configures the terminal-event retry policy (D2).
// maxRetries <= 0 forces a single attempt (no retry); baseBackoff <= 0 falls
// back to the default. Tests use this to keep retries fast and bounded.
func (r *TaskRunner) SetEventRetryPolicy(maxRetries int, baseBackoff time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if maxRetries <= 0 {
		maxRetries = 1
	}
	r.maxEventRetries = maxRetries
	if baseBackoff <= 0 {
		baseBackoff = defaultEventRetryBase
	}
	r.eventRetryBase = baseBackoff
}

// Start launches a task asynchronously and returns the resolved task ID. The
// task ID is taken from req metadata (server-provided) when present, so events
// flow back to the correct task_runs row; otherwise a local ID is generated.
//
// ARCH-014 D1: a Start for a task ID that is already in-flight is an
// idempotent no-op — the existing task ID is returned without launching a
// second execution and without reporting a second started event. A duplicate
// Start is either transport re-delivery or an operator re-dispatch; in both
// cases the first execution is the real one and re-running must use a new task
// ID. This guarantees one coherent event stream per task ID.
func (r *TaskRunner) Start(req *sdkv1.InvokeRequest) string {
	taskID := resolveTaskID(req)

	r.mu.Lock()
	if _, exists := r.tasks[taskID]; exists {
		r.mu.Unlock()
		r.logger.Info("task start ignored: task id already in-flight", "task_id", taskID)
		return taskID
	}
	r.nextGen++
	gen := r.nextGen
	taskCtx, cancel := context.WithCancel(context.Background())
	r.tasks[taskID] = &taskEntry{generation: gen, cancel: cancel}
	r.mu.Unlock()

	go r.run(taskCtx, taskID, gen, cancel, req)
	return taskID
}

// Cancel requests cancellation of a running task. Returns true if a task with
// the given ID was found and its cancel function invoked. The Server records
// the cancellation intent before forwarding this request; TaskRunner reports
// only the eventual cancelled terminal event from run, so an async intent
// event cannot arrive late and overwrite that terminal state.
//
// ARCH-014 D1: Cancel does NOT delete the tracking entry eagerly. The entry
// lives until the executor actually returns; run's cleanup is the single
// deletion point (generation-checked). This keeps a winding-down task visible
// to Count and makes a Start arriving during cancel wind-down a no-op (the ID
// is still in-flight), which avoids double-executing while the first execution
// has not exited.
func (r *TaskRunner) Cancel(taskID string) bool {
	r.mu.RLock()
	entry, ok := r.tasks[taskID]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	entry.cancel()
	r.logger.Info("task cancel requested", "task_id", taskID)
	return true
}

// Count returns the number of in-flight tasks (including those winding down
// after a cancel, since their entry lives until the executor returns).
func (r *TaskRunner) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tasks)
}

func (r *TaskRunner) run(ctx context.Context, taskID string, gen uint64, cancel context.CancelFunc, req *sdkv1.InvokeRequest) {
	defer func() {
		r.mu.Lock()
		if entry, ok := r.tasks[taskID]; ok && entry.generation == gen {
			delete(r.tasks, taskID)
		}
		r.mu.Unlock()
		cancel() // release the context to avoid leaks
	}()

	_ = r.report(context.Background(), &sdkv1.TaskEvent{
		TaskId:   taskID,
		Type:     "started",
		Message:  "任务开始执行",
		Progress: 0,
		Payload:  []byte("null"),
	})

	result, err := r.executor(ctx, req)
	if err != nil {
		eventType := "failed"
		message := err.Error()
		if ctx.Err() != nil {
			eventType = "cancelled"
			message = "任务已取消"
		}
		// ARCH-014 D2: the terminal event is the durable boundary. Retry it
		// until the Server ACKs (or the bounded policy gives up), so a
		// transport blip does not silently leave the row at running.
		_ = r.reportTerminal(&sdkv1.TaskEvent{
			TaskId:   taskID,
			Type:     eventType,
			Message:  message,
			Progress: 0,
			Payload:  []byte("null"),
		})
		return
	}

	_ = r.reportTerminal(&sdkv1.TaskEvent{
		TaskId:   taskID,
		Type:     "completed",
		Message:  "任务执行完成",
		Progress: 100,
		Payload:  result,
	})
}

// reportTerminal reports a terminal event with bounded retry until the Server
// ACKs (nil reporter response), per ARCH-014 D2. A nil reporter drops the
// event immediately. After the bound, the terminal event is dropped and the
// row is left to the server-side reaper (D3). Retry is idempotent on the
// Server side: the terminal-guard drop responds success without re-appending.
func (r *TaskRunner) reportTerminal(event *sdkv1.TaskEvent) error {
	r.mu.RLock()
	maxRetries := r.maxEventRetries
	baseBackoff := r.eventRetryBase
	r.mu.RUnlock()

	backoff := baseBackoff
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), maxEventRetryBackoff)
		err := r.report(ctx, event)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == maxRetries {
			break
		}
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxEventRetryBackoff {
			backoff = maxEventRetryBackoff
		}
	}
	if lastErr != nil {
		r.logger.Warn("task terminal event not acknowledged; left to server reaper",
			"task_id", event.GetTaskId(), "event_type", event.GetType(), "err", lastErr)
	}
	return lastErr
}

func (r *TaskRunner) report(ctx context.Context, event *sdkv1.TaskEvent) error {
	if event == nil {
		return nil
	}
	if len(event.GetPayload()) == 0 {
		event.Payload = []byte("null")
	}
	r.mu.RLock()
	reporter := r.reporter
	r.mu.RUnlock()
	if reporter == nil {
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
