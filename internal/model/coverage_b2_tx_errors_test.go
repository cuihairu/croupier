package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func b2Trigger(t *testing.T, db *gorm.DB, ddl string) {
	t.Helper()
	require.NoError(t, db.Exec(ddl).Error)
}

func TestB2AdminModel_Branches(t *testing.T) {
	ctx := context.Background()
	db := setupAllModelsDB(t)
	m := NewAdminModel(db)

	admin := &Admin{Username: "b2admin", Nickname: "n"}
	hashed, err := bcryptHash("secret-123")
	require.NoError(t, err)
	require.NoError(t, m.Create(ctx, admin, string(hashed)))
	assert.NotEmpty(t, admin.PasswordHash)

	// RecordLoginFailure：不存在 ID → First 报 ErrRecordNotFound。
	_, _, rerr := m.RecordLoginFailure(ctx, 424242, 3, 0)
	assert.Error(t, rerr)

	// ValidatePassword：校验通过但 last_login 更新被触发器阻断（best-effort）。
	b2Trigger(t, db, `CREATE TRIGGER b2_stop_last_login BEFORE UPDATE OF last_login_at ON admins BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	got, err := m.ValidatePassword(ctx, "b2admin", "secret-123")
	require.NoError(t, err)
	assert.NotNil(t, got)

	// RecordLoginFailure：达到阈值后 locked_until 更新失败。
	b2Trigger(t, db, `CREATE TRIGGER b2_stop_locked BEFORE UPDATE OF locked_until ON admins BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	_, _, err = m.RecordLoginFailure(ctx, admin.ID, 1, 0)
	assert.Error(t, err)
}

func TestB2ProfileModel_ReplaceErrors(t *testing.T) {
	ctx := context.Background()

	closed := NewProfileModel(newClosedDB(t))
	assert.Error(t, closed.ReplacePermissions(ctx, 1, nil))
	assert.Error(t, closed.ReplaceGames(ctx, 1, nil))

	db := setupAllModelsDB(t)
	m := NewProfileModel(db)
	b2Trigger(t, db, `CREATE TRIGGER b2_stop_pp BEFORE INSERT ON profile_permissions BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	b2Trigger(t, db, `CREATE TRIGGER b2_stop_pg BEFORE INSERT ON profile_games BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	assert.Error(t, m.ReplacePermissions(ctx, 1, []ProfilePermission{{AdminID: 1}}))
	assert.Error(t, m.ReplaceGames(ctx, 1, []ProfileGame{{AdminID: 1}}))
}

func TestB2FunctionModel_ReplaceAndCopyErrors(t *testing.T) {
	ctx := context.Background()

	closed := NewFunctionModel(newClosedDB(t))
	assert.Error(t, closed.ReplacePermissions(ctx, "f", nil))

	db := setupAllModelsDB(t)
	m := NewFunctionModel(db)
	require.NoError(t, db.Exec(`INSERT INTO functions (function_id, game_id, name, status, created_at, updated_at) VALUES ('b2src', 'b2g', 'n', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)

	b2Trigger(t, db, `CREATE TRIGGER b2_stop_fp BEFORE INSERT ON function_permissions BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	assert.Error(t, m.ReplacePermissions(ctx, "b2src", []FunctionPermission{{FunctionID: "b2src"}}))

	b2Trigger(t, db, `CREATE TRIGGER b2_stop_fn BEFORE INSERT ON functions BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	_, err := m.CopyFunction(ctx, "b2src")
	assert.Error(t, err)
}

func TestB2PlayerModel_UpdateBalanceError(t *testing.T) {
	m := NewPlayerModel(newClosedDB(t))
	_, err := m.UpdateBalance(context.Background(), 1, 10, "test")
	assert.Error(t, err)
}

func TestB2GameModel_TxErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("update envs blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameModel(db)
		g := &Game{GameID: "b2g", Name: "G"}
		require.NoError(t, m.Create(ctx, g))
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_envs BEFORE UPDATE OF envs ON games BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		err := m.UpdateEnvsAndBindings(ctx, "b2g", g.ID, JSON(`[]`), nil, nil)
		assert.Error(t, err)
	})

	t.Run("delete binding blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameModel(db)
		require.NoError(t, db.Exec(`INSERT INTO game_envs (game_id, env, database_name, created_at, updated_at) VALUES ('b2g', 'prod', 'db', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_geb_del BEFORE DELETE ON game_envs BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		err := m.UpdateEnvsAndBindings(ctx, "b2g", 1, JSON(`[]`), []string{"prod"}, nil)
		_ = err
		assert.Error(t, err)
	})

	t.Run("upsert binding blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameModel(db)
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_geb_ins BEFORE INSERT ON game_envs BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		err := m.UpdateEnvsAndBindings(ctx, "b2g", 1, JSON(`[]`), nil, []GameEnvBinding{{Env: "prod", DatabaseName: "db"}})
		assert.Error(t, err)
	})

	t.Run("delete game blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameModel(db)
		g := &Game{GameID: "b2g", Name: "G"}
		require.NoError(t, m.Create(ctx, g))
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_game_del BEFORE UPDATE OF deleted_at ON games BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		assert.Error(t, m.DeleteWithEnvBindings(ctx, g.ID, "b2g"))
	})
}

func TestB2GameModel_BackfillEnvBindings(t *testing.T) {
	ctx := context.Background()

	closed := NewGameModel(newClosedDB(t))
	_, err := closed.BackfillEnvBindings(ctx, func(_, _ string) string { return "db" })
	assert.Error(t, err)

	t.Run("restore soft deleted binding", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameModel(db)
		g := &Game{GameID: "b2g", Name: "G", Envs: JSON(`[{"env":"prod"}]`)}
		require.NoError(t, m.Create(ctx, g))
		binding := GameEnvBinding{GameID: "b2g", Env: "prod", DatabaseName: "b2g_prod"}
		require.NoError(t, db.Create(&binding).Error)
		require.NoError(t, db.Delete(&binding).Error)

		created, err := m.BackfillEnvBindings(ctx, func(gameID, env string) string { return gameID + "_" + env })
		require.NoError(t, err)
		assert.Equal(t, 1, created)
	})

	t.Run("restore update blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameModel(db)
		g := &Game{GameID: "b2g", Name: "G", Envs: JSON(`[{"env":"prod"}]`)}
		require.NoError(t, m.Create(ctx, g))
		binding := GameEnvBinding{GameID: "b2g", Env: "prod", DatabaseName: "b2g_prod"}
		require.NoError(t, db.Create(&binding).Error)
		require.NoError(t, db.Delete(&binding).Error)
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_geb_upd BEFORE UPDATE ON game_envs BEGIN SELECT RAISE(ABORT, 'blocked'); END`)

		_, err := m.BackfillEnvBindings(ctx, func(_, _ string) string { return "db" })
		assert.Error(t, err)
	})

	t.Run("empty game id skipped", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameModel(db)
		require.NoError(t, db.Exec(`INSERT INTO games (game_id, name, created_at, updated_at) VALUES ('', 'anon', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
		created, err := m.BackfillEnvBindings(ctx, func(_, _ string) string { return "db" })
		require.NoError(t, err)
		assert.Equal(t, 0, created)
	})
}

func TestB2GameReleaseModel_TransitionErrors(t *testing.T) {
	ctx := context.Background()

	newRel := func(t *testing.T, db *gorm.DB, status string) uint {
		t.Helper()
		rel := GameRelease{
			GameID: "b2g", Env: "prod", Channel: "stable", Platform: "ios",
			Version: "1.0.0", Status: status, ObjectKey: "obj",
		}
		require.NoError(t, db.Create(&rel).Error)
		return rel.ID
	}

	t.Run("archive full release blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameReleaseModel(db)
		newRel(t, db, ReleaseStatusFull)
		id := newRel(t, db, ReleaseStatusGray)
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_rel BEFORE UPDATE ON game_releases BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		_, err := m.Transition(ctx, id, ReleaseStatusFull, nil)
		assert.Error(t, err)
	})

	t.Run("final updates blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewGameReleaseModel(db)
		id := newRel(t, db, ReleaseStatusGray)
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_rel2 BEFORE UPDATE ON game_releases BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		_, err := m.Transition(ctx, id, ReleaseStatusFull, nil)
		assert.Error(t, err)
	})
}

func TestB2HotpatchModel_TransitionError(t *testing.T) {
	ctx := context.Background()
	db := setupAllModelsDB(t)
	m := NewHotpatchModel(db)
	hp := Hotpatch{
		GameID: "b2g", Env: "prod", BugID: 7,
		Status: HotpatchStatusDraft, PackageKey: "pkg", RolloutSeed: "s",
	}
	require.NoError(t, db.Create(&hp).Error)
	b2Trigger(t, db, `CREATE TRIGGER b2_stop_hp BEFORE UPDATE ON hotpatches BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	_, err := m.Transition(ctx, hp.ID, HotpatchStatusApproved, nil)
	assert.Error(t, err)
}

func TestB2ConfigVersionModel_CreateError(t *testing.T) {
	m := NewConfigVersionModel(newClosedDB(t))
	_, err := m.CreateWithMeta(context.Background(), ConfigVersionPayload{}, "admin")
	assert.Error(t, err)
}

func TestB2BugModel_Errors(t *testing.T) {
	ctx := context.Background()
	m := NewBugModel(newClosedDB(t))
	_, err := m.FindOne(ctx, 1)
	assert.Error(t, err)
	assert.Error(t, m.Delete(ctx, 1))
}

func TestB2ListFilterBranches(t *testing.T) {
	ctx := context.Background()
	db := setupAllModelsDB(t)
	require.NoError(t, db.Exec(`INSERT INTO tickets (title, status, created_at, updated_at) VALUES ('b2t', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
	_, _, err := NewTicketModel(db).List(ctx, TicketQueryOptions{Status: -1, PlayerID: "player-1", Env: "prod", Query: "b2"})
	require.NoError(t, err)

	_, _, err = NewGameReleaseModel(db).List(ctx, ReleaseQueryOptions{Type: "hotfix", Status: ReleaseStatusFull})
	require.NoError(t, err)
}

func bcryptHash(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
}
