package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

type Store struct {
	runs   *model.TaskRunModel
	events *model.TaskEventModel
	scopes ScopeResolver
}

func NewStore(runs *model.TaskRunModel, events *model.TaskEventModel) *Store {
	return &Store{runs: runs, events: events}
}

// TaskScope names one (game, env) scope whose per-game database holds that
// scope's task_runs rows (model.GameModels — the meta database has no
// task_runs table in multi-game mode).
type TaskScope struct {
	GameID string
	Env    string
}

// ScopeResolver fans reap sweeps out across per-game databases in multi-game
// mode. It mirrors the router's scope resolution (Router.Resolve injects the
// dbctx override) without importing internal/db/router.
type ScopeResolver interface {
	// OpenScopes lists scopes whose physical database is currently open in
	// this process. A scope never opened cannot hold task_runs rows written
	// by this process, so sweeps skip it — enumerating dormant registered
	// games would needlessly trigger per-game first-open side effects.
	OpenScopes() []TaskScope
	// ScopeContext returns ctx carrying the scope's per-game DB override so
	// game-scoped model calls route to that database.
	ScopeContext(ctx context.Context, gameID, env string) (context.Context, error)
}

// RegisteredScopeSource optionally enumerates ALL registered (gameID, env)
// scopes — including dormant ones whose physical database is not currently
// open (never accessed by a user this process). The watchdog ReapStale uses
// it to converge stale tasks in dormant game databases (AUD-015); the
// event-triggered ReapAgentNonTerminal keeps the open-scope-only fan-out so
// a disconnect stays cheap. A resolver that does not implement this
// interface keeps the pre-AUD-015 behavior (open scopes only).
type RegisteredScopeSource interface {
	// RegisteredScopes lists every registered game scope from the meta
	// database's game_envs binding (authoritative routing truth, ARCH-006).
	RegisteredScopes() ([]TaskScope, error)
}

// dormantScopeBudget bounds how many registered-but-not-open scopes one
// watchdog sweep opens per call, so first-open (connect + migrate) side
// effects for dormant games are rate-limited rather than burst on cold
// start. Each opened scope joins the router cache and is swept as an open
// scope on subsequent sweeps — the dormant set shrinks monotonically.
const dormantScopeBudget = 8

// SetScopeResolver wires the multi-game reap fan-out. Nil (default) keeps
// the single-database behavior: one sweep on the store's own DB, where the
// ScopeSingle migration baseline creates task_runs.
func (s *Store) SetScopeResolver(r ScopeResolver) {
	s.scopes = r
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

// RecordTaskEvent 在单一事务内完成带 guard 的状态更新与事件追加（R30-03 /
// ARCH-015）：task_runs 与 task_events 持久一致，事件写失败整体回滚，不产生
// 「状态已终态但事件缺失」的部分写入。updated=false 表示终态 guard 拦截，
// 事件未追加（调用方按已收敛处理）。
func (s *Store) RecordTaskEvent(ctx context.Context, taskID string, blockedStatuses []string, updates map[string]interface{}, eventType EventType, progress int32, message string, payload []byte) (bool, error) {
	return s.runs.RecordTaskEventTransactional(ctx, taskID, blockedStatuses, updates, &model.TaskEvent{
		Type:     string(eventType),
		Progress: progress,
		Message:  message,
		Payload:  payload,
	})
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
	return s.reapAcrossScopes(ctx, func(ctx context.Context) (int64, error) {
		return s.runs.ReapByAgent(ctx, agentID, ReapableStatuses(), status, message)
	})
}

// ReapStale 收割超过预算仍无事件的非终态任务到目标状态（ARCH-018 D3
// 看门狗：updated_at 早于 now-budget 的活动行 → timed_out）。Rows already
// terminal, unknown, or fresh are left untouched. In multi-game mode it
// sweeps every OPEN scope (existing fan-out) plus a bounded set of dormant
// registered scopes (AUD-015) — a game database never opened by a user still
// converges its stale tasks, with first-open side effects rate-limited per
// call by dormantScopeBudget.
func (s *Store) ReapStale(ctx context.Context, budget time.Duration, status, message string) (int64, error) {
	sweep := func(ctx context.Context) (int64, error) {
		return s.runs.ReapStale(ctx, ReapableStatuses(), time.Now().Add(-budget), status, message)
	}
	total, err := s.reapAcrossScopes(ctx, sweep)
	dTotal, dErr := s.reapDormantScopes(ctx, sweep)
	return total + dTotal, errors.Join(err, dErr)
}

// reapDormantScopes sweeps registered-but-not-open game scopes, capped at
// dormantScopeBudget per call. Each visited scope is opened via ScopeContext
// (which, through the real router, adds it to the cache so subsequent sweeps
// treat it as open) and immediately swept; failures are aggregated, never
// silently dropped. A resolver without RegisteredScopeSource (or a nil
// resolver) is a no-op.
func (s *Store) reapDormantScopes(ctx context.Context, sweep func(context.Context) (int64, error)) (int64, error) {
	if s.scopes == nil {
		return 0, nil
	}
	src, ok := s.scopes.(RegisteredScopeSource)
	if !ok {
		return 0, nil
	}
	registered, err := src.RegisteredScopes()
	if err != nil {
		return 0, fmt.Errorf("registered scopes: %w", err)
	}
	openSet := make(map[TaskScope]struct{}, len(s.scopes.OpenScopes()))
	for _, sc := range s.scopes.OpenScopes() {
		openSet[sc] = struct{}{}
	}
	total := int64(0)
	var errs []error
	budget := dormantScopeBudget
	for _, sc := range registered {
		if budget <= 0 {
			break
		}
		if _, isOpen := openSet[sc]; isOpen {
			continue
		}
		scopedCtx, err := s.scopes.ScopeContext(ctx, sc.GameID, sc.Env)
		if err != nil {
			errs = append(errs, fmt.Errorf("reap dormant scope %s/%s: %w", sc.GameID, sc.Env, err))
			continue
		}
		n, err := sweep(scopedCtx)
		if err != nil {
			errs = append(errs, fmt.Errorf("reap dormant scope %s/%s: %w", sc.GameID, sc.Env, err))
			continue
		}
		total += n
		budget--
	}
	return total, errors.Join(errs...)
}

// reapAcrossScopes runs one reap sweep per open game database and sums the
// updated-row counts. Multi-game: task_runs lives in per-game databases
// (model.GameModels) while the store's own DB is the meta database without
// that table — sweeping it would fail with "no such table". Single-database
// (nil resolver): one sweep on the store's own DB, unchanged. A failing
// scope neither aborts the other scopes' sweeps nor is silently dropped:
// failures are aggregated into the returned error (best-effort reap keeps
// the disconnect path non-blocking, per root.go's reaper contract).
func (s *Store) reapAcrossScopes(ctx context.Context, sweep func(context.Context) (int64, error)) (int64, error) {
	if s.scopes == nil {
		return sweep(ctx)
	}
	total := int64(0)
	var errs []error
	for _, sc := range s.scopes.OpenScopes() {
		scopedCtx, err := s.scopes.ScopeContext(ctx, sc.GameID, sc.Env)
		if err != nil {
			errs = append(errs, fmt.Errorf("reap scope %s/%s: %w", sc.GameID, sc.Env, err))
			continue
		}
		n, err := sweep(scopedCtx)
		if err != nil {
			errs = append(errs, fmt.Errorf("reap scope %s/%s: %w", sc.GameID, sc.Env, err))
			continue
		}
		total += n
	}
	return total, errors.Join(errs...)
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
