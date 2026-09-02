package agent

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

// TaskRunner owns the lifecycle of in-flight agent tasks: it tracks running
// tasks by ID for cancellation, executes them via an injected TaskExecutor,
// and reports started/completed/failed/cancelled events through a
// TaskEventReporter. Extracting this from LocalHandler keeps the handler a
// thin protocol decoder and makes task semantics independently testable.
type TaskRunner struct {
	executor TaskExecutor
	reporter TaskEventReporter
	logger   *slog.Logger

	mu    sync.RWMutex
	tasks map[string]context.CancelFunc
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
		executor: executor,
		reporter: reporter,
		logger:   logger,
		tasks:    make(map[string]context.CancelFunc),
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
func (r *TaskRunner) Start(req *sdkv1.InvokeRequest) string {
	taskID := resolveTaskID(req)

	taskCtx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.tasks[taskID] = cancel
	r.mu.Unlock()

	go r.run(taskCtx, taskID, cancel, req)
	return taskID
}

// Cancel requests cancellation of a running task. Returns true if a task with
// the given ID was found and its cancel function invoked. The Server records
// the cancellation intent before forwarding this request; TaskRunner reports
// only the eventual cancelled terminal event from run, so an async intent
// event cannot arrive late and overwrite that terminal state.
func (r *TaskRunner) Cancel(taskID string) bool {
	r.mu.RLock()
	cancel, ok := r.tasks[taskID]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	cancel()
	r.mu.Lock()
	delete(r.tasks, taskID)
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

func (r *TaskRunner) run(ctx context.Context, taskID string, cancel context.CancelFunc, req *sdkv1.InvokeRequest) {
	defer func() {
		r.mu.Lock()
		delete(r.tasks, taskID)
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
		_ = r.report(context.Background(), &sdkv1.TaskEvent{
			TaskId:   taskID,
			Type:     eventType,
			Message:  message,
			Progress: 0,
			Payload:  []byte("null"),
		})
		return
	}

	_ = r.report(context.Background(), &sdkv1.TaskEvent{
		TaskId:   taskID,
		Type:     "completed",
		Message:  "任务执行完成",
		Progress: 100,
		Payload:  result,
	})
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
