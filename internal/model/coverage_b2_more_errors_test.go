package model

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestB2RecentDefaultLimit(t *testing.T) {
	db := setupAllModelsDB(t)
	require.NoError(t, db.Exec(`INSERT INTO messages (recipient, type, status, created_at) VALUES ('b2to', 'info', 1, CURRENT_TIMESTAMP)`).Error)
	_, err := NewMessageModel(db).Recent(context.Background(), 0, "")
	assert.NoError(t, err)
}

func TestB2CreateWithMetaError(t *testing.T) {
	m := NewConfigVersionModel(newClosedDB(t))
	_, err := m.CreateWithMeta(context.Background(), ConfigVersionPayload{Key: "b2key"}, "admin")
	assert.Error(t, err)

	db := setupAllModelsDB(t)
	b2Trigger(t, db, `CREATE TRIGGER b2_stop_cv BEFORE INSERT ON config_versions BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	_, err = NewConfigVersionModel(db).CreateWithMeta(context.Background(), ConfigVersionPayload{Key: "b2key2"}, "admin")
	assert.Error(t, err)
}

func TestB2PlayerUpdateBalanceMissing(t *testing.T) {
	db := setupAllModelsDB(t)
	_, err := NewPlayerModel(db).UpdateBalance(context.Background(), 987654, 10, "test")
	assert.Error(t, err)
}

func TestB2ReplaceDeleteTriggers(t *testing.T) {
	ctx := context.Background()
	db := setupAllModelsDB(t)
	require.NoError(t, db.Exec(`INSERT INTO function_permissions (function_id, resource, created_at, updated_at) VALUES ('b2fn', 'res', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
	b2Trigger(t, db, `CREATE TRIGGER b2_stop_fpd BEFORE UPDATE OF deleted_at ON function_permissions BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	assert.Error(t, NewFunctionModel(db).ReplacePermissions(ctx, "b2fn", nil))

	require.NoError(t, db.Exec(`INSERT INTO profile_permissions (admin_id, resource, created_at, updated_at) VALUES (1, 'r', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
	b2Trigger(t, db, `CREATE TRIGGER b2_stop_ppd BEFORE UPDATE OF deleted_at ON profile_permissions BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	assert.Error(t, NewProfileModel(db).ReplacePermissions(ctx, 1, nil))

	require.NoError(t, db.Exec(`INSERT INTO profile_games (admin_id, game_id, created_at, updated_at) VALUES (1, 'b2g', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
	b2Trigger(t, db, `CREATE TRIGGER b2_stop_pgd BEFORE UPDATE OF deleted_at ON profile_games BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	assert.Error(t, NewProfileModel(db).ReplaceGames(ctx, 1, nil))
}

func TestB2BackfillEnvBindingsMoreErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("find games scan error", func(t *testing.T) {
		db := setupAllModelsDB(t)
		require.NoError(t, db.Exec(`INSERT INTO games (game_id, name, created_at, updated_at) VALUES ('b2scan', 'x', 'not-a-time', CURRENT_TIMESTAMP)`).Error)
		_, err := NewGameModel(db).BackfillEnvBindings(ctx, func(_, _ string) string { return "db" })
		assert.Error(t, err)
	})

	t.Run("create binding blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameModel(db)
		g := &Game{GameID: "b2g3", Name: "G", Envs: JSON(`[{"env":"prod"}]`)}
		require.NoError(t, m.Create(ctx, g))
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_geb_ins2 BEFORE INSERT ON game_envs BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		_, err := m.BackfillEnvBindings(ctx, func(_, _ string) string { return "db" })
		assert.Error(t, err)
	})

	t.Run("existing binding scan error", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameModel(db)
		g := &Game{GameID: "b2g4", Name: "G", Envs: JSON(`[{"env":"prod"}]`)}
		require.NoError(t, m.Create(ctx, g))
		require.NoError(t, db.Exec(`INSERT INTO game_envs (game_id, env, database_name, created_at, updated_at) VALUES ('b2g4', 'prod', 'db', 'not-a-time', CURRENT_TIMESTAMP)`).Error)
		_, err := m.BackfillEnvBindings(ctx, func(_, _ string) string { return "db" })
		assert.Error(t, err)
	})
}

func TestB2AnalyticsDayParseFallback(t *testing.T) {
	ctx := context.Background()
	db := setupAllModelsDB(t)

	mid := time.Now().UTC().Format("2006-01-02") + "T99"
	require.NoError(t, db.Exec(`INSERT INTO behavior_events (game_id, env, event_type, user_id, occurred_at) VALUES ('b2g', 'prod', 'click', 'u1', ?)`, mid).Error)
	stats, err := NewBehaviorModel(db).DailyActivity(ctx, "b2g", "prod", time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	require.NotEmpty(t, stats)
	for _, s := range stats {
		assert.True(t, s.Day.IsZero())
	}

	mid2 := time.Now().UTC().Format("2006-01-02") + "T99"
	require.NoError(t, db.Exec(`INSERT INTO payment_transactions (transaction_id, game_id, env, user_id, status, occurred_at) VALUES ('b2txn', 'b2g', 'prod', 'u1', 'success', ?)`, mid2).Error)
	rev, err := NewPaymentsModel(db).DailyRevenue(ctx, "b2g", "prod", time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	require.NotEmpty(t, rev)
	for _, r := range rev {
		assert.True(t, r.Day.IsZero())
	}

	mid3 := time.Now().UTC().Format("2006-01-02") + "T99"
	require.NoError(t, db.Exec(`INSERT INTO players (username, game_id, created_at) VALUES ('b2p2', 'b2g', ?)`, mid3).Error)
	players, err := NewPlayerModel(db).DailyNewPlayers(ctx, "b2g", time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	require.NotEmpty(t, players)
	for _, p := range players {
		assert.True(t, p.Day.IsZero())
	}
}

var feedbackColumnIndexes = map[string][]string{
	"category": {"idx_feedbacks_category"},
	"status":   {"idx_feedbacks_status"},
	"rating":   {"idx_feedbacks_rating"},
	"reply":    {},
}

func TestB2FeedbackStatsErrors(t *testing.T) {
	ctx := context.Background()

	_, err := NewFeedbackModel(newClosedDB(t)).Stats(ctx, FeedbackStatsOptions{})
	assert.Error(t, err)

	for _, col := range []string{"category", "status", "rating", "reply"} {
		col := col
		t.Run("drop "+col, func(t *testing.T) {
			db := setupAllModelsDB(t)
			for _, idx := range feedbackColumnIndexes[col] {
				require.NoError(t, db.Exec("DROP INDEX IF EXISTS "+idx).Error)
			}
			require.NoError(t, db.Exec("ALTER TABLE feedbacks DROP COLUMN "+col).Error)
			_, err := NewFeedbackModel(db).Stats(ctx, FeedbackStatsOptions{})
			assert.Error(t, err)
		})
	}
}

func TestB2TermDictionaryAliasMapBranches(t *testing.T) {
	ctx := context.Background()
	db := setupAllModelsDB(t)
	require.NoError(t, db.Exec(`INSERT INTO term_dictionary (domain, term_key, alias, created_at, updated_at) VALUES ('', 'k', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO term_dictionary (domain, term_key, alias, created_at, updated_at) VALUES ('resource', 'k2', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)

	m, err := NewTermDictionaryModel(db).AliasMap(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, m)
}
