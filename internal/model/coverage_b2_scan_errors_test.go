package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// corruptScanRow 注入 created_at 为非法时间的行：Count 走 count(*) 不受影响，
// Find 扫描 time.Time 失败，覆盖各 List 的 Find 错误分支。
func corruptScanRow(t *testing.T, table string, extraCols string, extraVals string) *gorm.DB {
	t.Helper()
	db := setupAllModelsDB(t)
	cols := "created_at"
	vals := "'not-a-time'"
	if extraCols != "" {
		cols += ", " + extraCols
		vals += ", " + extraVals
	}
	require.NoError(t, db.Exec("INSERT INTO "+table+" ("+cols+") VALUES ("+vals+")").Error, table)
	return db
}

func TestB2ListFindScanErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("admin", func(t *testing.T) {
		db := corruptScanRow(t, "admins", "username", "'b2u'")
		_, _, err := NewAdminModel(db).List(ctx, ListAdminsOptions{})
		assert.Error(t, err)
	})
	t.Run("alert", func(t *testing.T) {
		db := corruptScanRow(t, "alerts", "", "")
		_, _, err := NewAlertModel(db).List(ctx, ListAlertsOptions{})
		assert.Error(t, err)
	})
	t.Run("behavior", func(t *testing.T) {
		db := corruptScanRow(t, "behavior_events", "", "")
		_, _, err := NewBehaviorModel(db).ListEvents(ctx, BehaviorEventOptions{})
		assert.Error(t, err)
	})
	t.Run("payments", func(t *testing.T) {
		db := corruptScanRow(t, "payment_transactions", "", "")
		_, _, err := NewPaymentsModel(db).ListTransactions(ctx, PaymentQueryOptions{})
		assert.Error(t, err)
	})
	t.Run("backup", func(t *testing.T) {
		db := corruptScanRow(t, "backups", "", "")
		_, _, err := NewBackupModel(db).List(ctx, ListBackupsOptions{})
		assert.Error(t, err)
	})
	t.Run("bug", func(t *testing.T) {
		db := corruptScanRow(t, "bugs", "title", "'b2t'")
		_, _, err := NewBugModel(db).List(ctx, BugQueryOptions{})
		assert.Error(t, err)
	})
	t.Run("certificate", func(t *testing.T) {
		db := corruptScanRow(t, "certificates", "", "")
		_, _, err := NewCertificateModel(db).List(ctx, ListCertificatesOptions{})
		assert.Error(t, err)
	})
	t.Run("certificate alerts", func(t *testing.T) {
		db := corruptScanRow(t, "certificate_alerts", "", "")
		_, _, err := NewCertificateModel(db).ListAlerts(ctx, 1, 10)
		assert.Error(t, err)
	})
	t.Run("faq", func(t *testing.T) {
		db := corruptScanRow(t, "faqs", "", "")
		_, _, err := NewFAQModel(db).List(ctx, ListFAQOptions{})
		assert.Error(t, err)
	})
	t.Run("feedback", func(t *testing.T) {
		db := corruptScanRow(t, "feedbacks", "", "")
		_, _, err := NewFeedbackModel(db).List(ctx, ListFeedbackOptions{Status: -1, ExcludeStatus: -1})
		assert.Error(t, err)
	})
	t.Run("function", func(t *testing.T) {
		db := corruptScanRow(t, "functions", "function_id", "'b2f'")
		_, _, err := NewFunctionModel(db).List(ctx, ListFunctionsOptions{})
		assert.Error(t, err)
	})
	t.Run("game", func(t *testing.T) {
		db := corruptScanRow(t, "games", "game_id, name", "'b2g', 'B2 Game'")
		_, _, err := NewGameModel(db).List(ctx, ListGamesOptions{})
		assert.Error(t, err)
	})
	t.Run("game release", func(t *testing.T) {
		db := corruptScanRow(t, "game_releases", "", "")
		_, _, err := NewGameReleaseModel(db).List(ctx, ReleaseQueryOptions{})
		assert.Error(t, err)
	})
	t.Run("hotpatch", func(t *testing.T) {
		db := corruptScanRow(t, "hotpatches", "", "")
		_, _, err := NewHotpatchModel(db).List(ctx, HotpatchQueryOptions{})
		assert.Error(t, err)
	})
	t.Run("message", func(t *testing.T) {
		db := corruptScanRow(t, "messages", "recipient, type", "'b2to', 'info'")
		_, _, err := NewMessageModel(db).List(ctx, NewListMessagesOptions())
		assert.Error(t, err)
		_, err = NewMessageModel(db).Recent(ctx, 5, "")
		assert.Error(t, err)
	})
	t.Run("permission", func(t *testing.T) {
		db := corruptScanRow(t, "permissions", "id, name, resource, action, category", "'b2p', 'n', 'r', 'a', 'c'")
		_, _, err := NewPermissionModel(db).List(ctx, ListPermissionsOptions{})
		assert.Error(t, err)
	})
	t.Run("player", func(t *testing.T) {
		db := corruptScanRow(t, "players", "username, game_id", "'b2p', 'b2g'")
		_, _, err := NewPlayerModel(db).List(ctx, ListPlayersOptions{})
		assert.Error(t, err)
	})
	t.Run("role", func(t *testing.T) {
		db := corruptScanRow(t, "roles", "name", "'b2r'")
		_, _, err := NewRoleModel(db).List(ctx, ListRolesOptions{})
		assert.Error(t, err)
	})
	t.Run("support ticket", func(t *testing.T) {
		db := corruptScanRow(t, "support_tickets", "", "")
		_, _, err := NewSupportModel(db).ListTickets(ctx, ListTicketsOptions{})
		assert.Error(t, err)
	})
	t.Run("task", func(t *testing.T) {
		db := corruptScanRow(t, "task_runs", "", "")
		_, _, err := NewTaskRunModel(db).List(ctx, ListTasksOptions{})
		assert.Error(t, err)
	})
	t.Run("ticket", func(t *testing.T) {
		db := corruptScanRow(t, "tickets", "title", "'b2t'")
		_, _, err := NewTicketModel(db).List(ctx, TicketQueryOptions{Status: -1})
		assert.Error(t, err)
	})
	t.Run("page version", func(t *testing.T) {
		db := corruptScanRow(t, "page_versions", "game_id, env, page_key", "'b2g', 'prod', 'b2k'")
		_, _, err := NewPageVersionModel(db).ListByScopeAndPageKeyPaged(ctx, "b2g", "prod", "b2k", 10, 0)
		assert.Error(t, err)
	})
	t.Run("capability semantic version", func(t *testing.T) {
		db := corruptScanRow(t, "capability_semantic_versions", "semantics_id", "1")
		_, _, err := NewCapabilitySemanticVersionModel(db).ListBySemanticsIDPaged(ctx, 1, 10, 0)
		assert.Error(t, err)
	})
	t.Run("config source binding", func(t *testing.T) {
		db := corruptScanRow(t, "config_source_bindings", "game_id, env, name, type", "'b2g', 'prod', 'n', 'db'")
		_, err := NewConfigSourceBindingModel(db).ListByScope(ctx, "b2g", "prod")
		assert.Error(t, err)
	})
	t.Run("term dictionary alias map", func(t *testing.T) {
		db := corruptScanRow(t, "term_dictionary", "domain, term_key, alias", "'b2d', 'b2k', 'b2a'")
		_, err := NewTermDictionaryModel(db).AliasMap(ctx)
		assert.Error(t, err)
	})
	t.Run("component template upsert scan error", func(t *testing.T) {
		db := corruptScanRow(t, "component_templates", "key, name, tree", "'b2k', '{}', '{}'")
		err := NewComponentTemplateModel(db).UpsertBuiltin(ctx, &ComponentTemplate{Key: "b2k", Name: JSON(`{}`)})
		assert.Error(t, err)
	})
}
