package model

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newClosedDB opens a fully migrated in-memory database and closes the
// underlying pool. Every subsequent gorm operation fails, which lets these
// tests exercise the error-handling branches of the model helpers.
func newClosedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupAllModelsDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return db
}

func TestErrorPaths_AdminModel(t *testing.T) {
	m := NewAdminModel(newClosedDB(t))
	ctx := context.Background()

	longPassword := strings.Repeat("x", 100)
	assert.Error(t, m.Create(ctx, &Admin{Username: "u"}, longPassword))
	assert.Error(t, m.Create(ctx, &Admin{Username: "u"}, "short"))
	_, err := m.FindOne(ctx, 1)
	assert.Error(t, err)
	_, err = m.FindByUsername(ctx, "u")
	assert.Error(t, err)
	assert.Error(t, m.Update(ctx, 1, map[string]interface{}{"status": 1}))
	_, _, err = m.List(ctx, ListAdminsOptions{Search: "x"})
	assert.Error(t, err)
	_, err = m.ValidatePassword(ctx, "u", "pw")
	assert.Error(t, err)
	assert.Error(t, m.UpdatePassword(ctx, 1, "pw"))
	_, err = m.GetLastScope(ctx, 1)
	assert.Error(t, err)
	_, err = m.GetAdminRoles(ctx, 1)
	assert.Error(t, err)
}

func TestErrorPaths_PlayerModel(t *testing.T) {
	m := NewPlayerModel(newClosedDB(t))
	ctx := context.Background()

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)

	assert.Error(t, m.Create(ctx, &Player{Username: "p"}, ""))
	_, err := m.FindOne(ctx, 1)
	assert.Error(t, err)
	_, err = m.FindByUsername(ctx, "p", "")
	assert.Error(t, err)
	assert.Error(t, m.Update(ctx, 1, map[string]interface{}{"level": 2}))
	_, _, err = m.List(ctx, ListPlayersOptions{GameID: "demo"})
	assert.Error(t, err)
	_, err = m.ValidatePassword(ctx, "p", "pw", "")
	assert.Error(t, err)
	assert.Error(t, m.UpdatePassword(ctx, 1, "pw"))
	_, err = m.UpdateBalance(ctx, 1, 10, "test")
	assert.Error(t, err)
	_, err = m.CountNewPlayers(ctx, "demo", start, end)
	assert.Error(t, err)
	_, err = m.DailyNewPlayers(ctx, "", start, end)
	assert.Error(t, err)
}

func TestErrorPaths_GameModel(t *testing.T) {
	m := NewGameModel(newClosedDB(t))
	ctx := context.Background()

	_, err := m.FindOne(ctx, 1)
	assert.Error(t, err)
	_, err = m.FindByName(ctx, "demo")
	assert.Error(t, err)
	_, err = m.FindByGameIDString(ctx, "demo")
	assert.Error(t, err)
	_, err = m.ExistsByNameIgnoreCase(ctx, "demo")
	assert.Error(t, err)
	_, _, err = m.List(ctx, ListGamesOptions{})
	assert.Error(t, err)
	_, err = m.ListAll(ctx)
	assert.Error(t, err)
	assert.Error(t, m.AddEnvBinding(ctx, "demo", "prod", "db", "", ""))
	_, err = m.FindEnvBinding(ctx, "demo", "prod")
	assert.Error(t, err)
	_, err = m.LookupDatabaseName(ctx, "demo", "prod")
	assert.Error(t, err)
	_, err = m.HasEnvBinding(ctx, "demo", "prod")
	assert.Error(t, err)
	_, err = m.BackfillEnvBindings(ctx, func(string, string) string { return "db" })
	assert.Error(t, err)
	assert.Error(t, m.UpdateEnvsAndBindings(ctx, "demo", 1, JSON(`[]`), nil, nil))
	assert.Error(t, m.DeleteWithEnvBindings(ctx, 1, "demo"))
	_, err = m.ListEnvBindings(ctx, "demo")
	assert.Error(t, err)
	_, err = m.ListAllEnvBindings(ctx)
	assert.Error(t, err)
}

func TestErrorPaths_TicketSupportBackupModels(t *testing.T) {
	db := newClosedDB(t)
	ctx := context.Background()

	tm := NewTicketModel(db)
	_, _, err := tm.List(ctx, TicketQueryOptions{})
	assert.Error(t, err)
	_, err = tm.FindOne(ctx, 1)
	assert.Error(t, err)
	_, err = tm.ListComments(ctx, 1)
	assert.Error(t, err)

	sm := NewSupportModel(db)
	_, _, err = sm.ListTickets(ctx, ListTicketsOptions{})
	assert.Error(t, err)
	_, err = sm.ListComments(ctx, 1)
	assert.Error(t, err)
	_, err = sm.ListFAQs(ctx)
	assert.Error(t, err)
	_, err = sm.ListFeedback(ctx)
	assert.Error(t, err)

	bm := NewBackupModel(db)
	_, _, err = bm.List(ctx, ListBackupsOptions{})
	assert.Error(t, err)
	_, err = bm.FindByID(ctx, 1)
	assert.Error(t, err)
	_, err = bm.FindByBackupID(ctx, "b")
	assert.Error(t, err)
}

func TestErrorPaths_AlertModel(t *testing.T) {
	m := NewAlertModel(newClosedDB(t))
	ctx := context.Background()

	_, _, err := m.List(ctx, ListAlertsOptions{})
	assert.Error(t, err)
	_, err = m.FindByAlertID(ctx, "a1")
	assert.Error(t, err)
	assert.Error(t, m.Create(ctx, &Alert{AlertID: "a1"}))
	assert.Error(t, m.BootstrapAlerts(ctx, []Alert{{AlertID: "a1"}}))
	_, err = m.FindByIDs(ctx, []uint{1})
	assert.Error(t, err)
	_, err = m.ListSilences(ctx, ListSilencesOptions{ActiveOnly: true})
	assert.Error(t, err)
	assert.Error(t, m.CreateSilence(ctx, &AlertSilence{DurationMinute: 5}))
	assert.Error(t, m.DeleteSilence(ctx, 1))
	assert.Error(t, m.PruneExpiredSilences(ctx))
}

func TestErrorPaths_FeedbackMessageModels(t *testing.T) {
	db := newClosedDB(t)
	ctx := context.Background()

	fm := NewFeedbackModel(db)
	_, _, err := fm.List(ctx, ListFeedbackOptions{})
	assert.Error(t, err)
	_, err = fm.Stats(ctx, FeedbackStatsOptions{Days: 3})
	assert.Error(t, err)

	mm := NewMessageModel(db)
	_, _, err = mm.List(ctx, ListMessagesOptions{})
	assert.Error(t, err)
	_, err = mm.Recent(ctx, 10, "admin")
	assert.Error(t, err)
	_, err = mm.CountUnread(ctx, "")
	assert.Error(t, err)
	_, err = mm.FindOne(ctx, 1)
	assert.Error(t, err)
	assert.Error(t, mm.MarkRead(ctx, 1))
	assert.Error(t, mm.Create(ctx, &Message{To: "a"}))
}

func TestErrorPaths_CertificateFAQModels(t *testing.T) {
	db := newClosedDB(t)
	ctx := context.Background()

	cm := NewCertificateModel(db)
	_, _, err := cm.List(ctx, ListCertificatesOptions{})
	assert.Error(t, err)
	_, err = cm.ListAll(ctx)
	assert.Error(t, err)
	_, err = cm.ExpiringWithin(ctx, time.Hour)
	assert.Error(t, err)
	_, err = cm.Stats(ctx)
	assert.Error(t, err)
	_, _, err = cm.ListAlerts(ctx, 1, 10)
	assert.Error(t, err)
	_, err = cm.FindOne(ctx, 1)
	assert.Error(t, err)
	_, err = cm.FindByDomain(ctx, "d")
	assert.Error(t, err)
	assert.Error(t, cm.Create(ctx, &Certificate{Domain: "d"}))

	fm := NewFAQModel(db)
	_, _, err = fm.List(ctx, ListFAQOptions{})
	assert.Error(t, err)
	_, err = fm.ListCategories(ctx)
	assert.Error(t, err)
	_, err = fm.FindOne(ctx, 1)
	assert.Error(t, err)
	assert.Error(t, fm.UpsertCategory(ctx, &FAQCategory{Name: "c"}))
}

func TestErrorPaths_FunctionModel(t *testing.T) {
	m := NewFunctionModel(newClosedDB(t))
	ctx := context.Background()

	_, _, err := m.List(ctx, ListFunctionsOptions{})
	assert.Error(t, err)
	_, err = m.CopyFunction(ctx, "f1")
	assert.Error(t, err)
	_, _, err = m.BatchUpdateStatus(ctx, []string{"f1"}, true)
	assert.Error(t, err)
	_, _, err = m.BatchDeleteFunctions(ctx, []string{"f1"})
	assert.Error(t, err)
	// BatchCopyFunctions records per-item failures instead of returning an error.
	count, failed, copied, err := m.BatchCopyFunctions(ctx, []string{"f1"})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Equal(t, []string{"f1"}, failed)
	assert.Empty(t, copied)
	assert.Error(t, m.ReplacePermissions(ctx, "f1", nil))
	_, err = m.ListPermissions(ctx, "f1")
	assert.Error(t, err)
	_, err = m.ListDescriptorTemplates(ctx, "cat")
	assert.Error(t, err)
	_, err = m.FindByFunctionID(ctx, "f1")
	assert.Error(t, err)
}

func TestErrorPaths_TaskAndNodeModels(t *testing.T) {
	db := newClosedDB(t)
	ctx := context.Background()

	rm := NewTaskRunModel(db)
	em := NewTaskEventModel(db)
	nm := NewNodeModel(db)

	assert.Error(t, rm.Create(ctx, &TaskRun{TaskID: "t"}))
	_, err := rm.FindByTaskID(ctx, "t")
	assert.Error(t, err)
	_, _, err = rm.List(ctx, ListTasksOptions{})
	assert.Error(t, err)
	ok, err := rm.UpdateByTaskIDIfStatusNotIn(ctx, "t", nil, nil)
	assert.Error(t, err)
	assert.False(t, ok)
	assert.Error(t, em.Append(ctx, &TaskEvent{TaskID: "t"}))
	_, err = em.ListByTaskID(ctx, "t", 0)
	assert.Error(t, err)
	_, err = em.NextSeq(ctx, "t")
	assert.Error(t, err)

	_, err = nm.List(ctx, ListNodesOptions{})
	assert.Error(t, err)
	_, err = nm.ListCommands(ctx)
	assert.Error(t, err)
	_, err = nm.FindByNodeID(ctx, "n1")
	assert.Error(t, err)
	assert.Error(t, nm.UpdateMeta(ctx, "n1", nil))
	assert.Error(t, nm.UpdateStatus(ctx, "n1", "down"))
}

func TestErrorPaths_TermConfigRateLimitProfileModels(t *testing.T) {
	db := newClosedDB(t)
	ctx := context.Background()

	tm := NewTermDictionaryModel(db)
	cm := NewConfigVersionModel(db)
	rlm := NewRateLimitModel(db)
	pm := NewProfileModel(db)
	rm := NewRoleModel(db)
	pmM := NewPermissionModel(db)

	_, err := tm.List(ctx, "")
	assert.Error(t, err)
	_, err = tm.AliasMap(ctx)
	assert.Error(t, err)
	assert.Error(t, tm.DeleteByAlias(ctx, "resource", "a"))
	assert.Error(t, tm.Upsert(ctx, &TermDictionary{Domain: "resource", TermKey: "k", Alias: "a"}))

	_, err = cm.List(ctx, "k")
	assert.Error(t, err)
	_, err = cm.CreateWithMeta(ctx, ConfigVersionPayload{Key: "k"}, "u")
	assert.Error(t, err)
	_, err = cm.ListLatest(ctx, ConfigListOptions{})
	assert.Error(t, err)

	_, err = rlm.List(ctx, "api")
	assert.Error(t, err)
	assert.Error(t, rlm.Upsert(ctx, &RateLimit{RateLimitID: "r"}))

	assert.Error(t, pm.ReplacePermissions(ctx, 1, nil))
	_, err = pm.ListPermissions(ctx, 1)
	assert.Error(t, err)
	assert.Error(t, pm.ReplaceGames(ctx, 1, nil))
	_, err = pm.ListGames(ctx, 1)
	assert.Error(t, err)

	_, _, err = rm.List(ctx, ListRolesOptions{})
	assert.Error(t, err)
	_, err = rm.FindOne(ctx, 1)
	assert.Error(t, err)
	_, err = rm.GetRolePermissionIDs(ctx, 1)
	assert.Error(t, err)
	_, err = rm.GetRolesPermissionIDs(ctx, []uint{1})
	assert.Error(t, err)
	_, err = rm.ValidatePermissionIDs(ctx, []string{"p"})
	assert.Error(t, err)
	assert.Error(t, rm.ReplacePermissions(ctx, 1, []string{"p"}))

	_, _, err = pmM.List(ctx, ListPermissionsOptions{})
	assert.Error(t, err)
	_, err = pmM.FindOne(ctx, "player:view")
	assert.Error(t, err)
}

func TestErrorPaths_OpenAPIAndContractModels(t *testing.T) {
	db := newClosedDB(t)
	sm := NewOpenAPISourceModel(db)
	bm := NewOpenAPISourceBindingModel(db)
	cmm := NewFunctionContractModel(db)
	capm := NewResourceCapabilityModel(db)
	semm := NewCapabilitySemanticsModel(db)
	vm := NewCapabilitySemanticVersionModel(db)
	ppm := NewPageProposalModel(db)
	pvmm := NewPageProposalVersionModel(db)
	psm := NewPageSpecModel(db)
	pubm := NewPublishedPageSpecModel(db)
	vem := NewPageVersionModel(db)
	ctx := context.Background()

	_, err := sm.ListByScope(ctx, "demo", "prod")
	assert.Error(t, err)
	_, err = sm.FindByScopeAndSourceID(ctx, "demo", "prod", "s1")
	assert.Error(t, err)
	_, err = bm.ListBySource(ctx, "demo", "prod", "s1")
	assert.Error(t, err)
	_, err = bm.ListByScopeAndFunctionID(ctx, "demo", "prod", "f1")
	assert.Error(t, err)
	_, err = bm.FindByScopeSourceAndBindingID(ctx, "demo", "prod", "s1", "b1")
	assert.Error(t, err)
	assert.Error(t, bm.Upsert(ctx, &OpenAPISourceBinding{BindingID: "b1"}))
	assert.Error(t, bm.Delete(ctx, "demo", "prod", "s1", "b1"))

	assert.Error(t, cmm.UpsertContract(ctx, &FunctionContract{FunctionID: "f1"}))
	_, err = cmm.FindByScopeAndFunctionID(ctx, "g", "e", "f1")
	assert.Error(t, err)
	_, err = cmm.ListByScope(ctx, "g", "e")
	assert.Error(t, err)
	_, err = cmm.ListByResourceKey(ctx, "g", "e", "r")
	assert.Error(t, err)
	assert.Error(t, capm.UpsertCapability(ctx, &ResourceCapability{ResourceKey: "r"}))
	_, err = capm.FindByScopeAndResourceKey(ctx, "g", "e", "r")
	assert.Error(t, err)
	_, err = capm.ListByScope(ctx, "g", "e")
	assert.Error(t, err)
	assert.Error(t, semm.UpsertSemantics(ctx, &CapabilitySemantics{ResourceKey: "r"}))
	_, err = semm.FindByScopeAndResourceKey(ctx, "g", "e", "r")
	assert.Error(t, err)
	_, err = semm.ListByScope(ctx, "g", "e")
	assert.Error(t, err)
	_, err = vm.ListBySemanticsID(ctx, 1)
	assert.Error(t, err)
	_, _, err = vm.ListBySemanticsIDPaged(ctx, 1, 10, 0)
	assert.Error(t, err)

	assert.Error(t, ppm.UpsertProposal(ctx, &PageProposal{ProposalKey: "k"}))
	_, err = ppm.FindByScopeAndKey(ctx, "g", "e", "k")
	assert.Error(t, err)
	_, err = ppm.ListByScope(ctx, "g", "e")
	assert.Error(t, err)
	_, err = pvmm.LatestByProposalID(ctx, 1)
	assert.Error(t, err)
	_, err = pvmm.GetNextVersion(ctx, 1)
	assert.Error(t, err)

	assert.Error(t, psm.Upsert(ctx, &PageSpec{PageKey: "p"}))
	_, err = psm.ListByScope(ctx, "g", "e")
	assert.Error(t, err)
	assert.Error(t, pubm.Create(ctx, &PublishedPageSpec{PageKey: "p"}))
	_, err = pubm.ListLatestActiveByScope(ctx, "g", "e")
	assert.Error(t, err)
	assert.Error(t, vem.UpsertByScopePageKeyVersion(ctx, &PageVersion{PageKey: "p"}))
	_, err = vem.GetNextVersion(ctx, "g", "e", "p")
	assert.Error(t, err)
	_, _, err = vem.ListByScopeAndPageKeyPaged(ctx, "g", "e", "p", 10, 0)
	assert.Error(t, err)
}

func TestErrorPaths_AgentSessionRegistrationWarningAnalytics(t *testing.T) {
	db := newClosedDB(t)

	am := NewAgentSessionModel(db)
	wm := NewRegistrationWarningModel(db)
	bm := NewBehaviorModel(db)
	pym := NewPaymentsModel(db)
	rm := NewRetentionModel(db)
	ctx := context.Background()

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)

	assert.Error(t, am.Upsert(ctx, &registry.AgentSession{AgentID: "a"}))
	_, err := am.LoadActiveSessions(ctx)
	assert.Error(t, err)

	assert.Error(t, wm.Upsert(ctx, &registry.FunctionRegistrationWarning{Code: "dup"}))
	_, err = wm.List(ctx, WarningFilter{})
	assert.Error(t, err)
	_, err = wm.CountByStatus(ctx)
	assert.Error(t, err)

	assert.Error(t, bm.RecordEvent(ctx, &BehaviorEvent{EventType: "login"}))
	_, _, err = bm.ListEvents(ctx, BehaviorEventOptions{})
	assert.Error(t, err)
	assert.Error(t, bm.UpsertFeatureAdoption(ctx, &FeatureAdoption{Feature: "guild"}))
	_, err = bm.ListFeatureAdoptions(ctx, "", "")
	assert.Error(t, err)
	_, err = bm.CountDistinctUsers(ctx, "", "", start, end)
	assert.Error(t, err)
	_, err = bm.CountEvents(ctx, "", "", start, end)
	assert.Error(t, err)
	_, err = bm.EventTypeCounts(ctx, "", "", start, end, 5)
	assert.Error(t, err)
	_, err = bm.DailyActivity(ctx, "", "", start, end)
	assert.Error(t, err)

	assert.Error(t, pym.CreateTransaction(ctx, &PaymentTransaction{TransactionID: "tx"}))
	_, _, err = pym.ListTransactions(ctx, PaymentQueryOptions{})
	assert.Error(t, err)
	_, err = pym.AggregateRevenue(ctx, "", "", start, end)
	assert.Error(t, err)
	_, err = pym.DailyRevenue(ctx, "", "", start, end)
	assert.Error(t, err)
	_, err = pym.ListProductTrends(ctx, "", "")
	assert.Error(t, err)

	assert.Error(t, rm.UpsertCohort(ctx, &RetentionCohort{Cohort: "c"}))
	_, err = rm.ListCohorts(ctx, "", "", "")
	assert.Error(t, err)
}

func TestTermDictionary_AliasMapRejectsUnknownDomain(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewTermDictionaryModel(db)
	ctx := context.Background()

	require.NoError(t, db.Exec(
		`INSERT INTO term_dictionary (domain, term_key, alias, created_at, updated_at) VALUES ('bogus', 'k', 'a', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
	).Error)

	_, err := m.AliasMap(ctx)
	assert.Error(t, err)
}

func TestGameModel_BackfillRestoresSoftDeletedBinding(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewGameModel(db)
	ctx := context.Background()

	require.NoError(t, m.Create(ctx, &Game{Name: "G", GameID: "g1", AliasName: "g1"}))
	game, err := m.FindByGameIDString(ctx, "g1")
	require.NoError(t, err)
	require.NoError(t, game.SetEnvs([]GameEnv{{Env: "prod"}}))
	require.NoError(t, db.Save(game).Error)

	resolver := func(gameID, env string) string { return "db_" + gameID + "_" + env }
	created, err := m.BackfillEnvBindings(ctx, resolver)
	require.NoError(t, err)
	assert.Equal(t, 1, created)

	// Soft-delete the binding, then backfill again: the legacy JSON still
	// references it, so it must be restored while keeping its database name.
	binding, err := m.FindEnvBinding(ctx, "g1", "prod")
	require.NoError(t, err)
	require.NotNil(t, binding)
	require.NoError(t, db.Delete(binding).Error)
	require.NoError(t, db.Exec("UPDATE game_envs SET deleted_at = ? WHERE id = ?", time.Now(), binding.ID).Error)

	created, err = m.BackfillEnvBindings(ctx, resolver)
	require.NoError(t, err)
	assert.Equal(t, 1, created)

	restored, err := m.LookupDatabaseName(ctx, "g1", "prod")
	require.NoError(t, err)
	assert.Equal(t, "db_g1_prod", restored)
}
