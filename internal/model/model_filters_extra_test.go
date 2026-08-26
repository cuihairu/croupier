package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This file exercises optional-filter branches and secondary list helpers that
// the main CRUD tests leave uncovered.

// setupAllModelsDB creates an isolated in-memory sqlite database migrated
// with the full model list. A unique DSN avoids interference with the shared
// cache=shared database used by older tests in this package.
func setupAllModelsDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:filters_extra_%d.db?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, autoMigrateAllModels(db))
	require.NoError(t, db.AutoMigrate(&RegistrationWarningDB{}))
	return db
}

func TestAdminModel_ListFilters(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewAdminModel(db)
	ctx := context.Background()

	status := 1
	for _, name := range []string{"alice", "bob", "carol"} {
		require.NoError(t, m.Create(ctx, &Admin{Username: name, Nickname: name, Status: 1}, "pass"))
	}

	admins, _, err := m.List(ctx, ListAdminsOptions{Search: "ali"})
	require.NoError(t, err)
	require.Len(t, admins, 1)

	_, _, err = m.List(ctx, ListAdminsOptions{Role: "nonexistent-role"})
	assert.NoError(t, err)

	got, _, err := m.List(ctx, ListAdminsOptions{Status: &status})
	require.NoError(t, err)
	assert.Len(t, got, 3)

	// ValidatePassword wrong-password branch.
	_, err = m.ValidatePassword(ctx, "alice", "wrong")
	assert.Error(t, err)

	// GetLastScope on a missing admin returns the error path.
	_, err = m.GetLastScope(ctx, 9999)
	assert.Error(t, err)
}

func TestPlayerModel_ListFilters(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewPlayerModel(db)
	ctx := context.Background()

	status, level, vip := 1, 3, 0
	require.NoError(t, m.Create(ctx, &Player{Username: "p1", GameID: "demo"}, ""))
	require.NoError(t, m.Create(ctx, &Player{Username: "p2", GameID: "other"}, ""))

	players, total, err := m.List(ctx, ListPlayersOptions{GameID: "demo"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, players, 1)

	_, _, err = m.List(ctx, ListPlayersOptions{Search: "p", Status: &status, Level: &level, VIP: &vip})
	require.NoError(t, err)

	found, err := m.FindByUsername(ctx, "p2", "other")
	require.NoError(t, err)
	assert.Equal(t, "p2", found.Username)
}

func TestGameModel_ListFiltersAndBindings(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewGameModel(db)
	ctx := context.Background()

	require.NoError(t, m.Create(ctx, &Game{Name: "Demo Game", GameID: "demo", AliasName: "demo"}))
	require.NoError(t, m.Create(ctx, &Game{Name: "Other", GameID: "other", AliasName: "other", Status: "disabled"}))

	_, total, err := m.List(ctx, ListGamesOptions{Status: "disabled"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)

	_, total, err = m.List(ctx, ListGamesOptions{Search: "demo"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)

	exists, err := m.ExistsByNameIgnoreCase(ctx, "DEMO GAME")
	require.NoError(t, err)
	assert.True(t, exists)

	game, err := m.FindOne(ctx, 1)
	require.NoError(t, err)
	exists, err = m.ExistsByNameIgnoreCase(ctx, "demo game", game.ID)
	require.NoError(t, err)
	assert.False(t, exists)

	// upsertEnvBinding restore path: soft-deleted binding gets restored.
	require.NoError(t, m.AddEnvBinding(ctx, "demo", "prod", "game_demo_prod", "d", "red"))
	binding, err := m.FindEnvBinding(ctx, "demo", "prod")
	require.NoError(t, err)
	require.NotNil(t, binding)
	require.NoError(t, db.Unscoped().Model(&GameEnvBinding{}).Where("id = ?", binding.ID).Update("deleted_at", time.Now()).Error)
	require.NoError(t, m.AddEnvBinding(ctx, "demo", "prod", "game_demo_prod2", "d2", "blue"))
	restored, err := m.FindEnvBinding(ctx, "demo", "prod")
	require.NoError(t, err)
	require.NotNil(t, restored)
	assert.Equal(t, "game_demo_prod2", restored.DatabaseName)

	// UpdateEnvsAndBindings rejects bindings without a database name.
	game.SetEnvs([]GameEnv{{Env: "prod"}})
	err = m.UpdateEnvsAndBindings(ctx, "demo", game.ID, datatypes.JSON(`[{"env":"prod"}]`), nil,
		[]GameEnvBinding{{Env: "", DatabaseName: "x"}})
	assert.Error(t, err)

	err = m.UpdateEnvsAndBindings(ctx, "demo", game.ID, datatypes.JSON(`[{"env":"prod"}]`), []string{"", "old"},
		[]GameEnvBinding{{Env: "prod", DatabaseName: "game_demo_prod"}})
	assert.NoError(t, err)
}

func TestGameModel_BackfillEnvBindings_Branches(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewGameModel(db)
	ctx := context.Background()

	// Nil resolver is rejected.
	_, err := m.BackfillEnvBindings(ctx, nil)
	assert.Error(t, err)

	// Empty database names are rejected.
	require.NoError(t, m.Create(ctx, &Game{Name: "G", GameID: "g1", AliasName: "g1"}))
	game, _ := m.FindByGameIDString(ctx, "g1")
	game.Envs = datatypes.JSON(`[{"env":"prod","description":"d","color":"c"}]`)
	require.NoError(t, db.Save(game).Error)
	_, err = m.BackfillEnvBindings(ctx, func(gameID, env string) string { return "" })
	assert.Error(t, err)

	// Happy path creates the binding; rerun is idempotent.
	resolver := func(gameID, env string) string { return "db_" + gameID + "_" + env }
	created, err := m.BackfillEnvBindings(ctx, resolver)
	require.NoError(t, err)
	assert.Equal(t, 1, created)
	created, err = m.BackfillEnvBindings(ctx, resolver)
	require.NoError(t, err)
	assert.Equal(t, 0, created)

	// Invalid envs JSON surfaces an error.
	require.NoError(t, m.Create(ctx, &Game{Name: "Bad", GameID: "bad", AliasName: "bad"}))
	badGame, _ := m.FindByGameIDString(ctx, "bad")
	badGame.Envs = datatypes.JSON(`not-json`)
	require.NoError(t, db.Save(badGame).Error)
	_, err = m.BackfillEnvBindings(ctx, resolver)
	assert.Error(t, err)
}

func TestTicketModel_ListFilters(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewTicketModel(db)
	ctx := context.Background()

	require.NoError(t, m.Create(ctx, &Ticket{
		Title: "help me", Content: "cannot login", Category: "account", Priority: "high",
		Status: dbenum.TicketStatusOpen, Assignee: "op1", PlayerID: "player-7", GameID: "demo", Env: "prod",
	}))

	tickets, total, err := m.List(ctx, TicketQueryOptions{
		Query: "login", Status: dbenum.TicketStatusOpen, Category: "account", Priority: "high",
		Assignee: "op1", GameID: "demo", Env: "prod",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, tickets, 1)

	comments := []TicketComment{{TicketID: tickets[0].ID, Author: "op", Content: "ok"}}
	for i := range comments {
		require.NoError(t, m.CreateComment(ctx, &comments[i]))
	}
	listed, err := m.ListComments(ctx, tickets[0].ID)
	require.NoError(t, err)
	assert.Len(t, listed, 1)
}

func TestAlertModel_FiltersAndBootstrap(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewAlertModel(db)
	ctx := context.Background()

	alerts, total, err := m.List(ctx, ListAlertsOptions{Level: "critical", Status: "firing", Source: "monitor"})
	require.NoError(t, err)
	assert.Empty(t, alerts)
	assert.Equal(t, int64(0), total)

	require.NoError(t, m.Create(ctx, &Alert{AlertID: "a1", Level: "critical", Status: "firing", Source: "monitor"}))

	// BootstrapAlerts skips empty IDs and already-existing alerts.
	require.NoError(t, m.BootstrapAlerts(ctx, []Alert{
		{AlertID: "", Message: "ignored"},
		{AlertID: "a1"},
		{AlertID: "a2", Level: "critical"},
	}))
	found, err := m.FindByAlertID(ctx, "a2")
	require.NoError(t, err)
	require.NotNil(t, found)

	_, err = m.FindByAlertID(ctx, "")
	assert.Error(t, err)
	_, err = m.FindByAlertID(ctx, "missing")
	assert.Error(t, err)

	byIDs, err := m.FindByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, byIDs)
	byIDs, err = m.FindByIDs(ctx, []uint{found.ID})
	require.NoError(t, err)
	assert.Len(t, byIDs, 1)

	// Silences with ActiveOnly filter.
	require.NoError(t, m.CreateSilence(ctx, &AlertSilence{AlertID: found.ID, Reason: "r", DurationMinute: 60}))
	silences, err := m.ListSilences(ctx, ListSilencesOptions{ActiveOnly: true})
	require.NoError(t, err)
	assert.Len(t, silences, 1)
	require.NoError(t, m.PruneExpiredSilences(ctx))
}

func TestFeedbackModel_ListAndStatsFilters(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewFeedbackModel(db)
	ctx := context.Background()

	require.NoError(t, m.Create(ctx, &Feedback{
		PlayerID: "p1", Contact: "c1", Content: "great game", Category: "praise",
		Priority: "low", Status: dbenum.FeedbackStatusOpen, Rating: 5, Reply: "thanks", GameID: "demo", Env: "prod",
	}))
	require.NoError(t, m.Create(ctx, &Feedback{PlayerID: "p2", Content: "bug", Category: "bug", Status: dbenum.FeedbackStatusClosed, Rating: 2}))

	items, total, err := m.List(ctx, ListFeedbackOptions{
		GameID: "demo", Env: "prod", Status: dbenum.FeedbackStatusOpen, ExcludeStatus: -1, Category: "praise", Keyword: "great",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)

	stats, err := m.Stats(ctx, FeedbackStatsOptions{GameID: "demo", Days: 7})
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.Total)
	assert.Equal(t, int64(1), stats.ByCategory["praise"])
	assert.Equal(t, int64(1), stats.Responded)
	assert.InDelta(t, 5.0, stats.AvgRating, 0.001)

	// Stats with no rows leaves AvgRating at zero (sql.NullFloat64 invalid).
	require.NoError(t, db.Where("1 = 1").Delete(&Feedback{}).Error)
	emptyStats, err := m.Stats(ctx, FeedbackStatsOptions{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), emptyStats.Total)
	assert.InDelta(t, 0.0, emptyStats.AvgRating, 0.001)
}

func TestMessageModel_FiltersAndEncodeData(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewMessageModel(db)
	ctx := context.Background()

	payload, err := EncodeData(map[string]string{"k": "v"})
	require.NoError(t, err)
	require.NotEmpty(t, payload)

	nullPayload, err := EncodeData(nil)
	require.NoError(t, err)
	assert.Equal(t, datatypes.JSON([]byte("null")), nullPayload)

	_, err = EncodeData(make(chan int))
	assert.Error(t, err)

	require.NoError(t, m.Create(ctx, &Message{To: "admin", Type: "system", Title: "hi", Data: payload}))

	msgs, total, err := m.List(ctx, ListMessagesOptions{Type: "system", Status: dbenum.MessageStatusUnread, To: "admin"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, msgs, 1)

	recent, err := m.Recent(ctx, 5, "admin")
	require.NoError(t, err)
	assert.Len(t, recent, 1)

	count, err := m.CountUnread(ctx, "admin")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	require.NoError(t, m.MarkRead(ctx, msgs[0].ID))
	count, err = m.CountUnread(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCertificateModel_ListAndAlerts(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewCertificateModel(db)
	ctx := context.Background()

	now := time.Now()
	require.NoError(t, m.Create(ctx, &Certificate{Domain: "a.example.com", ExpiresAt: now.Add(10 * 24 * time.Hour), Status: "expiring"}))
	require.NoError(t, m.Create(ctx, &Certificate{Domain: "b.example.com", ExpiresAt: now.Add(90 * 24 * time.Hour), Status: "active"}))
	require.NoError(t, m.AddAlert(ctx, &CertificateAlert{Domain: "a.example.com", ThresholdDays: 30, Active: true}))

	certs, total, err := m.List(ctx, ListCertificatesOptions{Page: 0, PageSize: 0, Status: "active"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, certs, 1)

	all, err := m.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	expiring, err := m.ExpiringWithin(ctx, 30*24*time.Hour)
	require.NoError(t, err)
	assert.Len(t, expiring, 1)

	stats, err := m.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats["total"])

	alerts, alertTotal, err := m.ListAlerts(ctx, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), alertTotal)
	assert.Len(t, alerts, 1)

	one, err := m.FindByDomain(ctx, "a.example.com")
	require.NoError(t, err)
	require.NotNil(t, one)
}

func TestFAQModel_FiltersAndCategories(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewFAQModel(db)
	ctx := context.Background()

	visibleFlag := true
	require.NoError(t, m.Create(ctx, &FAQ{Question: "how to play?", Answer: "click", Category: "basics", Visible: true}))
	require.NoError(t, m.UpsertCategory(ctx, &FAQCategory{Name: "basics", Description: "Basics", Visible: true}))

	faqs, total, err := m.List(ctx, ListFAQOptions{Category: "basics", Keyword: "play", Visible: &visibleFlag})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, faqs, 1)

	cats, err := m.ListCategories(ctx)
	require.NoError(t, err)
	require.Len(t, cats, 1)
	assert.Equal(t, 1, cats[0].Count)
}

func TestPermissionAndRoleModel_ListFilters(t *testing.T) {
	db := setupAllModelsDB(t)
	pm := NewPermissionModel(db)
	rm := NewRoleModel(db)
	ctx := context.Background()

	require.NoError(t, db.Create(&Permission{ID: "player:view", Name: "view players", Resource: "player", Action: "view", Category: "players"}).Error)
	perms, total, err := pm.List(ctx, ListPermissionsOptions{Category: "players", Resource: "player"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, perms, 1)

	require.NoError(t, rm.Create(ctx, &Role{Name: "ops", Description: "operators", Category: "system"}))
	roles, roleTotal, err := rm.List(ctx, ListRolesOptions{Category: "system", Search: "ops"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), roleTotal)
	require.Len(t, roles, 1)

	require.NoError(t, rm.ReplacePermissions(ctx, roles[0].ID, []string{"player:view"}))
	ids, err := rm.GetRolePermissionIDs(ctx, roles[0].ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"player:view"}, ids)

	permMap, err := rm.GetRolesPermissionIDs(ctx, []uint{roles[0].ID})
	require.NoError(t, err)
	assert.Contains(t, permMap[roles[0].ID], "player:view")

	valid, err := rm.ValidatePermissionIDs(ctx, []string{"player:view"})
	require.NoError(t, err)
	assert.Equal(t, []string{"player:view"}, valid)
}

func TestFunctionModel_ListFiltersAndBatchOps(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewFunctionModel(db)
	ctx := context.Background()

	fn := &Function{FunctionID: "f1", Name: "kick player", Resource: "player", GameID: "demo", Status: 1}
	require.NoError(t, m.Create(ctx, fn))
	searchPtr := 1

	functions, total, err := m.List(ctx, ListFunctionsOptions{GameID: "demo", Resource: "player", Status: &searchPtr, Search: "kick"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, functions, 1)

	require.NoError(t, m.ReplacePermissions(ctx, "f1", []FunctionPermission{{Resource: "player", Actions: datatypes.JSON(`["ban"]`)}}))
	perms, err := m.ListPermissions(ctx, "f1")
	require.NoError(t, err)
	assert.Len(t, perms, 1)

	templates, err := m.ListDescriptorTemplates(ctx, "combat")
	require.NoError(t, err)
	assert.Empty(t, templates)

	copiedID, err := m.CopyFunction(ctx, "f1")
	require.NoError(t, err)
	assert.NotEmpty(t, copiedID)

	// Flip status from the created value so RowsAffected reflects the change.
	updatedCount, _, err := m.BatchUpdateStatus(ctx, []string{"f1"}, false)
	require.NoError(t, err)
	assert.Equal(t, 1, updatedCount)

	deletedCount, _, err := m.BatchDeleteFunctions(ctx, []string{copiedID})
	require.NoError(t, err)
	assert.Equal(t, 1, deletedCount)

	// Copy a distinct function to avoid same-second unique-ID collisions.
	require.NoError(t, m.Create(ctx, &Function{FunctionID: "f2", Name: "second", Resource: "item", GameID: "demo"}))
	batchCopiedCount, _, batchCopiedIDs, err := m.BatchCopyFunctions(ctx, []string{"f2"})
	require.NoError(t, err)
	assert.Equal(t, 1, batchCopiedCount)
	assert.NotEmpty(t, batchCopiedIDs)
}

func TestTaskModels_Branches(t *testing.T) {
	db := setupAllModelsDB(t)
	rm := NewTaskRunModel(db)
	em := NewTaskEventModel(db)
	ctx := context.Background()

	task := &TaskRun{TaskID: "t-1", FunctionID: "f", GameID: "demo", Env: "prod", Status: "running"}
	require.NoError(t, rm.Create(ctx, task))

	updated, err := rm.UpdateByTaskIDIfStatusNotIn(ctx, "t-1", []string{"done", "failed"}, map[string]interface{}{"progress": 50})
	require.NoError(t, err)
	assert.True(t, updated)

	// Blocked status wins: no rows are affected.
	updated, err = rm.UpdateByTaskIDIfStatusNotIn(ctx, "t-1", []string{"running"}, map[string]interface{}{"status": "done"})
	require.NoError(t, err)
	assert.False(t, updated)

	_, err = rm.UpdateByTaskIDIfStatusNotIn(ctx, "", nil, nil)
	assert.Error(t, err)

	items, total, err := rm.List(ctx, ListTasksOptions{FunctionID: "f", Status: "running", GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)

	// TaskEvent Append fills zero timestamps.
	event := &TaskEvent{TaskID: "t-1", Seq: 1, Type: "log", Message: "started"}
	require.NoError(t, em.Append(ctx, event))
	assert.False(t, event.CreatedAt.IsZero())

	events, err := em.ListByTaskID(ctx, "t-1", 0)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	events, err = em.ListByTaskID(ctx, "t-1", 1)
	require.NoError(t, err)
	assert.Empty(t, events)
	events, err = em.ListByTaskID(ctx, "   ", 0)
	require.NoError(t, err)
	assert.Empty(t, events)

	seq, err := em.NextSeq(ctx, "t-1")
	require.NoError(t, err)
	assert.Equal(t, int64(2), seq)
	seq, err = em.NextSeq(ctx, "t-missing")
	require.NoError(t, err)
	assert.Equal(t, int64(1), seq)
	seq, err = em.NextSeq(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), seq)
}

func TestSupportBackupNodeRateLimitLists(t *testing.T) {
	db := setupAllModelsDB(t)
	sm := NewSupportModel(db)
	bm := NewBackupModel(db)
	nm := NewNodeModel(db)
	rlm := NewRateLimitModel(db)
	ctx := context.Background()

	require.NoError(t, sm.CreateTicket(ctx, &SupportTicket{Title: "issue", Status: "open"}))
	tickets, ticketTotal, err := sm.ListTickets(ctx, ListTicketsOptions{Status: "open"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), ticketTotal)
	assert.Len(t, tickets, 1)

	require.NoError(t, bm.Create(ctx, &Backup{BackupID: "b1", Name: "nightly", Type: "full", Status: "done"}))
	backups, backupTotal, err := bm.List(ctx, ListBackupsOptions{Type: "full"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), backupTotal)
	assert.Len(t, backups, 1)

	require.NoError(t, nm.Upsert(ctx, &Node{NodeID: "n1", Name: "node1", Type: "agent", Status: "online"}))
	node, err := nm.FindByNodeID(ctx, "n1")
	require.NoError(t, err)
	require.NotNil(t, node)
	_, err = nm.FindByNodeID(ctx, "missing")
	assert.Error(t, err)

	nodes, err := nm.List(ctx, ListNodesOptions{Type: "agent", Status: "online"})
	require.NoError(t, err)
	assert.Len(t, nodes, 1)

	commands, err := nm.ListCommands(ctx)
	require.NoError(t, err)
	assert.Empty(t, commands)
	require.NoError(t, nm.UpsertCommand(ctx, &NodeCommand{Name: "restart", Description: "restart agent"}))
	commands, err = nm.ListCommands(ctx)
	require.NoError(t, err)
	assert.Len(t, commands, 1)

	require.NoError(t, rlm.Upsert(ctx, &RateLimit{RateLimitID: "rl1", Name: "api", Resource: "api", Limit: 100, Window: 60}))
	rules, err := rlm.List(ctx, "api")
	require.NoError(t, err)
	assert.Len(t, rules, 1)
	require.NoError(t, rlm.DeleteByKey(ctx, "rl1"))
	rules, err = rlm.List(ctx, "api")
	require.NoError(t, err)
	assert.Empty(t, rules)
}

func TestTermDictionaryModel_Branches(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewTermDictionaryModel(db)
	ctx := context.Background()

	item := &TermDictionary{Domain: "resource", TermKey: "Player", Alias: "User", Display: map[string]string{"zh-CN": "玩家", "en-US": "player"}, SortOrder: 1}
	require.NoError(t, m.Upsert(ctx, item))
	// Upserting the same alias updates the existing row.
	item.Display = map[string]string{"zh-CN": "用户"}
	require.NoError(t, m.Upsert(ctx, item))

	// Missing fields are ignored silently.
	require.NoError(t, m.Upsert(ctx, nil))
	require.NoError(t, m.Upsert(ctx, &TermDictionary{Domain: "resource"}))
	require.NoError(t, m.Upsert(ctx, &TermDictionary{Domain: "resource", TermKey: "k", Alias: ""}))

	filtered, err := m.List(ctx, "resource")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	_, err = m.List(ctx, "bogus-domain")
	assert.Error(t, err)

	aliasMap, err := m.AliasMap(ctx)
	require.NoError(t, err)
	assert.Equal(t, "player", aliasMap["resource"]["user"])

	require.NoError(t, m.DeleteByAlias(ctx, "resource", "user"))
	aliasMap, err = m.AliasMap(ctx)
	require.NoError(t, err)
	assert.Empty(t, aliasMap["resource"])
	assert.Error(t, m.DeleteByAlias(ctx, "bogus", "x"))
}

func TestConfigVersionModel_Branches(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewConfigVersionModel(db)
	ctx := context.Background()

	versions, err := m.List(ctx, "   ")
	require.NoError(t, err)
	assert.Empty(t, versions)

	first, err := m.CreateWithMeta(ctx, ConfigVersionPayload{Key: "cfg", Content: "v1"}, "tester")
	require.NoError(t, err)
	assert.Equal(t, 1, first.Version)

	// Optimistic-lock conflict.
	_, err = m.CreateWithMeta(ctx, ConfigVersionPayload{Key: "cfg", Content: "v2", BaseVersion: first.Version + 5}, "tester")
	assert.Error(t, err)

	second, err := m.CreateWithMeta(ctx, ConfigVersionPayload{Key: "cfg", Content: "v2", BaseVersion: first.Version}, "tester")
	require.NoError(t, err)
	assert.Equal(t, 2, second.Version)

	_, err = m.CreateWithMeta(ctx, ConfigVersionPayload{Key: "", Content: "v"}, "tester")
	assert.Error(t, err)

	// BaseVersion set but no prior record exists.
	_, err = m.CreateWithMeta(ctx, ConfigVersionPayload{Key: "fresh", Content: "x", BaseVersion: 3}, "tester")
	assert.Error(t, err)

	listed, err := m.List(ctx, "cfg")
	require.NoError(t, err)
	assert.Len(t, listed, 2)

	latest, err := m.ListLatest(ctx, ConfigListOptions{GameID: "", Env: "", Format: "", IDLike: "cfg"})
	require.NoError(t, err)
	require.Len(t, latest, 1)
	assert.Equal(t, 2, latest[0].Version)

	scoped, err := m.ListLatest(ctx, ConfigListOptions{GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.Empty(t, scoped)
}

func TestOpenAPISourceModel_Methods(t *testing.T) {
	db := setupAllModelsDB(t)
	sm := NewOpenAPISourceModel(db)
	bm := NewOpenAPISourceBindingModel(db)
	ctx := context.Background()

	src := &OpenAPISource{GameID: "demo", Env: "prod", SourceID: "src1", Name: "catalog", Format: "json", OpenAPIVersion: "3.0.3", ContentHash: "abc"}
	src.SetSpec([]byte(`{"openapi":"3.0.3"}`))
	require.NoError(t, src.SetOperations([]map[string]string{{"operationId": "getUser"}}))
	require.NoError(t, src.SetDiagnostics([]string{"warn"}))
	require.NoError(t, sm.Create(ctx, src))

	var ops []map[string]string
	require.NoError(t, src.GetOperations(&ops))
	require.Len(t, ops, 1)
	var diags []string
	require.NoError(t, src.GetDiagnostics(&diags))
	require.Len(t, diags, 1)

	listed, err := sm.ListByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	require.Len(t, listed, 1)

	found, err := sm.FindByScopeAndSourceID(ctx, "demo", "prod", "src1")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "src1", found.SourceID)

	binding := &OpenAPISourceBinding{GameID: "demo", Env: "prod", SourceID: "src1", BindingID: "bind1", OperationID: "getUser", Kind: "rest", FunctionID: "f1"}
	require.NoError(t, bm.Upsert(ctx, binding))

	bySource, err := bm.ListBySource(ctx, "demo", "prod", "src1")
	require.NoError(t, err)
	assert.Len(t, bySource, 1)

	byFn, err := bm.ListByScopeAndFunctionID(ctx, "demo", "prod", "f1")
	require.NoError(t, err)
	assert.Len(t, byFn, 1)

	got, err := bm.FindByScopeSourceAndBindingID(ctx, "demo", "prod", "src1", "bind1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NoError(t, bm.Delete(ctx, "demo", "prod", "src1", "bind1"))
	bySource, err = bm.ListBySource(ctx, "demo", "prod", "src1")
	require.NoError(t, err)
	assert.Empty(t, bySource)
}

func TestAnalyticsModels_FilterBranches(t *testing.T) {
	db := setupAllModelsDB(t)
	bm := NewBehaviorModel(db)
	pm := NewPaymentsModel(db)
	rm := NewRetentionModel(db)
	ctx := context.Background()

	start := time.Now().Add(-24 * time.Hour)
	end := time.Now().Add(24 * time.Hour)

	require.NoError(t, bm.RecordEvent(ctx, &BehaviorEvent{GameID: "demo", Env: "prod", ServerID: "s1", EventType: "login", UserID: "u1"}))
	events, total, err := bm.ListEvents(ctx, BehaviorEventOptions{
		GameID: "demo", Env: "prod", ServerID: "s1", EventType: "login",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, events, 1)

	require.NoError(t, bm.UpsertFeatureAdoption(ctx, &FeatureAdoption{GameID: "demo", Env: "prod", Feature: "guild", Users: 10}))
	require.NoError(t, bm.UpsertFeatureAdoption(ctx, &FeatureAdoption{GameID: "demo", Env: "prod", Feature: "mail", Users: 20}))
	adoptions, err := bm.ListFeatureAdoptions(ctx, "demo", "prod")
	require.NoError(t, err)
	assert.Len(t, adoptions, 2)

	users, err := bm.CountDistinctUsers(ctx, "demo", "prod", start, end)
	require.NoError(t, err)
	assert.Equal(t, int64(1), users)

	count, err := bm.CountEvents(ctx, "demo", "prod", start, end)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	typeCounts, err := bm.EventTypeCounts(ctx, "demo", "prod", start, end, 10)
	require.NoError(t, err)
	require.Len(t, typeCounts, 1)

	daily, err := bm.DailyActivity(ctx, "demo", "prod", start, end)
	require.NoError(t, err)
	assert.Len(t, daily, 1)

	require.NoError(t, pm.CreateTransaction(ctx, &PaymentTransaction{
		GameID: "demo", Env: "prod", ServerID: "s1", TransactionID: "tx1", UserID: "u1", ProductID: "p1",
		ProductName: "gem", Amount: 9.99, Currency: "USD", Status: "success",
	}))
	txs, txTotal, err := pm.ListTransactions(ctx, PaymentQueryOptions{GameID: "demo", Env: "prod", ServerID: "s1", Status: "success"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), txTotal)
	assert.Len(t, txs, 1)

	revenue, err := pm.AggregateRevenue(ctx, "demo", "prod", start, end)
	require.NoError(t, err)
	assert.InDelta(t, 9.99, revenue.Revenue, 0.001)

	dailyRevenue, err := pm.DailyRevenue(ctx, "demo", "prod", start, end)
	require.NoError(t, err)
	assert.Len(t, dailyRevenue, 1)

	require.NoError(t, pm.UpsertProductTrend(ctx, &ProductTrend{GameID: "demo", Env: "prod", ProductID: "p1", Revenue: 99, Sales: 3}))
	trends, err := pm.ListProductTrends(ctx, "demo", "prod")
	require.NoError(t, err)
	assert.Len(t, trends, 1)

	require.NoError(t, rm.UpsertCohort(ctx, &RetentionCohort{GameID: "demo", Env: "prod", Cohort: "2026w34", Users: 42}))
	cohorts, err := rm.ListCohorts(ctx, "demo", "prod", "2026w34")
	require.NoError(t, err)
	assert.Len(t, cohorts, 1)
}

func TestProfileModel_ReplaceMethods(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewProfileModel(db)
	ctx := context.Background()

	require.NoError(t, m.ReplacePermissions(ctx, 1, []ProfilePermission{
		{Resource: "player", Actions: datatypes.JSON(`["view","edit"]`)},
	}))
	perms, err := m.ListPermissions(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, perms, 1)

	require.NoError(t, m.ReplaceGames(ctx, 1, []ProfileGame{
		{GameID: "demo", GameName: "Demo", Color: "#fff"},
	}))
	games, err := m.ListGames(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, games, 1)
}

func TestAgentSessionModel_UpsertAndLoad(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewAgentSessionModel(db)
	ctx := context.Background()

	session := &registry.AgentSession{
		AgentID: "agent-1", GameID: "demo", Env: "prod", Version: "1.0", Region: "cn", Zone: "z1",
		ExpireAt: time.Now().Add(time.Hour), LastSeen: time.Now(),
		Labels:    map[string]string{"tier": "gold"},
		Providers: []registry.ProviderSession{{ProviderID: "p1", GameID: "demo", Env: "prod"}},
	}
	require.NoError(t, m.Upsert(ctx, session))
	// Second upsert hits the ON CONFLICT update path.
	session.Region = "us"
	require.NoError(t, m.Upsert(ctx, session))

	loaded, err := m.LoadActiveSessions(ctx)
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "us", loaded[0].Region)
	assert.Equal(t, "gold", loaded[0].Labels["tier"])

	// A session whose Labels column contains invalid JSON is skipped silently.
	expireAt := time.Now().Add(time.Hour).Format("2006-01-02 15:04:05")
	require.NoError(t, db.Exec(
		`INSERT INTO agent_sessions (agent_id, expire_at, last_seen, labels, created_at, updated_at) VALUES ('broken', ?, ?, 'not-json', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		expireAt, expireAt,
	).Error)
	loaded, err = m.LoadActiveSessions(ctx)
	require.NoError(t, err)
	assert.Len(t, loaded, 1)

	affected, err := m.DeleteExpired(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, affected, int64(0))
}

func TestRegistrationWarningModel_ListAndCount(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewRegistrationWarningModel(db)
	ctx := context.Background()

	warn := &registry.FunctionRegistrationWarning{
		GameID: "demo", Env: "prod", AgentID: "a1", FunctionID: "f1", Code: "dup", Message: "duplicate",
	}
	require.NoError(t, m.Upsert(ctx, warn))

	listed, err := m.List(ctx, WarningFilter{GameID: "demo", Env: "prod", AgentID: "a1", FunctionID: "f1", Code: "dup", Status: "pending", Limit: 10})
	require.NoError(t, err)
	require.Len(t, listed, 1)

	counts, err := m.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), counts["pending"])
}

func TestContractRelatedModels_ScopeLists(t *testing.T) {
	db := setupAllModelsDB(t)
	cm := NewFunctionContractModel(db)
	capm := NewResourceCapabilityModel(db)
	semm := NewCapabilitySemanticsModel(db)
	vm := NewCapabilitySemanticVersionModel(db)
	ppm := NewPageProposalModel(db)
	pvm := NewPageProposalVersionModel(db)
	psm := NewPageSpecModel(db)
	pubm := NewPublishedPageSpecModel(db)
	ctx := context.Background()

	contract := &FunctionContract{GameID: "demo", Env: "prod", FunctionID: "f1", ResourceKey: "player", Capability: dbenum.CapabilityCollectionQuery}
	require.NoError(t, cm.UpsertContract(ctx, contract))

	byScope, err := cm.ListByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	require.Len(t, byScope, 1)

	byResource, err := cm.ListByResourceKey(ctx, "demo", "prod", "player")
	require.NoError(t, err)
	require.Len(t, byResource, 1)

	capability := &ResourceCapability{GameID: "demo", Env: "prod", ResourceKey: "player"}
	require.NoError(t, capm.UpsertCapability(ctx, capability))
	caps, err := capm.ListByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	assert.Len(t, caps, 1)

	semantics := &CapabilitySemantics{GameID: "demo", Env: "prod", ResourceKey: "player", CollectionQueryID: contract.ID}
	require.NoError(t, semm.UpsertSemantics(ctx, semantics))
	sems, err := semm.ListByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	require.Len(t, sems, 1)

	version := &CapabilitySemanticVersion{SemanticsID: semantics.ID, Version: semantics.Version, CreatedBy: "test"}
	require.NoError(t, vm.CreateVersion(ctx, version))
	vers, err := vm.ListBySemanticsID(ctx, semantics.ID)
	require.NoError(t, err)
	assert.Len(t, vers, 1)
	pagedVers, pagedTotal, err := vm.ListBySemanticsIDPaged(ctx, semantics.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), pagedTotal)
	assert.Len(t, pagedVers, 1)

	proposal := &PageProposal{GameID: "demo", Env: "prod", ProposalKey: "resource:player", PageKey: "player-page", Status: dbenum.ProposalStatusPending, ResourceKey: "player"}
	require.NoError(t, ppm.UpsertProposal(ctx, proposal))

	proposals, err := ppm.ListByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	assert.Len(t, proposals, 1)
	byStatus, err := ppm.ListByStatus(ctx, "demo", "prod", dbenum.ProposalStatusPending)
	require.NoError(t, err)
	assert.Len(t, byStatus, 1)
	byRes, err := ppm.ListByScopeAndResourceKey(ctx, "demo", "prod", "player")
	require.NoError(t, err)
	assert.Len(t, byRes, 1)
	byBoth, err := ppm.ListByScopeStatusAndResourceKey(ctx, "demo", "prod", dbenum.ProposalStatusPending, "player")
	require.NoError(t, err)
	assert.Len(t, byBoth, 1)

	require.NoError(t, pvm.CreateVersion(ctx, &PageProposalVersion{ProposalID: proposal.ID, Version: 1, CreatedBy: "test"}))
	proposalVersions, err := pvm.ListByProposalID(ctx, proposal.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, proposalVersions)
	foundVersion, err := pvm.FindByProposalIDAndVersion(ctx, proposal.ID, proposalVersions[0].Version)
	require.NoError(t, err)
	require.NotNil(t, foundVersion)
	latest, err := pvm.LatestByProposalID(ctx, proposal.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)

	pageSpec := &PageSpec{GameID: "demo", Env: "prod", PageKey: "player-page", Type: "resource", Status: "draft", DraftRevision: 1}
	require.NoError(t, pageSpec.SetTitle(map[string]string{"zh-CN": "玩家"}))
	require.NoError(t, pageSpec.SetCategoryLabels(map[string]string{"zh-CN": "玩家"}))
	pageSpec.SetSpec([]byte(`{"pageKey":"player-page"}`))
	require.NoError(t, psm.Upsert(ctx, pageSpec))

	specs, err := psm.ListByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	assert.Len(t, specs, 1)
	drafts, err := psm.ListByScopeAndStatus(ctx, "demo", "prod", "draft")
	require.NoError(t, err)
	assert.Len(t, drafts, 1)

	published := &PublishedPageSpec{
		GameID: "demo", Env: "prod", PageKey: "player-page", Version: 1,
		SpecJSON: `{"pageKey":"player-page"}`, RendererSchemaVersion: "page-spec:1", Active: true,
		PublishedAt: time.Now(), PublishedBy: "tester",
	}
	require.NoError(t, pubm.Create(ctx, published))
	actives, err := pubm.ListLatestActiveByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	assert.Len(t, actives, 1)
}

func TestPageProposalHelpers_UpsertAndFind(t *testing.T) {
	db := setupAllModelsDB(t)
	ctx := context.Background()

	m := NewPageProposalModel(db)
	proposal := &PageProposal{GameID: "demo", Env: "prod", ProposalKey: "operation:f1", PageKey: "f1-page", Status: dbenum.ProposalStatusPending}
	require.NoError(t, m.UpsertProposal(ctx, proposal))

	found, err := m.FindByScopeAndKey(ctx, "demo", "prod", "operation:f1")
	require.NoError(t, err)
	require.NotNil(t, found)

	_, err = m.FindByScopeAndPageKey(ctx, "demo", "prod", "f1-page")
	assert.NoError(t, err)
}

func TestBlockedIssueModel_ScopeQueries(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewBlockedProposalIssueModel(db)
	ctx := context.Background()

	issue := &BlockedProposalIssue{GameID: "demo", Env: "prod", ResourceKey: "player", FunctionID: "f1", Status: "open"}
	require.NoError(t, m.Upsert(ctx, issue))

	found, err := m.FindByScopeAndResourceKey(ctx, "demo", "prod", "player")
	require.NoError(t, err)
	require.NotNil(t, found)

	all, err := m.ListByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	assert.Len(t, all, 1)

	byRes, err := m.ListByScopeAndResourceKey(ctx, "demo", "prod", "player")
	require.NoError(t, err)
	assert.Len(t, byRes, 1)

	require.NoError(t, m.Resolve(ctx, "demo", "prod", "player", "f1", "tester"))
	require.NoError(t, m.UpdateStatus(ctx, issue.ID, "resolved", "tester"))
}

func TestGameSettersAndMigrationHelpers(t *testing.T) {
	game := &Game{}
	require.NoError(t, game.SetEnvs([]GameEnv{{Env: "prod", Description: "production", Color: "#ff0000"}}))
	envs, err := game.GetEnvs()
	require.NoError(t, err)
	require.Len(t, envs, 1)

	require.NoError(t, game.SetConfig(map[string]string{"region": "cn"}))
	var cfg map[string]string
	require.NoError(t, game.GetConfig(&cfg))
	assert.Equal(t, "cn", cfg["region"])
}
