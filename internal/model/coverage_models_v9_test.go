package model

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestAdminModelLoginFailureLockingV9(t *testing.T) {
	db := setupTestDB(t)
	m := NewAdminModel(db)
	ctx := context.Background()

	admin := &Admin{Username: "lockadmin", Status: 1}
	require.NoError(t, m.Create(ctx, admin, "password"))

	// Below the threshold: counter grows, no lock.
	attempts, lockedUntil, err := m.RecordLoginFailure(ctx, admin.ID, 3, 10*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, attempts)
	assert.Nil(t, lockedUntil)

	attempts, lockedUntil, err = m.RecordLoginFailure(ctx, admin.ID, 3, 10*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.Nil(t, lockedUntil)

	// Reaching the threshold locks the account.
	attempts, lockedUntil, err = m.RecordLoginFailure(ctx, admin.ID, 3, 10*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
	require.NotNil(t, lockedUntil)
	assert.True(t, lockedUntil.After(time.Now()))

	// An active lock is not extended by further failures.
	future := time.Now().Add(time.Hour).UTC()
	require.NoError(t, db.Exec(`UPDATE admins SET locked_until = ? WHERE id = ?`, future, admin.ID).Error)
	_, lockedUntil, err = m.RecordLoginFailure(ctx, admin.ID, 3, 10*time.Minute)
	require.NoError(t, err)
	assert.Nil(t, lockedUntil)

	// A stale (past) lock is replaced by a fresh one.
	past := time.Now().Add(-time.Hour).UTC()
	require.NoError(t, db.Exec(`UPDATE admins SET locked_until = ? WHERE id = ?`, past, admin.ID).Error)
	_, lockedUntil, err = m.RecordLoginFailure(ctx, admin.ID, 3, 10*time.Minute)
	require.NoError(t, err)
	require.NotNil(t, lockedUntil)
	assert.True(t, lockedUntil.After(time.Now()))

	// Reset clears the counter and the lock.
	require.NoError(t, m.ResetLoginFailures(ctx, admin.ID))
	found, err := m.FindOne(ctx, admin.ID)
	require.NoError(t, err)
	assert.Zero(t, found.FailedAttempts)
	assert.Nil(t, found.LockedUntil)
}

func TestAdminModelTokenVersionAndOTPV9(t *testing.T) {
	db := setupTestDB(t)
	m := NewAdminModel(db)
	ctx := context.Background()

	admin := &Admin{Username: "tokenotpadmin", Status: 1}
	require.NoError(t, m.Create(ctx, admin, "password"))

	v, err := m.GetTokenVersion(ctx, admin.ID)
	require.NoError(t, err)
	assert.Zero(t, v)

	require.NoError(t, m.BumpTokenVersion(ctx, admin.ID))
	v, err = m.GetTokenVersion(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, v)

	_, err = m.GetTokenVersion(ctx, 99999)
	assert.Error(t, err)

	require.NoError(t, m.SetOTPSecret(ctx, admin.ID, "JBSWY3DPEHPK3PXP"))
	require.NoError(t, m.EnableOTP(ctx, admin.ID))
	found, err := m.FindByUsername(ctx, "tokenotpadmin")
	require.NoError(t, err)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", found.OTPSecret)
	assert.True(t, found.OTPEnabled)

	require.NoError(t, m.DisableOTP(ctx, admin.ID))
	found, err = m.FindByUsername(ctx, "tokenotpadmin")
	require.NoError(t, err)
	assert.Empty(t, found.OTPSecret)
	assert.False(t, found.OTPEnabled)
}

func TestPasswordEdgeCasesV9(t *testing.T) {
	db := setupTestDB(t)
	adminModel := NewAdminModel(db)
	playerModel := NewPlayerModel(db)
	ctx := context.Background()

	// bcrypt refuses passwords longer than 72 bytes.
	tooLong := strings.Repeat("x", 100)
	assert.Error(t, adminModel.Create(ctx, &Admin{Username: "longpwd"}, tooLong))
	assert.Error(t, adminModel.UpdatePassword(ctx, 1, tooLong))
	assert.Error(t, playerModel.Create(ctx, &Player{Username: "longpwd"}, tooLong))
	assert.Error(t, playerModel.UpdatePassword(ctx, 1, tooLong))

	// A player without a stored password cannot be validated.
	require.NoError(t, playerModel.Create(ctx, &Player{Username: "nopwd-v9", GameID: "game1", Status: 1}, ""))
	_, err := playerModel.ValidatePassword(ctx, "nopwd-v9", "whatever", "game1")
	assert.ErrorContains(t, err, "no password set")
}

func TestGameReleaseTransitionBranchesV9(t *testing.T) {
	db := setupGameRelDB(t)
	m := NewGameReleaseModel(db)
	ctx := context.Background()

	rel := seedGameRelease(t, db, "draft", "1.0.0")

	// Invalid transition from draft.
	_, err := m.Transition(ctx, rel.ID, ReleaseStatusFull, nil)
	assert.ErrorContains(t, err, "invalid transition")

	// Testing requires an uploaded artifact.
	require.NoError(t, db.Model(rel).Update("status", ReleaseStatusUploading).Error)
	require.NoError(t, db.Model(rel).Update("object_key", "").Error)
	_, err = m.Transition(ctx, rel.ID, ReleaseStatusTesting, nil)
	assert.ErrorContains(t, err, "artifact must be uploaded")

	// Valid testing transition.
	require.NoError(t, db.Model(rel).Update("object_key", "obj").Error)
	_, err = m.Transition(ctx, rel.ID, ReleaseStatusTesting, nil)
	require.NoError(t, err)

	// Gray percent out of range.
	bad := -1
	_, err = m.Transition(ctx, rel.ID, ReleaseStatusGray, &bad)
	assert.ErrorContains(t, err, "gray percent must be within 0-100")
	bad = 101
	_, err = m.Transition(ctx, rel.ID, ReleaseStatusGray, &bad)
	assert.ErrorContains(t, err, "gray percent must be within 0-100")

	// Gray percent cannot decrease.
	pct := 10
	_, err = m.Transition(ctx, rel.ID, ReleaseStatusGray, &pct)
	require.NoError(t, err)
	lower := 5
	_, err = m.Transition(ctx, rel.ID, ReleaseStatusGray, &lower)
	assert.ErrorContains(t, err, "gray percent can only increase")

	// gray -> gray raising the percent, then rollback.
	higher := 50
	got, err := m.Transition(ctx, rel.ID, ReleaseStatusGray, &higher)
	require.NoError(t, err)
	assert.Equal(t, 50, got.GrayPercent)

	rolled, err := m.Transition(ctx, rel.ID, ReleaseStatusRolledBack, nil)
	require.NoError(t, err)
	assert.Equal(t, ReleaseStatusRolledBack, rolled.Status)

	// Unknown source status cannot transition.
	assert.False(t, CanTransition("bogus", ReleaseStatusDraft))

	// Missing release id.
	_, err = m.Transition(ctx, 99999, ReleaseStatusArchived, nil)
	assert.Error(t, err)
}

func TestHotpatchTransitionAndResultsV9(t *testing.T) {
	db := newModelTestDB(t, &Hotpatch{})
	m := NewHotpatchModel(db)
	ctx := context.Background()

	// List filters and the not-found lookup.
	require.NoError(t, m.Create(ctx, &Hotpatch{GameID: "demo", Env: "prod", Framework: "skynet", Status: HotpatchStatusDraft}))
	require.NoError(t, m.Create(ctx, &Hotpatch{GameID: "other", Env: "dev", Framework: "jvm", Status: HotpatchStatusApplied}))
	items, total, err := m.List(ctx, HotpatchQueryOptions{
		PaginationOptions: PaginationOptions{Page: 1, PageSize: 10},
		GameID:            "demo", Env: "prod", Framework: "skynet", Status: HotpatchStatusDraft,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	_, err = m.FindOne(ctx, 99999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	hp := &Hotpatch{GameID: "demo", Env: "prod", Framework: "skynet", Status: HotpatchStatusDraft}
	require.NoError(t, m.Create(ctx, hp))

	// Package is mandatory before approval.
	_, err = m.Transition(ctx, hp.ID, HotpatchStatusApproved, nil)
	assert.ErrorContains(t, err, "package must be uploaded")
	require.NoError(t, m.Update(ctx, hp.ID, map[string]interface{}{"package_key": "pkg"}))

	// Bug reference is mandatory (traceability).
	_, err = m.Transition(ctx, hp.ID, HotpatchStatusApproved, nil)
	assert.ErrorContains(t, err, "must reference a bug")
	require.NoError(t, m.Update(ctx, hp.ID, map[string]interface{}{"bug_id": 7}))
	_, err = m.Transition(ctx, hp.ID, HotpatchStatusApproved, nil)
	require.NoError(t, err)

	// Invalid transition approved -> applied.
	_, err = m.Transition(ctx, hp.ID, HotpatchStatusApplied, nil)
	assert.ErrorContains(t, err, "invalid transition")

	// Rollout percent bounds.
	bad := -1
	_, err = m.Transition(ctx, hp.ID, HotpatchStatusRolling, &bad)
	assert.ErrorContains(t, err, "rollout percent must be within 0-100")
	bad = 101
	_, err = m.Transition(ctx, hp.ID, HotpatchStatusRolling, &bad)
	assert.ErrorContains(t, err, "rollout percent must be within 0-100")

	pct := 20
	got, err := m.Transition(ctx, hp.ID, HotpatchStatusRolling, &pct)
	require.NoError(t, err)
	assert.Equal(t, 20, got.RolloutPercent)

	// Percent cannot decrease.
	lower := 10
	_, err = m.Transition(ctx, hp.ID, HotpatchStatusRolling, &lower)
	assert.ErrorContains(t, err, "rollout percent can only increase")

	// Rolling -> applied -> rolled_back.
	higher := 60
	_, err = m.Transition(ctx, hp.ID, HotpatchStatusRolling, &higher)
	require.NoError(t, err)
	_, err = m.Transition(ctx, hp.ID, HotpatchStatusApplied, nil)
	require.NoError(t, err)
	_, err = m.Transition(ctx, hp.ID, HotpatchStatusRolledBack, nil)
	require.NoError(t, err)

	// Missing hotpatch.
	_, err = m.Transition(ctx, 99999, HotpatchStatusFailed, nil)
	assert.Error(t, err)

	// AppendResult: corrupt existing JSON -> unmarshal error.
	require.NoError(t, db.Exec(`UPDATE hotpatches SET results = 'not-json' WHERE id = ?`, hp.ID).Error)
	assert.ErrorContains(t, m.AppendResult(ctx, hp.ID, HotpatchResult{AgentID: "a1", Status: "ok", At: "t"}), "unmarshal existing results")
	require.NoError(t, db.Exec(`UPDATE hotpatches SET results = NULL WHERE id = ?`, hp.ID).Error)

	// Append then replace (per-agent last wins).
	require.NoError(t, m.AppendResult(ctx, hp.ID, HotpatchResult{AgentID: "a1", Status: "ok", At: "t1"}))
	require.NoError(t, m.AppendResult(ctx, hp.ID, HotpatchResult{AgentID: "a2", Status: "failed", At: "t2"}))
	require.NoError(t, m.AppendResult(ctx, hp.ID, HotpatchResult{AgentID: "a1", Status: "ok", At: "t3"}))

	gotHp, err := m.FindOne(ctx, hp.ID)
	require.NoError(t, err)
	assert.Contains(t, string(gotHp.Results), "t3")
	assert.Contains(t, string(gotHp.Results), "a2")
	assert.NotContains(t, string(gotHp.Results), "t1")

	// Missing hotpatch.
	assert.Error(t, m.AppendResult(ctx, 99999, HotpatchResult{AgentID: "a"}))
}

func TestBugModelListAllFiltersV9(t *testing.T) {
	db := newModelTestDB(t, &Bug{})
	m := NewBugModel(db)
	ctx := context.Background()

	bug := &Bug{
		Title: "crash on launch", Content: "stacktrace boom", Status: BugStatusTriage,
		Severity: BugSeverityBlocker, Priority: "urgent", Assignee: "dev-bob",
		GameID: "demo", Env: "prod", Platform: "android", FixVersion: "1.2.0",
		PlayerID: "p-42", CrashFingerprint: "fp1",
	}
	require.NoError(t, m.Create(ctx, bug))
	require.NoError(t, m.Create(ctx, &Bug{Title: "typo", Content: "x", Status: BugStatusConfirmed, GameID: "other"}))

	cases := []BugQueryOptions{
		{Query: "crash"},
		{Status: BugStatusTriage},
		{Severity: BugSeverityBlocker},
		{Priority: "urgent"},
		{Assignee: "dev-bob"},
		{GameID: "demo"},
		{Env: "prod"},
		{Platform: "android"},
		{FixVersion: "1.2.0"},
		{PlayerID: "p-42"},
	}
	for _, opts := range cases {
		opts.PaginationOptions = PaginationOptions{Page: 1, PageSize: 10}
		items, total, err := m.List(ctx, opts)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total, "filter %+v should match exactly one bug", opts)
		require.Len(t, items, 1)
	}

	// Open-by-fingerprint lookup: terminal statuses no longer aggregate.
	found, err := m.FindOpenByCrashFingerprint(ctx, "demo", "prod", "fp1")
	require.NoError(t, err)
	assert.Equal(t, bug.ID, found.ID)

	require.NoError(t, m.Update(ctx, bug.ID, map[string]interface{}{"status": BugStatusReleased}))
	_, err = m.FindOpenByCrashFingerprint(ctx, "demo", "prod", "fp1")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = m.FindOpenByCrashFingerprint(ctx, "demo", "", "missing")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// Delete: not-found surfaces ErrRecordNotFound.
	require.NoError(t, m.Delete(ctx, bug.ID))
	assert.ErrorIs(t, m.Delete(ctx, bug.ID), gorm.ErrRecordNotFound)

	_, err = m.FindOne(ctx, 99999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestConfigSourceBindingModelBranchesV9(t *testing.T) {
	db := newModelTestDB(t, &ConfigSourceBinding{})
	m := NewConfigSourceBindingModel(db)
	ctx := context.Background()

	// nil binding / invalid type / missing fields.
	assert.ErrorContains(t, m.Create(ctx, nil), "binding required")
	assert.ErrorContains(t, m.Create(ctx, &ConfigSourceBinding{GameID: "g", Env: "e", Name: "n", Type: "bogus"}), "invalid config source type")
	assert.ErrorContains(t, m.Create(ctx, &ConfigSourceBinding{GameID: "", Env: "e", Name: "n", Type: "git"}), "required")

	// NormalizeConfigSourceType direct checks.
	norm, ok := NormalizeConfigSourceType("  Nacos ")
	assert.True(t, ok)
	assert.Equal(t, "nacos", norm)
	_, ok = NormalizeConfigSourceType("bogus")
	assert.False(t, ok)

	// Create success (type gets normalized).
	created := &ConfigSourceBinding{GameID: " demo ", Env: " prod ", Name: " main ", Type: "REDIS", Config: "{}"}
	require.NoError(t, m.Create(ctx, created))
	assert.Equal(t, "demo", created.GameID)
	assert.Equal(t, "redis", created.Type)

	// Update / Get / Delete guards.
	assert.ErrorContains(t, m.Update(ctx, nil), "binding id required")
	assert.ErrorContains(t, m.Update(ctx, &ConfigSourceBinding{}), "binding id required")
	assert.ErrorContains(t, m.Delete(ctx, 0), "binding id required")
	_, err := m.Get(ctx, 0)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.NoError(t, m.Update(ctx, &ConfigSourceBinding{Model: gorm.Model{ID: created.ID}, Name: "renamed", Type: "redis"}))
	got, err := m.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Name)

	require.NoError(t, m.Delete(ctx, created.ID))
	_, err = m.Get(ctx, created.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// ListByScope filters.
	require.NoError(t, m.Create(ctx, &ConfigSourceBinding{GameID: "g1", Env: "e1", Name: "a", Type: "git"}))
	require.NoError(t, m.Create(ctx, &ConfigSourceBinding{GameID: "g1", Env: "e2", Name: "b", Type: "git"}))
	out, err := m.ListByScope(ctx, "g1", "e1")
	require.NoError(t, err)
	assert.Len(t, out, 1)
	out, err = m.ListByScope(ctx, "g1", "")
	require.NoError(t, err)
	assert.Len(t, out, 2)
}

func TestTaskScheduleModelBranchesV9(t *testing.T) {
	db := newModelTestDB(t, &TaskSchedule{}, &TaskScheduleRunLog{}, &TaskRun{})
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	// Validation branches.
	assert.ErrorContains(t, ValidateScheduleInput("", "* * * * *", "g", "e", "f"), "名称")
	assert.ErrorContains(t, ValidateScheduleInput("n", " ", "g", "e", "f"), "cron")
	assert.ErrorContains(t, ValidateScheduleInput("n", "* * * * *", " ", "e", "f"), "gameId/env")
	assert.ErrorContains(t, ValidateScheduleInput("n", "* * * * *", "g", "e", " "), "functionId")

	// Create with invalid input short-circuits.
	_, err := m.Create(ctx, CreateScheduleInput{Name: ""})
	assert.ErrorContains(t, err, "名称")

	// Create applies the default max-failed runs.
	s, err := m.Create(ctx, CreateScheduleInput{
		Name: "nightly", CronExpr: "0 3 * * *", GameID: "demo", Env: "prod",
		FunctionID: "demo.job", Actor: "admin",
	})
	require.NoError(t, err)
	assert.Equal(t, 5, s.MaxFailedRuns)
	assert.Equal(t, ScheduleStatusActive, s.Status)

	// List with pagination and filters.
	list, total, err := m.List(ctx, ListSchedulesOptions{Page: 1, PageSize: 10, GameID: "demo", Env: "prod", Status: ScheduleStatusActive})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, list, 1)

	// SetStatus active resets the failure counter.
	require.NoError(t, m.UpdateSchedule(ctx, s.ID, map[string]interface{}{"consecutive_failures": 3}))
	require.NoError(t, m.SetStatus(ctx, s.ID, ScheduleStatusActive))
	got, err := m.FindByID(ctx, s.ID)
	require.NoError(t, err)
	assert.Zero(t, got.ConsecutiveFailures)

	// Duplicate run-log slot returns (false, nil).
	slot := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	log1 := &TaskScheduleRunLog{ScheduleID: s.ID, Slot: slot, TaskRunID: "run-1", Status: "dispatched"}
	ok, err := m.CreateRunLog(ctx, log1)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = m.CreateRunLog(ctx, &TaskScheduleRunLog{ScheduleID: s.ID, Slot: slot, TaskRunID: "run-2"})
	require.NoError(t, err)
	assert.False(t, ok)

	has, err := m.HasRunLog(ctx, s.ID, slot)
	require.NoError(t, err)
	assert.True(t, has)

	// isDuplicateKeyError unit branches.
	assert.False(t, isDuplicateKeyError(nil))
	assert.True(t, isDuplicateKeyError(errDummyV9("UNIQUE constraint failed: x")))
	assert.True(t, isDuplicateKeyError(errDummyV9("Duplicate entry 1 for key")))
	assert.True(t, isDuplicateKeyError(errDummyV9("duplicate key violates")))
	assert.False(t, isDuplicateKeyError(errDummyV9("boom")))

	// LastRunStatus: empty id, missing run, terminal and non-terminal statuses.
	st, err := m.LastRunStatus(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, st)
	st, err = m.LastRunStatus(ctx, "nope")
	require.NoError(t, err)
	assert.Empty(t, st)

	require.NoError(t, db.Create(&TaskRun{TaskID: "run-done", Status: "succeeded"}).Error)
	require.NoError(t, db.Create(&TaskRun{TaskID: "run-mid", Status: "running"}).Error)
	st, err = m.LastRunStatus(ctx, "run-done")
	require.NoError(t, err)
	assert.Equal(t, "succeeded", st)
	st, err = m.LastRunStatus(ctx, "run-mid")
	require.NoError(t, err)
	assert.Empty(t, st)

	// ListRunLogs pagination.
	logs, total, err := m.ListRunLogs(ctx, s.ID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, logs, 1)

	// Delete.
	require.NoError(t, m.Delete(ctx, s.ID))
	_, err = m.FindByID(ctx, s.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestDBSourceValidateMaskAndDeleteV9(t *testing.T) {
	db := newModelTestDB(t, &DBSource{})
	m := NewDBSourceModel(db)
	ctx := context.Background()

	// Validation branches.
	root := &DBSource{Name: "mysql-prod", Driver: "mysql", Kind: "self", DSN: "root:pass@tcp(1.2.3.4)/db"}
	assert.ErrorContains(t, ValidateDBSource(root), "只读监控账号")
	badDSN := &DBSource{Name: "pg", Driver: "postgres", Kind: "self", DSN: "plainhost"}
	assert.ErrorContains(t, ValidateDBSource(badDSN), "DSN 格式无效")

	// MaskedDSN branches.
	assert.Equal(t, "mysql://***@host/db", (&DBSource{DSN: "mysql://user:secret@host/db"}).MaskedDSN())
	assert.Equal(t, "mysql://***@host/db", (&DBSource{DSN: "mysql://user@host/db"}).MaskedDSN())
	assert.Equal(t, "user:***@host/db", (&DBSource{DSN: "user:secret@host/db"}).MaskedDSN())
	assert.Equal(t, "***", (&DBSource{DSN: "no-at-sign"}).MaskedDSN())

	// FindOne / Delete.
	src := &DBSource{Name: "s1", Driver: "mysql", Kind: "self", DSN: "u:p@h/d", Enabled: true}
	require.NoError(t, m.Create(ctx, src))
	got, err := m.FindOne(ctx, src.ID)
	require.NoError(t, err)
	assert.Equal(t, "s1", got.Name)
	_, err = m.FindOne(ctx, 99999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.NoError(t, m.Delete(ctx, src.ID))
	assert.ErrorIs(t, m.Delete(ctx, src.ID), gorm.ErrRecordNotFound)
}

func TestAgentSessionModelBadJSONV9(t *testing.T) {
	db := newModelTestDB(t, &AgentSessionDB{})
	m := NewAgentSessionModel(db)
	ctx := context.Background()

	future := time.Now().Add(time.Hour)
	require.NoError(t, db.Exec(`INSERT INTO agent_sessions
		(agent_id, game_id, env, labels, providers, expire_at, last_seen, created_at, updated_at)
		VALUES ('bad-labels', 'g', 'e', 'not-json', NULL, ?, ?, ?, ?)`,
		future, time.Now(), time.Now(), time.Now()).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_sessions
		(agent_id, game_id, env, labels, providers, expire_at, last_seen, created_at, updated_at)
		VALUES ('bad-providers', 'g', 'e', NULL, 'also-not-json', ?, ?, ?, ?)`,
		future, time.Now(), time.Now(), time.Now()).Error)

	sessions, err := m.LoadActiveSessions(ctx)
	require.NoError(t, err)
	assert.Empty(t, sessions, "sessions with corrupt JSON must be skipped")
}

func TestJSONAndContractHelpersV9(t *testing.T) {
	// contractSemanticallyEqual nil guards.
	assert.False(t, contractSemanticallyEqual(nil, &FunctionContract{}))
	assert.False(t, contractSemanticallyEqual(&FunctionContract{}, nil))

	// canonicalJSON passes invalid JSON through untouched.
	assert.Equal(t, []byte("not-json"), canonicalJSON(JSON("not-json")))
	assert.Nil(t, canonicalJSON(nil))

	// jsonMapEqual: length mismatch and zero-value normalization.
	assert.False(t, jsonMapEqual(datatypes.JSONMap{"a": true}, datatypes.JSONMap{"a": true, "b": "x"}))
	assert.True(t, jsonMapEqual(datatypes.JSONMap{"required": false, "policyKey": ""}, nil))
	assert.True(t, jsonMapEqual(nil, datatypes.JSONMap{}))

	// normalizeJSONMap drops zero-valued entries.
	normalized := normalizeJSONMap(datatypes.JSONMap{"b": false, "s": "  ", "keep": "v", "n": 1.5})
	assert.Equal(t, datatypes.JSONMap{"keep": "v", "n": 1.5}, normalized)

	// Structurally different schemas are not equal.
	a := &FunctionContract{Version: "1", InputSchema: JSON(`{"a":1}`)}
	b := &FunctionContract{Version: "1", InputSchema: JSON(`{"a":2}`)}
	assert.False(t, contractSemanticallyEqual(a, b))
	// Key order / spacing differences are equal after canonicalization.
	c := &FunctionContract{Version: "1", InputSchema: JSON(`{ "b" : 2, "a" : 1 }`)}
	d := &FunctionContract{Version: "1", InputSchema: JSON(`{"a":1,"b":2}`)}
	assert.True(t, contractSemanticallyEqual(c, d))
	// Trimmed permission equality.
	e := &FunctionContract{Version: "1", Permission: " perm "}
	f := &FunctionContract{Version: "1", Permission: "perm"}
	assert.True(t, contractSemanticallyEqual(e, f))

	// OpenAPISource setters: unmarshalable payloads error out.
	src := &OpenAPISource{}
	assert.Error(t, src.SetOperations(make(chan int)))
	assert.Error(t, src.SetDiagnostics(make(chan int)))
	require.NoError(t, src.SetOperations([]string{"op"}))
	require.NoError(t, src.SetDiagnostics(map[string]string{"k": "v"}))
	var ops []string
	require.NoError(t, src.GetOperations(&ops))
	assert.Equal(t, []string{"op"}, ops)

	// Game config setter error path.
	assert.Error(t, (&Game{}).SetConfig(make(chan int)))

	// MustJSON: nil and failing marshallers fall back to "null".
	assert.Equal(t, "null", string(MustJSON(nil)))
	assert.Equal(t, "null", string(MustJSON(v9JSONStub{})))
	assert.Equal(t, `{"a":1}`, string(MustJSON(map[string]int{"a": 1})))
}

// v9JSONStub makes json.Marshal fail (and produce no bytes).
type v9JSONStub struct{}

func (v9JSONStub) MarshalJSON() ([]byte, error) { return nil, assert.AnError }

type errDummyV9 string

func (e errDummyV9) Error() string { return string(e) }
