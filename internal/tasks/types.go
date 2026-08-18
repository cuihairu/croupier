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
)

// IsTerminalStatus reports whether a task status cannot transition again.
func IsTerminalStatus(status string) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut:
		return true
	default:
		return false
	}
}

// TerminalStatuses returns the statuses protected from late event updates.
// The returned slice is deliberately new so callers cannot mutate shared
// state.
func TerminalStatuses() []string {
	return []string{StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut}
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
