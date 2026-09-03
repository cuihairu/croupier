package model

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func gormErrRecordNotFoundV9() error { return gorm.ErrRecordNotFound }

func v9Session(agentID string) *registry.AgentSession {
	return &registry.AgentSession{
		AgentID: agentID, GameID: "demo", Env: "prod",
		ExpireAt: time.Now().Add(time.Hour), LastSeen: time.Now(),
	}
}

// This file sweeps the remaining DB-error branches using a closed database
// (newClosedDB) plus a few filter branches exercised on healthy databases.

func TestErrorPathsV9ClosedDBModels(t *testing.T) {
	ctx := context.Background()
	db := newClosedDB(t)

	t.Run("admin security", func(t *testing.T) {
		m := NewAdminModel(db)
		_, _, err := m.RecordLoginFailure(ctx, 1, 3, time.Minute)
		assert.Error(t, err)
		assert.Error(t, m.ResetLoginFailures(ctx, 1))
		_, err = m.GetTokenVersion(ctx, 1)
		assert.Error(t, err)
		assert.Error(t, m.BumpTokenVersion(ctx, 1))
		assert.Error(t, m.SetOTPSecret(ctx, 1, "secret"))
		assert.Error(t, m.EnableOTP(ctx, 1))
		assert.Error(t, m.DisableOTP(ctx, 1))
	})

	t.Run("agent session", func(t *testing.T) {
		m := NewAgentSessionModel(db)
		assert.Error(t, m.Upsert(ctx, v9Session("closed-agent")))
	})

	t.Run("profile", func(t *testing.T) {
		m := NewProfileModel(db)
		assert.Error(t, m.ReplacePermissions(ctx, 1, []ProfilePermission{{AdminID: 1, Resource: "games"}}))
		assert.Error(t, m.ReplaceGames(ctx, 1, []ProfileGame{{AdminID: 1, GameID: "g"}}))
		_, err := m.ListPermissions(ctx, 1)
		assert.Error(t, err)
		_, err = m.ListGames(ctx, 1)
		assert.Error(t, err)
	})

	t.Run("function models", func(t *testing.T) {
		fm := NewFunctionModel(db)
		_, err := fm.ListDescriptors(ctx, "fn")
		assert.Error(t, err)
		_, err = fm.ListPermissions(ctx, "fn")
		assert.Error(t, err)
		_, err = fm.ListPending(ctx)
		assert.Error(t, err)

		cm := NewCapabilitySemanticVersionModel(db)
		_, _, err = cm.ListBySemanticsIDPaged(ctx, 1, 10, 0)
		assert.Error(t, err)
	})
}

func TestErrorPathsV9ListHelpers(t *testing.T) {
	ctx := context.Background()

	t.Run("alert rule", func(t *testing.T) {
		healthy := newModelTestDB(t, &AlertRule{})
		hm := NewAlertRuleModel(healthy)
		enabled := true
		require.NoError(t, hm.Create(ctx, &AlertRule{Name: "cpu", Metric: "cpu.usagePercent", Operator: "gt", Threshold: 90, Enabled: false}))
		rules, err := hm.List(ctx, ListAlertRulesOptions{Enabled: &enabled, Metric: "cpu.usagePercent"})
		require.NoError(t, err)
		assert.Empty(t, rules)
		disabled := false
		rules, err = hm.List(ctx, ListAlertRulesOptions{Enabled: &disabled, Metric: "cpu.usagePercent"})
		require.NoError(t, err)
		assert.Len(t, rules, 1)

		m := NewAlertRuleModel(newClosedDB(t))
		_, err = m.List(ctx, ListAlertRulesOptions{})
		assert.Error(t, err)
		assert.Error(t, m.Create(ctx, &AlertRule{Name: "x", Metric: "m", Operator: "gt", Threshold: 1}))
	})

	t.Run("alert", func(t *testing.T) {
		m := NewAlertModel(newClosedDB(t))
		_, _, err := m.List(ctx, ListAlertsOptions{})
		assert.Error(t, err)
	})

	t.Run("backup", func(t *testing.T) {
		m := NewBackupModel(newClosedDB(t))
		_, _, err := m.List(ctx, ListBackupsOptions{})
		assert.Error(t, err)
	})

	t.Run("ticket", func(t *testing.T) {
		m := NewTicketModel(newClosedDB(t))
		_, _, err := m.List(ctx, TicketQueryOptions{})
		assert.Error(t, err)
	})

	t.Run("certificate", func(t *testing.T) {
		m := NewCertificateModel(newClosedDB(t))
		_, _, err := m.List(ctx, ListCertificatesOptions{})
		assert.Error(t, err)
		_, _, err = m.ListAlerts(ctx, 1, 10)
		assert.Error(t, err)
	})

	t.Run("faq", func(t *testing.T) {
		healthy := newModelTestDB(t, &FAQ{})
		hm := NewFAQModel(healthy)
		visible := false
		_, _, err := hm.List(ctx, ListFAQOptions{Category: "c", Keyword: "k", Tag: "t", Visible: &visible, OrderByHelpful: true})
		require.NoError(t, err)
		assert.Error(t, hm.Vote(ctx, 1, true))

		m := NewFAQModel(newClosedDB(t))
		_, _, err = m.List(ctx, ListFAQOptions{})
		assert.Error(t, err)
		assert.Error(t, m.Vote(ctx, 1, false))
	})

	t.Run("message", func(t *testing.T) {
		m := NewMessageModel(newClosedDB(t))
		_, _, err := m.List(ctx, ListMessagesOptions{})
		assert.Error(t, err)
		_, err = m.Recent(ctx, 5, "bob")
		assert.Error(t, err)
	})

	t.Run("support", func(t *testing.T) {
		m := NewSupportModel(newClosedDB(t))
		_, _, err := m.ListTickets(ctx, ListTicketsOptions{})
		assert.Error(t, err)
	})

	t.Run("task", func(t *testing.T) {
		m := NewTaskRunModel(newClosedDB(t))
		_, _, err := m.List(ctx, ListTasksOptions{})
		assert.Error(t, err)
	})

	t.Run("feedback", func(t *testing.T) {
		m := NewFeedbackModel(newClosedDB(t))
		_, _, err := m.List(ctx, ListFeedbackOptions{})
		assert.Error(t, err)
		_, err = m.Stats(ctx, FeedbackStatsOptions{GameID: "demo", Days: 7})
		assert.Error(t, err)
	})

	t.Run("analytics", func(t *testing.T) {
		behave := NewBehaviorModel(newClosedDB(t))
		_, _, err := behave.ListEvents(ctx, BehaviorEventOptions{})
		assert.Error(t, err)
		now := time.Now()
		_, err = behave.DailyActivity(ctx, "demo", "prod", now.Add(-time.Hour), now)
		assert.Error(t, err)

		pay := NewPaymentsModel(newClosedDB(t))
		_, _, err = pay.ListTransactions(ctx, PaymentQueryOptions{})
		assert.Error(t, err)
		_, err = pay.DailyRevenue(ctx, "demo", "prod", now.Add(-time.Hour), now)
		assert.Error(t, err)
	})

	t.Run("rate limit", func(t *testing.T) {
		healthy := newModelTestDB(t, &RateLimit{})
		hm := NewRateLimitModel(healthy)
		assert.ErrorIs(t, hm.DeleteByKey(ctx, "missing"), gormErrRecordNotFoundV9())

		m := NewRateLimitModel(newClosedDB(t))
		assert.Error(t, m.DeleteByKey(ctx, "k"))
	})

	t.Run("platform setting", func(t *testing.T) {
		m := NewPlatformSettingModel(newClosedDB(t))
		_, err := m.List(ctx)
		assert.Error(t, err)
		_, _, err = m.Get(ctx, "k")
		assert.Error(t, err)
		assert.Error(t, m.Clear(ctx, "k"))
	})

	t.Run("tool link", func(t *testing.T) {
		healthy := newModelTestDB(t, &ToolLink{})
		hm := NewToolLinkModel(healthy)
		assert.ErrorIs(t, hm.Delete(ctx, 999), gormErrRecordNotFoundV9())

		m := NewToolLinkModel(newClosedDB(t))
		assert.Error(t, m.Delete(ctx, 1))
	})

	t.Run("component template", func(t *testing.T) {
		healthy := newModelTestDB(t, &ComponentTemplate{})
		hm := NewComponentTemplateModel(healthy)
		assert.ErrorIs(t, hm.Create(ctx, &ComponentTemplate{Key: "  "}), ErrComponentTemplateKeyRequired)
		assert.ErrorIs(t, hm.UpsertBuiltin(ctx, &ComponentTemplate{Key: ""}), ErrComponentTemplateKeyRequired)
		_, _, err := hm.List(ctx, ComponentTemplateListOptions{Category: "layout", BuiltinOnly: true, CreatedBy: "admin"})
		require.NoError(t, err)

		m := NewComponentTemplateModel(newClosedDB(t))
		_, _, err = m.List(ctx, ComponentTemplateListOptions{})
		assert.Error(t, err)
	})

	t.Run("config version", func(t *testing.T) {
		healthy := newModelTestDB(t, &ConfigVersion{})
		hm := NewConfigVersionModel(healthy)
		_, err := hm.CreateWithMeta(ctx, ConfigVersionPayload{Key: "k", Namespace: "bogus"}, "admin")
		assert.ErrorContains(t, err, "invalid config namespace")
		_, err = hm.CreateWithMeta(ctx, ConfigVersionPayload{Key: "k", BaseVersion: 3}, "admin")
		assert.ErrorContains(t, err, "base version mismatch")

		m := NewConfigVersionModel(newClosedDB(t))
		_, err = m.CreateWithMeta(ctx, ConfigVersionPayload{Key: "k"}, "admin")
		assert.Error(t, err)
	})

	t.Run("term dictionary", func(t *testing.T) {
		healthy := newModelTestDB(t, &TermDictionary{})
		hm := NewTermDictionaryModel(healthy)
		require.NoError(t, healthy.Exec(`INSERT INTO term_dictionary (domain, term_key, alias, sort_order, created_at, updated_at) VALUES ('bogus', 'tk', 'alias', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
		_, err := hm.AliasMap(ctx)
		assert.ErrorContains(t, err, "unsupported term dictionary domain")

		m := NewTermDictionaryModel(newClosedDB(t))
		_, err = m.List(ctx, "")
		assert.Error(t, err)
		assert.Error(t, m.Upsert(ctx, &TermDictionary{Domain: TermDomainResource, TermKey: "tk", Alias: "a"}))
		_, err = m.AliasMap(ctx)
		assert.Error(t, err)
	})
}

func TestErrorPathsV9PageModels(t *testing.T) {
	ctx := context.Background()
	db := newClosedDB(t)

	ps := NewPageSpecModel(db)
	_, err := ps.FindByScopeAndPageKey(ctx, "g", "e", "k")
	assert.Error(t, err)
	_, err = ps.ListByScope(ctx, "g", "e")
	assert.Error(t, err)
	_, err = ps.ListByScopeAndStatus(ctx, "g", "e", "draft")
	assert.Error(t, err)
	assert.Error(t, ps.Upsert(ctx, &PageSpec{GameID: "g", Env: "e", PageKey: "k"}))

	pub := NewPublishedPageSpecModel(db)
	_, err = pub.FindByScopePageKeyAndVersion(ctx, "g", "e", "k", 1)
	assert.Error(t, err)
	_, err = pub.ListByScope(ctx, "g", "e")
	assert.Error(t, err)
	assert.Error(t, pub.DeactivatePage(ctx, "g", "e", "k", time.Now()))
	assert.Error(t, pub.Create(ctx, &PublishedPageSpec{GameID: "g", Env: "e", PageKey: "k"}))
	assert.Error(t, pub.DeleteByScopeAndPageKey(ctx, "g", "e", "k"))

	pv := NewPageVersionModel(db)
	_, err = pv.ListByScopeAndPageKey(ctx, "g", "e", "k")
	assert.Error(t, err)
	_, _, err = pv.ListByScopeAndPageKeyPaged(ctx, "g", "e", "k", 10, 0)
	assert.Error(t, err)
	_, err = pv.GetNextVersion(ctx, "g", "e", "k")
	assert.Error(t, err)

	prop := NewPageProposalModel(db)
	_, err = prop.FindByScopeAndKey(ctx, "g", "e", "k")
	assert.Error(t, err)
	_, err = prop.FindByScopeAndPageKey(ctx, "g", "e", "k")
	assert.Error(t, err)
	_, err = prop.ListByScope(ctx, "g", "e")
	assert.Error(t, err)
	_, err = prop.ListByStatus(ctx, "g", "e", 1)
	assert.Error(t, err)
	_, err = prop.ListByScopeAndResourceKey(ctx, "g", "e", "r")
	assert.Error(t, err)
	_, err = prop.ListByScopeStatusAndResourceKey(ctx, "g", "e", 1, "r")
	assert.Error(t, err)
	assert.Error(t, prop.DeleteByScopeAndKey(ctx, "g", "e", "k"))
	assert.Error(t, prop.DeleteByScopeAndPageKey(ctx, "g", "e", "k"))

	ver := NewPageProposalVersionModel(db)
	assert.Error(t, ver.CreateVersion(ctx, &PageProposalVersion{}))
	_, err = ver.ListByProposalID(ctx, 1)
	assert.Error(t, err)
	_, err = ver.FindByProposalIDAndVersion(ctx, 1, 1)
	assert.Error(t, err)
	_, err = ver.LatestByProposalID(ctx, 1)
	assert.Error(t, err)
	_, err = ver.GetNextVersion(ctx, 1)
	assert.Error(t, err)

	issue := NewBlockedProposalIssueModel(db)
	assert.Error(t, issue.Upsert(ctx, &BlockedProposalIssue{GameID: "g", Env: "e", ResourceKey: "r"}))
	_, err = issue.FindByScopeAndResourceKey(ctx, "g", "e", "r")
	assert.Error(t, err)
	_, err = issue.ListByScope(ctx, "g", "e")
	assert.Error(t, err)
	_, err = issue.ListByScopeAndResourceKey(ctx, "g", "e", "r")
	assert.Error(t, err)
}

func TestPageDeleteByScopeAndPageKeyV9(t *testing.T) {
	db := newModelTestDB(t, &PublishedPageSpec{}, &PageProposal{})
	ctx := context.Background()

	pub := NewPublishedPageSpecModel(db)
	require.NoError(t, pub.Create(ctx, &PublishedPageSpec{GameID: "g", Env: "e", PageKey: "k", Version: 1, RendererSchemaVersion: "1"}))
	require.NoError(t, pub.Create(ctx, &PublishedPageSpec{GameID: "g", Env: "e", PageKey: "other", Version: 1, RendererSchemaVersion: "1"}))
	require.NoError(t, pub.DeleteByScopeAndPageKey(ctx, "g", "e", "k"))
	items, err := pub.ListByScope(ctx, "g", "e")
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "other", items[0].PageKey)

	prop := NewPageProposalModel(db)
	require.NoError(t, db.Create(&PageProposal{GameID: "g", Env: "e", ProposalKey: "p1", PageKey: "k", Status: 1}).Error)
	require.NoError(t, db.Create(&PageProposal{GameID: "g", Env: "e", ProposalKey: "p2", PageKey: "keep", Status: 1}).Error)
	require.NoError(t, prop.DeleteByScopeAndPageKey(ctx, "g", "e", "k"))
	left, err := prop.ListByScope(ctx, "g", "e")
	require.NoError(t, err)
	assert.Len(t, left, 1)
	assert.Equal(t, "keep", left[0].PageKey)
}

func TestErrorPathsV9SchedulesAndReleases(t *testing.T) {
	ctx := context.Background()

	sched := NewTaskScheduleModel(newClosedDB(t))
	_, err := sched.Create(ctx, CreateScheduleInput{Name: "n", CronExpr: "* * * * *", GameID: "g", Env: "e", FunctionID: "f"})
	assert.Error(t, err)
	_, err = sched.FindByID(ctx, 1)
	assert.Error(t, err)
	_, _, err = sched.List(ctx, ListSchedulesOptions{})
	assert.Error(t, err)
	assert.Error(t, sched.UpdateSchedule(ctx, 1, map[string]interface{}{"status": "paused"}))
	assert.Error(t, sched.SetStatus(ctx, 1, ScheduleStatusActive))
	assert.Error(t, sched.Delete(ctx, 1))
	_, err = sched.ListDue(ctx, time.Now(), 10)
	assert.Error(t, err)
	_, err = sched.HasRunLog(ctx, 1, time.Now())
	assert.Error(t, err)
	_, err = sched.CreateRunLog(ctx, &TaskScheduleRunLog{ScheduleID: 1, Slot: time.Now()})
	assert.Error(t, err)
	_, err = sched.LastRunStatus(ctx, "run-1")
	assert.Error(t, err)
	_, _, err = sched.ListRunLogs(ctx, 1, 1, 10)
	assert.Error(t, err)

	rel := NewGameReleaseModel(newClosedDB(t))
	_, _, err = rel.List(ctx, ReleaseQueryOptions{})
	assert.Error(t, err)
	_, err = rel.FindOne(ctx, 1)
	assert.Error(t, err)
	_, err = rel.Transition(ctx, 1, ReleaseStatusArchived, nil)
	assert.Error(t, err)

	hot := NewHotpatchModel(newClosedDB(t))
	_, _, err = hot.List(ctx, HotpatchQueryOptions{})
	assert.Error(t, err)
	_, err = hot.FindOne(ctx, 1)
	assert.Error(t, err)
	_, err = hot.Transition(ctx, 1, HotpatchStatusFailed, nil)
	assert.Error(t, err)
	assert.Error(t, hot.AppendResult(ctx, 1, HotpatchResult{AgentID: "a"}))

	bug := NewBugModel(newClosedDB(t))
	_, _, err = bug.List(ctx, BugQueryOptions{})
	assert.Error(t, err)

	dbSrc := NewDBSourceModel(newClosedDB(t))
	_, err = dbSrc.FindOne(ctx, 1)
	assert.Error(t, err)
	assert.Error(t, dbSrc.Delete(ctx, 1))
}

func TestGameModelEnvBindingErrorsV9(t *testing.T) {
	db := newModelTestDB(t, &Game{}, &GameEnvBinding{})
	m := NewGameModel(db)
	ctx := context.Background()

	game := &Game{Name: "Env Error Game", AliasName: "enverrorgame", GameID: "enverrordemo"}
	require.NoError(t, m.Create(ctx, game))

	// Upsert binding without a database name fails.
	err := m.UpdateEnvsAndBindings(ctx, game.GameID, game.ID, game.Envs, []string{" "}, []GameEnvBinding{{Env: "prod", DatabaseName: ""}})
	assert.ErrorContains(t, err, "requires env and database name")

	// Valid binding plus blank removeEnvs entries.
	err = m.UpdateEnvsAndBindings(ctx, game.GameID, game.ID, JSON(`[{"env":"prod"},{"env":"stale"}]`), []string{"", "stale"}, []GameEnvBinding{{Env: "prod", DatabaseName: "game_demo_prod"}})
	require.NoError(t, err)

	// Backfill guards.
	_, err = m.BackfillEnvBindings(ctx, nil)
	assert.ErrorContains(t, err, "resolver is required")

	created, err := m.BackfillEnvBindings(ctx, func(gameID, env string) string { return "db_" + gameID + "_" + env })
	require.NoError(t, err)
	assert.Equal(t, 1, created) // the "stale" env from the JSON is backfilled; prod already exists

	// A new env appears in the JSON: the resolver returns an empty name.
	require.NoError(t, db.Model(game).Update("envs", JSON(`[{"env":"prod"},{"env":"dev"}]`)).Error)
	_, err = m.BackfillEnvBindings(ctx, func(string, string) string { return "" })
	assert.ErrorContains(t, err, "empty database name")

	// A game with corrupt envs JSON makes the backfill fail.
	require.NoError(t, db.Create(&Game{Name: "Broken Envs", AliasName: "brokenenvs", GameID: "brokenenvs", Envs: JSON("not-json")}).Error)
	_, err = m.BackfillEnvBindings(ctx, func(string, string) string { return "db" })
	assert.ErrorContains(t, err, "decode environments")
}

func TestUpsertContractSkipsUnchangedV9(t *testing.T) {
	db := newModelTestDB(t, &FunctionContract{})
	m := NewFunctionContractModel(db)
	ctx := context.Background()

	base := &FunctionContract{
		GameID: "demo", Env: "prod", FunctionID: "demo.list", Version: "1",
		ResourceKey: "demo", OperationKey: "list", Capability: 2, Execution: "sync",
		Risk: 1, Permission: "perm", Source: "sdk",
		Approval:     datatypes.JSONMap{"required": false},
		InputSchema:  JSON(`{"a":1}`),
		OutputSchema: JSON(`{"b":2}`),
	}
	require.NoError(t, m.UpsertContract(ctx, base))

	// Re-registering the same contract is a no-op (no updated_at churn).
	same := *base
	require.NoError(t, m.UpsertContract(ctx, &same))

	// A soft-deleted contract is revived by re-registration.
	require.NoError(t, db.Exec(`UPDATE function_contracts SET deleted_at = CURRENT_TIMESTAMP WHERE function_id = 'demo.list'`).Error)
	revived := *base
	require.NoError(t, m.UpsertContract(ctx, &revived))
	var count int64
	require.NoError(t, db.Unscoped().Model(&FunctionContract{}).Where("function_id = ? AND deleted_at IS NULL", "demo.list").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
