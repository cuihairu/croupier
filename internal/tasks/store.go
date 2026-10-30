package tasks

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

type Store struct {
	runs   *model.TaskRunModel
	events *model.TaskEventModel
}

func NewStore(runs *model.TaskRunModel, events *model.TaskEventModel) *Store {
	return &Store{runs: runs, events: events}
}

func (s *Store) CreateRun(ctx context.Context, run *model.TaskRun) error {
	return s.runs.Create(ctx, run)
}

func (s *Store) GetRun(ctx context.Context, taskID string) (*model.TaskRun, error) {
	return s.runs.FindByTaskID(ctx, taskID)
}

func (s *Store) UpdateRun(ctx context.Context, taskID string, updates map[string]interface{}) error {
	return s.runs.UpdateByTaskID(ctx, taskID, updates)
}

// UpdateRunIfStatusNotIn applies a guarded task update for lifecycle
// transitions with stricter preconditions than terminal protection alone.
func (s *Store) UpdateRunIfStatusNotIn(ctx context.Context, taskID string, blockedStatuses []string, updates map[string]interface{}) (bool, error) {
	return s.runs.UpdateByTaskIDIfStatusNotIn(ctx, taskID, blockedStatuses, updates)
}

func (s *Store) ListRuns(ctx context.Context, opts model.ListTasksOptions) ([]model.TaskRun, int64, error) {
	return s.runs.List(ctx, opts)
}

func (s *Store) AppendEvent(ctx context.Context, taskID string, eventType EventType, progress int32, message string, payload []byte) error {
	seq, err := s.events.NextSeq(ctx, taskID)
	if err != nil {
		return err
	}
	if len(payload) == 0 {
		payload = []byte("null")
	}
	return s.events.Append(ctx, &model.TaskEvent{
		TaskID:    strings.TrimSpace(taskID),
		Seq:       seq,
		Type:      string(eventType),
		Progress:  progress,
		Message:   message,
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}

// ReapAgentNonTerminal transitions the given agent's still-active task_runs
// rows to the target status (ARCH-014 D3: agent session loss → unknown).
// Active means non-terminal and not already unknown (ReapableStatuses);
// terminal and unknown rows are left untouched. Returns the number of rows
// updated.
func (s *Store) ReapAgentNonTerminal(ctx context.Context, agentID, status, message string) (int64, error) {
	return s.runs.ReapByAgent(ctx, agentID, ReapableStatuses(), status, message)
}

// ReapStale 收割超过预算仍无事件的非终态任务到目标状态（ARCH-018 D3
// 看门狗：updated_at 早于 now-budget 的活动行 → timed_out）。Rows already
// terminal, unknown, or fresh are left untouched.
func (s *Store) ReapStale(ctx context.Context, budget time.Duration, status, message string) (int64, error) {
	return s.runs.ReapStale(ctx, ReapableStatuses(), time.Now().Add(-budget), status, message)
}

func (s *Store) ListEvents(ctx context.Context, taskID string, afterSeq int64) ([]model.TaskEvent, error) {
	return s.events.ListByTaskID(ctx, taskID, afterSeq)
}

func JSONPayload(v interface{}) []byte {
	if v == nil {
		return []byte("null")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return b
}
