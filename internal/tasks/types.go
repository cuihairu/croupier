package tasks

import "context"

const (
	StatusQueued          = "queued"
	StatusDispatching     = "dispatching"
	StatusRunning         = "running"
	StatusSucceeded       = "succeeded"
	StatusFailed          = "failed"
	StatusCancelRequested = "cancel_requested"
	StatusCancelled       = "cancelled"
	StatusTimedOut        = "timed_out"
	// StatusUnknown is the terminal-adjacent state used when the platform
	// cannot determine whether a task's game side effect executed (e.g. the
	// agent restarted mid-task). It is terminal so a late event cannot regress
	// it (ARCH-014 D3/D4), and it is deliberately surfaced for manual
	// reconciliation rather than silently retried.
	StatusUnknown = "unknown"
)

// IsTerminalStatus reports whether a task status cannot transition again.
func IsTerminalStatus(status string) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut, StatusUnknown:
		return true
	default:
		return false
	}
}

// TerminalStatuses returns the statuses protected from late event updates.
// The returned slice is deliberately new so callers cannot mutate shared
// state.
func TerminalStatuses() []string {
	return []string{StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut, StatusUnknown}
}

type EventType string

const (
	EventQueued          EventType = "queued"
	EventStarted         EventType = "started"
	EventProgress        EventType = "progress"
	EventLog             EventType = "log"
	EventCompleted       EventType = "completed"
	EventFailed          EventType = "failed"
	EventCancelRequested EventType = "cancel_requested"
	EventCancelled       EventType = "cancelled"
	// EventRecovery is appended when the platform reconciles a task's fate it
	// can no longer determine (e.g. the agent restarted mid-task): the row is
	// moved to `unknown` and this event records the recovery for manual review.
	EventRecovery EventType = "recovery"
)

type StartRequest struct {
	FunctionID     string
	InputPayload   []byte
	GameID         string
	Env            string
	TraceID        string
	IdempotencyKey string
}

type TaskContext interface {
	Context() context.Context
	TaskID() string
	ReportProgress(progress int32, message string, payload []byte) error
	Log(message string, payload []byte) error
	IsCancelled() bool
}
