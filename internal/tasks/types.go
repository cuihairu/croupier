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
	// StatusUnknown（ARCH-014 D3）：agent 会话丢失后，该 agent 的非终态任务
	// 收敛于此——执行可能已完成/失败/从未开始，游戏侧结果未确立。软终态：
	// 停止派发侧活动跟踪，但同 agent 重连的迟到终态事件仍可收敛到真实终态
	// （未知不在 TerminalStatuses 中，事件守卫不挡它）。
	StatusUnknown = "unknown"
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

// IsReapableStatus reports whether a task is still active and can therefore be
// reaped by the agent-session-loss reaper (ARCH-014 D3). It mirrors
// ReapableStatuses: only the explicit active set is transitioned, so a
// NULL/empty, terminal, or already-unknown status is never reaped.
func IsReapableStatus(status string) bool {
	switch status {
	case StatusQueued, StatusDispatching, StatusRunning, StatusCancelRequested:
		return true
	default:
		return false
	}
}

// ReapableStatuses returns the statuses the agent-session-loss reaper
// transitions (everything active). The returned slice is deliberately new and
// must stay in sync with IsReapableStatus.
func ReapableStatuses() []string {
	return []string{
		StatusQueued,
		StatusDispatching,
		StatusRunning,
		StatusCancelRequested,
	}
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
