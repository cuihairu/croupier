package audit

import (
	"context"
	"fmt"
	"testing"

	auditcore "github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTableBackedService builds the audit API service against a real
// audit_records table (in-memory SQLite). The legacy OpsStateStore-backed
// in-memory trail was removed; tests now seed via the audit core service.
func setupTableBackedService(t *testing.T, svcCtxExtra func(*svc.ServiceContext)) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	auditStore, err := auditcore.NewSQLAuditStore(db)
	require.NoError(t, err)
	_ = auditStore

	svcCtx := &svc.ServiceContext{DB: db}
	if svcCtxExtra != nil {
		svcCtxExtra(svcCtx)
	}
	return NewService(svcCtx), db
}

// seedAuditEntry writes one row through the audit core service.
func seedAuditEntry(t *testing.T, db *gorm.DB, action, actor, gameID, env, target, outcome string, metadata map[string]interface{}) {
	t.Helper()
	auditStore, err := auditcore.NewSQLAuditStore(db)
	require.NoError(t, err)
	auditSvc := auditcore.NewAuditService(auditStore, nil)
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	_, _ = auditSvc.Log(context.Background(), auditcore.AuditEventType(action),
		auditcore.WithActorID(actor, "user", actor),
		auditcore.WithResourceID("function", target),
		auditcore.WithGameID(gameID, env),
		auditcore.WithDetails(metadata),
		auditcore.WithOutcome(outcome, ""),
	)
}

func TestService_GetAuditLogs_LimitsNonAdminToAuthorizedScopes(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	gameModel := model.NewGameModel(db)
	admin := &model.Admin{Username: "auditor", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))
	role := &model.Role{Name: "auditor"}
	require.NoError(t, roleModel.Create(context.Background(), role))
	require.NoError(t, db.Create(&model.Permission{
		ID: "audit:read", Name: "Audit Read", Resource: "audit", Action: "read", Category: "audit",
	}).Error)
	require.NoError(t, roleModel.ReplacePermissions(context.Background(), role.ID, []string{"audit:read"}))
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	game := &model.Game{GameID: "game-a", Name: "Game A"}
	require.NoError(t, gameModel.Create(context.Background(), game))
	require.NoError(t, gameModel.AddEnvBinding(context.Background(), "game-a", "prod", "game_a_prod", "", ""))
	require.NoError(t, adminModel.SetGameEnvScope(context.Background(), admin.ID, game.ID, "prod"))

	service, db := setupTableBackedService(t, func(svcCtx *svc.ServiceContext) {
		svcCtx.AdminModel = adminModel
		svcCtx.RoleModel = roleModel
		svcCtx.GameModel = gameModel
	})
	seedAuditEntry(t, db, "admin.action", "ops", "game-a", "prod", "target", "success", map[string]interface{}{"traceId": "allowed"})
	seedAuditEntry(t, db, "admin.action", "ops", "game-b", "dev", "target", "success", map[string]interface{}{"traceId": "blocked"})
	seedAuditEntry(t, db, "admin.action", "ops", "", "", "target", "success", map[string]interface{}{"traceId": "global"})
	ctx := context.WithValue(context.Background(), "username", admin.Username)
	resp, err := service.GetAuditLogs(ctx, &AuditRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "allowed", resp.Items[0].Metadata["traceId"])

	resp, err = service.GetAuditLogs(ctx, &AuditRequest{GameID: "game-b", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestService_GetAuditLogs_Success(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "action1", "user1", "game1", "prod", "target1", "success", nil)
	seedAuditEntry(t, db, "action2", "user2", "game2", "dev", "target2", "success", nil)
	seedAuditEntry(t, db, "action3", "user1", "game1", "prod", "target3", "failure", nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Items), 3)
	assert.GreaterOrEqual(t, resp.Total, 3)
}

func TestService_GetAuditLogs_WithActionFilter(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "create", "user1", "game1", "prod", "target1", "success", nil)
	seedAuditEntry(t, db, "delete", "user2", "game2", "dev", "target2", "success", nil)
	seedAuditEntry(t, db, "create", "user1", "game1", "prod", "target3", "success", nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
		Action:   "create",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Items), 2)

	for _, item := range resp.Items {
		assert.Equal(t, "create", item.Action)
	}
}

func TestService_GetAuditLogs_WithUserFilter(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "action1", "user1", "game1", "prod", "target1", "success", nil)
	seedAuditEntry(t, db, "action2", "user2", "game2", "dev", "target2", "success", nil)
	seedAuditEntry(t, db, "action3", "user1", "game1", "prod", "target3", "success", nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
		UserID:   "user1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Items), 2)

	for _, item := range resp.Items {
		assert.Equal(t, "user1", item.UserID)
	}
}

func TestService_GetAuditLogs_WithBothFilters(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "create", "user1", "game1", "prod", "target1", "success", nil)
	seedAuditEntry(t, db, "create", "user2", "game2", "dev", "target2", "success", nil)
	seedAuditEntry(t, db, "delete", "user1", "game1", "prod", "target3", "success", nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
		Action:   "create",
		UserID:   "user1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Items), 1)

	if len(resp.Items) > 0 {
		assert.Equal(t, "user1", resp.Items[0].UserID)
		assert.Equal(t, "create", resp.Items[0].Action)
	}
}

func TestService_GetAuditLogs_EmptyResults(t *testing.T) {

	service, _ := setupTableBackedService(t, nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
		Action:   "nonexistent",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Items, 0)
	assert.Equal(t, 0, resp.Total)
}

func TestService_GetAuditLogs_DefaultPagination(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	for i := 0; i < 5; i++ {
		seedAuditEntry(t, db, "action", "user", "game", "prod", "target", "success", nil)
	}

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
}

func TestService_GetAuditLogs_ZeroPage(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "action", "user", "game", "prod", "target", "success", nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     0,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Page)
}

func TestService_GetAuditLogs_ZeroPageSize(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "action", "user", "game", "prod", "target", "success", nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 0,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 20, resp.PageSize)
}

func TestService_GetAuditLogs_MaxPageSize(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	for i := 0; i < 2000; i++ {
		seedAuditEntry(t, db, "action", "user", "game", "prod", "target", "success", nil)
	}

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10000,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1000, resp.PageSize)
	assert.Len(t, resp.Items, 1000)
}

func TestService_GetAuditLogs_MaxPage(t *testing.T) {

	service, _ := setupTableBackedService(t, nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     2_000_000_001,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1000000000, resp.Page)
}

func TestService_GetAuditLogs_Pagination(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	for i := 0; i < 25; i++ {
		seedAuditEntry(t, db, "action", "user", "game", "prod", "target", "success", nil)
	}

	resp1, _ := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})
	assert.Len(t, resp1.Items, 10)
	assert.Equal(t, 25, resp1.Total)

	resp2, _ := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     2,
		PageSize: 10,
	})
	assert.Len(t, resp2.Items, 10)

	resp3, _ := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     3,
		PageSize: 10,
	})
	assert.Len(t, resp3.Items, 5)
}

func TestService_GetAuditLogs_PageBeyondData(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	for i := 0; i < 5; i++ {
		seedAuditEntry(t, db, "action", "user", "game", "prod", "target", "success", nil)
	}

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     10,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Items, 0)
	assert.Equal(t, 5, resp.Total)
}

func TestService_GetAuditLogs_NilRequest(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "action", "user", "game", "prod", "target", "success", nil)

	resp, err := service.GetAuditLogs(context.Background(), nil)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_GetAuditLogs_NilOpsStateStore(t *testing.T) {
	service := NewService(&svc.ServiceContext{
		OpsStateStore: nil,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestService_GetAuditLogs_WithSizeAlias(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "action", "user", "game", "prod", "target", "success", nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page: 1,
		Size: 5,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 5, resp.PageSize)
}

func TestService_GetAuditLogs_PageSizeTakesPrecedence(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "action", "user", "game", "prod", "target", "success", nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
		Size:     5,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 10, resp.PageSize)
}

func TestService_GetAuditLogs_WithMetadata(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "action1", "user1", "game1", "prod", "target1", "success", nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Items), 1)
	assert.NotNil(t, resp.Items[0].Metadata)
}

func TestService_GetAuditLogs_SortedByCreatedAt(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "action1", "user1", "game", "prod", "target", "success", nil)
	seedAuditEntry(t, db, "action2", "user2", "game", "prod", "target", "success", nil)
	seedAuditEntry(t, db, "action3", "user3", "game", "prod", "target", "success", nil)

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Items should be sorted by CreatedAt descending (newest first)
	if len(resp.Items) >= 3 {
		assert.Equal(t, "user3", resp.Items[0].UserID)
		assert.Equal(t, "user2", resp.Items[1].UserID)
		assert.Equal(t, "user1", resp.Items[2].UserID)
	}
}

func TestService_GetAuditLogs_AllFieldsPresent(t *testing.T) {
	service, db := setupTableBackedService(t, nil)
	seedAuditEntry(t, db, "testaction", "testuser", "testgame", "testenv", "testtarget", "testresult", map[string]interface{}{"traceId": "testtrace"})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Items), 1)

	item := resp.Items[0]
	assert.Equal(t, "testuser", item.UserID)
	assert.Equal(t, "testgame", item.GameID)
	assert.Equal(t, "testenv", item.Env)
	assert.Equal(t, "testaction", item.Action)
	assert.Equal(t, "testtarget", item.Target)
	assert.Equal(t, "testresult", item.Result)
	assert.Equal(t, "testtrace", item.TraceID)
	assert.NotEmpty(t, item.CreatedAt)
	assert.NotNil(t, item.Metadata)
}

func TestNewService(t *testing.T) {
	svcCtx := &svc.ServiceContext{}

	service := NewService(svcCtx)

	assert.NotNil(t, service)
	assert.Equal(t, svcCtx, service.svcCtx)
}
