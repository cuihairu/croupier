package analytics

// DATA-058: level completion rates must be computed over stable attempt
// identities, dedup repeated reports, and disclose completion-only windows
// (cross-window/async attribution) instead of the fabricated
// completions / max(attempts, 1) rate that produced 200% windows.

import (
	"testing"
	"time"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runLevels058(t *testing.T) func(events []model.BehaviorEvent) []LevelMetrics {
	t.Helper()
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	return func(events []model.BehaviorEvent) []LevelMetrics {
		for i := range events {
			require.NoError(t, svcCtx.BehaviorModel.RecordEvent(t.Context(), &events[i]))
		}
		resp, err := levels(t.Context(), svcCtx, &LevelsRequest{GameId: "g", Env: "prod"})
		require.NoError(t, err)
		return resp.Levels
	}
}

func TestLevels_CompletionRateExplicitDegradation(t *testing.T) {
	base := time.Now().UTC().Add(-time.Hour)

	t.Run("duplicate_reports_counted_once", func(t *testing.T) {
		run := runLevels058(t)
		dup := model.BehaviorEvent{GameID: "g", Env: "prod", EventType: "level_attempt", UserID: "u1",
			Data: datatypes.JSONMap{"levelId": "L1"}, OccurredAt: base}
		distinct := model.BehaviorEvent{GameID: "g", Env: "prod", EventType: "level_attempt", UserID: "u2",
			Data: datatypes.JSONMap{"levelId": "L1"}, OccurredAt: base.Add(time.Minute)}
		levels := run([]model.BehaviorEvent{dup, dup, dup, distinct})
		require.Len(t, levels, 1)
		lv := levels[0]
		assert.Equal(t, 2, lv.Attempts, "identical re-uploads count once, distinct attempts still count")
	})

	t.Run("identity_pairs_and_discloses_unmatched", func(t *testing.T) {
		run := runLevels058(t)
		mk := func(eventType, userID, attemptID string, at time.Time) model.BehaviorEvent {
			return model.BehaviorEvent{GameID: "g", Env: "prod", EventType: eventType, UserID: userID,
				Data: datatypes.JSONMap{"levelId": "L1", "attemptId": attemptID}, OccurredAt: at}
		}
		levels := run([]model.BehaviorEvent{
			mk("level_attempt", "u1", "a1", base),
			mk("level_attempt", "u1", "a2", base.Add(time.Minute)),
			mk("level_complete", "u1", "a1", base.Add(2*time.Minute)),
			// a9 started the previous day (outside the window scan) / arrives async.
			mk("level_complete", "u1", "a9", base.Add(3*time.Minute)),
		})
		require.Len(t, levels, 1)
		lv := levels[0]
		assert.Equal(t, 2, lv.Attempts)
		assert.Equal(t, 2, lv.Completions)
		require.NotNil(t, lv.CompletionRate)
		assert.InDelta(t, 0.5, *lv.CompletionRate, 0.001)
		assert.Equal(t, "ok", lv.CompletionRateStatus)
		assert.Equal(t, 1, lv.UnmatchedCompletions)
	})

	t.Run("completion_only_attributes_cross_window", func(t *testing.T) {
		run := runLevels058(t)
		// Zero attempts in the window; the completion belongs to a start
		// outside it (previous-day start or async arrival).
		levels := run([]model.BehaviorEvent{{
			GameID: "g", Env: "prod", EventType: "level_complete", UserID: "u1",
			Data: datatypes.JSONMap{"levelId": "L1"}, OccurredAt: base,
		}})
		require.Len(t, levels, 1)
		lv := levels[0]
		assert.Equal(t, 0, lv.Attempts)
		assert.Equal(t, 1, lv.Completions)
		require.Nil(t, lv.CompletionRate)
		assert.Equal(t, "cross_window", lv.CompletionRateStatus)
	})

	t.Run("identity_completion_only_attributes_cross_window", func(t *testing.T) {
		run := runLevels058(t)
		levels := run([]model.BehaviorEvent{{
			GameID: "g", Env: "prod", EventType: "level_complete", UserID: "u1",
			Data: datatypes.JSONMap{"levelId": "L1", "attemptId": "a1"}, OccurredAt: base,
		}})
		require.Len(t, levels, 1)
		lv := levels[0]
		require.Nil(t, lv.CompletionRate)
		assert.Equal(t, "cross_window", lv.CompletionRateStatus)
		assert.Equal(t, 1, lv.UnmatchedCompletions)
	})

	t.Run("count_mode_completions_exceed_attempts", func(t *testing.T) {
		run := runLevels058(t)
		levels := run([]model.BehaviorEvent{
			{GameID: "g", Env: "prod", EventType: "level_attempt", UserID: "u1",
				Data: datatypes.JSONMap{"levelId": "L1"}, OccurredAt: base},
			{GameID: "g", Env: "prod", EventType: "level_complete", UserID: "u2",
				Data: datatypes.JSONMap{"levelId": "L1"}, OccurredAt: base.Add(time.Minute)},
			{GameID: "g", Env: "prod", EventType: "level_complete", UserID: "u3",
				Data: datatypes.JSONMap{"levelId": "L1"}, OccurredAt: base.Add(2 * time.Minute)},
		})
		require.Len(t, levels, 1)
		lv := levels[0]
		assert.Equal(t, 1, lv.Attempts)
		assert.Equal(t, 2, lv.Completions)
		require.Nil(t, lv.CompletionRate)
		assert.Equal(t, "cross_window", lv.CompletionRateStatus)
		assert.Equal(t, 1, lv.UnmatchedCompletions)
	})

	t.Run("count_mode_ok_and_retry_attempts_stay_distinct", func(t *testing.T) {
		run := runLevels058(t)
		levels := run([]model.BehaviorEvent{
			{GameID: "g", Env: "prod", EventType: "level_attempt", UserID: "u1",
				Data: datatypes.JSONMap{"levelId": "L1"}, OccurredAt: base},
			{GameID: "g", Env: "prod", EventType: "level_attempt", UserID: "u2",
				Data: datatypes.JSONMap{"levelId": "L1"}, OccurredAt: base.Add(time.Minute)},
			{GameID: "g", Env: "prod", EventType: "level_complete", UserID: "u1",
				Data: datatypes.JSONMap{"levelId": "L1"}, OccurredAt: base.Add(2 * time.Minute)},
		})
		require.Len(t, levels, 1)
		lv := levels[0]
		assert.Equal(t, 2, lv.Attempts)
		assert.Equal(t, 1, lv.Completions)
		require.NotNil(t, lv.CompletionRate)
		assert.InDelta(t, 0.5, *lv.CompletionRate, 0.001)
		assert.Equal(t, "ok", lv.CompletionRateStatus)
	})
}
