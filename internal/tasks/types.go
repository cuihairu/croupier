package tasks

import "context"

const (
	StatusQueued          = "queued"
	StatusRunning         = "running"
	StatusSucceeded       = "succeeded"
	StatusFailed          = "failed"
	StatusCancelRequested = "cancel_requested"
	StatusCancelled       = "cancelled"
	StatusTimedOut        = "timed_out"
)

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
