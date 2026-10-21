package process

import "time"

type SupervisorAction int

const (
	SupervisorStop SupervisorAction = iota
	SupervisorBreak
	SupervisorBackoff
)

type SupervisorExitDecision struct {
	Action           SupervisorAction
	ConsecutiveFails int
	Delay            time.Duration
}

// SupervisorState owns the retry-policy decisions for one managed process.
// Callers retain process ownership, spawning, stopping, authorization, and
// event emission.
type SupervisorState struct {
	config           RestartConfig
	consecutiveFails int
	nextRestartAt    time.Time
}

func NewSupervisorState(config RestartConfig) *SupervisorState {
	return &SupervisorState{config: config}
}

func (s *SupervisorState) Config() RestartConfig {
	return s.config
}

func (s *SupervisorState) ConsecutiveFails() int {
	return s.consecutiveFails
}

func (s *SupervisorState) NextRestartAt() time.Time {
	return s.nextRestartAt
}

// RecordStart clears retry state after a process instance is successfully
// spawned, including manual recovery from a breaker.
func (s *SupervisorState) RecordStart() {
	s.consecutiveFails = 0
	s.nextRestartAt = time.Time{}
}

func (s *SupervisorState) CancelBackoff() {
	s.nextRestartAt = time.Time{}
}

func (s *SupervisorState) DecideExit(allowRestart bool, now time.Time) SupervisorExitDecision {
	if !allowRestart {
		s.nextRestartAt = time.Time{}
		return SupervisorExitDecision{Action: SupervisorStop}
	}

	s.consecutiveFails++
	if BreakerTripped(s.config, int32(s.consecutiveFails)) {
		s.nextRestartAt = time.Time{}
		return SupervisorExitDecision{
			Action:           SupervisorBreak,
			ConsecutiveFails: s.consecutiveFails,
		}
	}

	delay := RestartDelay(s.config, s.consecutiveFails)
	s.nextRestartAt = now.Add(delay)
	return SupervisorExitDecision{
		Action:           SupervisorBackoff,
		ConsecutiveFails: s.consecutiveFails,
		Delay:            delay,
	}
}

// ConsumeBackoff reports whether the pending restart is due. A restart
// attempt must be started only after a true result.
func (s *SupervisorState) ConsumeBackoff(now time.Time) bool {
	if s.nextRestartAt.IsZero() || now.Before(s.nextRestartAt) {
		return false
	}
	s.nextRestartAt = time.Time{}
	return true
}

// MarkRestartAttempted clears the pending restart after spawning the next
// instance. Health is not inferred from spawn success; the caller moves the
// counter only after its stability window.
func (s *SupervisorState) MarkRestartAttempted() {
	s.nextRestartAt = time.Time{}
}

// RecordSuccess clears the retry counter once a restarted instance has passed
// the caller-defined stability window.
func (s *SupervisorState) RecordSuccess() {
	s.consecutiveFails = 0
}

// SuccessWindow returns the stability interval after which a healthy restart
// would have cleared the retry counter under the original supervisor loop.
func (s *SupervisorState) SuccessWindow() time.Duration {
	return RestartDelay(s.config, 2)
}
