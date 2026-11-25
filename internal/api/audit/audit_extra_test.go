// Coverage targets: the Export/VerifyChain handler error branches in export.go;
// in export_service.go the buildAuditQuery filters/aliases/time windows, the
// ExportRows error and empty-visible-scope branches, the buildAuditItem
// userAgent/trace_id/errorMessage branches, and the VerifyChain missing-store /
// chain-read-failure branches; in service.go the allows/resolveVisibleScopes
// branches and the GetAuditLogs error branches.
package audit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	auditcore "github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAuditExtraDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("audit_extra_%d", atomic.AddInt64(&auditTestDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, db.AutoMigrate(&auditcore.AuditModel{}))
	return db
}

// seedAuditViewer creates an admin with the audit:read permission; when roleName
// is super_admin it takes the admin passthrough branch, otherwise the restricted
// scope branch.
func seedAuditViewer(t *testing.T, db *gorm.DB, roleName string) (*model.Admin, string) {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	username := "viewer_" + strings.ToLower(roleName)
	admin := &model.Admin{Username: username, Status: 1}
	require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	role := &model.Role{Name: roleName}
	require.NoError(t, roleModel.Create(ctx, role))
	require.NoError(t, db.Create(&model.Permission{
		ID: "audit:read", Name: "Audit Read", Resource: "audit", Action: "read", Category: "audit",
	}).Error)
	require.NoError(t, roleModel.ReplacePermissions(ctx, role.ID, []string{"audit:read"}))
	require.NoError(t, adminModel.AssignRole(ctx, admin.ID, role.ID))
	return admin, username
}

// ---- handler ----

func TestHandler_GetAuditLogs_SizeAliasAndServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	r := gin.New()
	r.GET("/audit", handler.GetAuditLogs)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit?size=5", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"pageSize":5`)

	// Missing DB → service error → 500.
	h2 := NewHandler(NewService(&svc.ServiceContext{}))
	r2 := gin.New()
	r2.GET("/audit", h2.GetAuditLogs)
	rec2 := httptest.NewRecorder()
	r2.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/audit", nil))
	assert.Equal(t, http.StatusInternalServerError, rec2.Code)
}

func TestHandler_Export_QueryBindError(t *testing.T) {
	db := setupExportDB(t)
	_, r := newExportHandler(t, db)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit/export?page=abc", nil))
	// A form numeric conversion failure is mapped to 500 (not 200) via the
	// response.Error fallback.
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestHandler_Export_ExportRowsError(t *testing.T) {
	db := setupExportDB(t)
	require.NoError(t, db.Migrator().DropTable("audit_records"))
	_, r := newExportHandler(t, db)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit/export", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_VerifyChain_Error(t *testing.T) {
	db := setupExportDB(t)
	// Missing AuditService → VerifyChain error → 500.
	_, r := newExportHandler(t, db)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit/chain/verify", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// ---- ExportRows ----

func TestExportRows_StoreUnavailable(t *testing.T) {
	svc := NewService(&svc.ServiceContext{})
	_, _, err := svc.ExportRows(context.Background(), &AuditRequest{}, 10)
	require.Error(t, err)
	assert.ErrorIs(t, err, errStoreUnavailable)
}

func TestExportRows_DefaultLimitAndCountError(t *testing.T) {
	db := newAuditExtraDB(t)
	seedAuditEntry(t, db, "a", "u", "g", "prod", "t", "success", nil)
	svc := NewService(&svc.ServiceContext{DB: db})

	// limit<=0 → default limit, no truncation.
	items, truncated, err := svc.ExportRows(context.Background(), &AuditRequest{}, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.False(t, truncated)

	require.NoError(t, db.Migrator().DropTable("audit_records"))
	_, _, err = svc.ExportRows(context.Background(), &AuditRequest{}, 10)
	require.Error(t, err)
}

func TestExportRows_ScopeResolutionError(t *testing.T) {
	db := newAuditExtraDB(t)
	_, username := seedAuditViewer(t, db, "auditor")
	svc := NewService(&svc.ServiceContext{
		DB: db, AdminModel: model.NewAdminModel(db), RoleModel: model.NewRoleModel(db),
	})

	// No username in the context → permission resolution fails.
	_, _, err := svc.ExportRows(context.Background(), &AuditRequest{}, 10)
	require.Error(t, err)

	// A username that is not an admin plus a missing GameModel → game model unavailable.
	_, _, err = svc.ExportRows(context.WithValue(context.Background(), "username", username), &AuditRequest{}, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "game model unavailable")
}

func TestExportRows_EmptyVisibleScopesReturnsEmpty(t *testing.T) {
	db := newAuditExtraDB(t)
	seedAuditEntry(t, db, "a", "u", "g", "prod", "t", "success", nil)
	admin, username := seedAuditViewer(t, db, "auditor")
	// The scope points at a non-existent game → empty visible scopes → empty set.
	require.NoError(t, model.NewAdminModel(db).SetGameEnvScope(
		context.Background(), admin.ID, 424242, "prod"))

	svc := NewService(&svc.ServiceContext{
		DB: db, AdminModel: model.NewAdminModel(db), RoleModel: model.NewRoleModel(db),
		GameModel: model.NewGameModel(db),
	})
	ctx := context.WithValue(context.Background(), "username", username)
	items, truncated, err := svc.ExportRows(ctx, &AuditRequest{}, 10)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Empty(t, items)
}

func TestExportRows_ScanError(t *testing.T) {
	db := newAuditExtraDB(t)
	seedAuditEntry(t, db, "a", "u", "g", "prod", "t", "success", nil)
	// Corrupt the timestamp column → scanning the row into time.Time fails.
	require.NoError(t, db.Exec("UPDATE audit_records SET timestamp = 'not-a-time'").Error)
	svc := NewService(&svc.ServiceContext{DB: db})

	_, _, err := svc.ExportRows(context.Background(), &AuditRequest{}, 10)
	require.Error(t, err)
}

// ---- buildAuditQuery ----

func TestBuildAuditQuery_FiltersAliasesAndTimeWindows(t *testing.T) {
	db := newAuditExtraDB(t)
	seedAuditEntry(t, db, "auth.login", "u1", "g1", "prod", "t1", "success", nil)
	seedAuditEntry(t, db, "function.invoke", "u2", "g1", "dev", "t2", "success", nil)
	svc := NewService(&svc.ServiceContext{DB: db})
	ctx := context.Background()

	count := func(req *AuditRequest) int64 {
		q := svc.buildAuditQuery(ctx, req, nil, true)
		var n int64
		require.NoError(t, q.Count(&n).Error)
		return n
	}

	assert.Equal(t, int64(2), count(nil), "nil req 不加过滤")
	assert.Equal(t, int64(1), count(&AuditRequest{Kind: " invoke "}), "kind 别名展开")
	assert.Equal(t, int64(2), count(&AuditRequest{Kinds: " login , invoke "}), "kinds 逗号分隔")
	assert.Equal(t, int64(1), count(&AuditRequest{Action: "function.invoke"}), "action 精确匹配")
	assert.Equal(t, int64(1), count(&AuditRequest{UserID: "u1"}))
	assert.Equal(t, int64(1), count(&AuditRequest{Actor: "u2"}))
	assert.Equal(t, int64(1), count(&AuditRequest{GameID: "g1", Env: "prod"}))
	assert.Equal(t, int64(0), count(&AuditRequest{IP: "10.0.0.1"}))
	assert.Equal(t, int64(2), count(&AuditRequest{Start: time.Now().Add(-time.Hour).Format(time.RFC3339)}))
	assert.Equal(t, int64(0), count(&AuditRequest{Start: time.Now().Add(time.Hour).Format(time.RFC3339)}))
	assert.Equal(t, int64(2), count(&AuditRequest{End: time.Now().Add(time.Hour).Format(time.RFC3339)}))
	assert.Equal(t, int64(0), count(&AuditRequest{End: time.Now().Add(-time.Hour).Format(time.RFC3339)}))
	// Invalid time strings are ignored (no filtering).
	assert.Equal(t, int64(2), count(&AuditRequest{Start: "not-a-time", End: "also-not-a-time"}))
}

// Timezone regression: audit records are always landed in the database as UTC
// (NewAuditRecord uses time.Now().UTC()), and time window parameters must be
// normalized to UTC before binding — sqlite serializes time.Time into an
// offset-bearing string compared byte by byte, so on a non-UTC server a
// local-offset parameter computes the wrong window (verified on a CST machine:
// Start=-1h returned 0 rows, End=-1h returned 2 rows). The window offset is
// pinned to +05:30, independent of the host's timezone, so this reproduced on
// any machine before the fix.
func TestBuildAuditQuery_TimeWindowOffsetIndependent(t *testing.T) {
	db := newAuditExtraDB(t)
	seedAuditEntry(t, db, "auth.login", "u1", "g1", "prod", "t1", "success", nil)
	svc := NewService(&svc.ServiceContext{DB: db})
	ctx := context.Background()

	count := func(req *AuditRequest) int64 {
		q := svc.buildAuditQuery(ctx, req, nil, true)
		var n int64
		require.NoError(t, q.Count(&n).Error)
		return n
	}

	fixed := time.FixedZone("FIXED", 5*60*60+30*60)
	// A non-local-offset window enclosing the landing moment → matches 1 row.
	assert.Equal(t, int64(1), count(&AuditRequest{
		Start: time.Now().In(fixed).Add(-time.Hour).Format(time.RFC3339),
		End:   time.Now().In(fixed).Add(time.Hour).Format(time.RFC3339),
	}))
	// A future start → 0 rows.
	assert.Equal(t, int64(0), count(&AuditRequest{
		Start: time.Now().In(fixed).Add(time.Hour).Format(time.RFC3339),
	}))
	// A past end → 0 rows.
	assert.Equal(t, int64(0), count(&AuditRequest{
		End: time.Now().In(fixed).Add(-time.Hour).Format(time.RFC3339),
	}))
}

// ---- VerifyChain (service layer) ----

func TestVerifyChain_StoreUnavailableVariants(t *testing.T) {
	db := newAuditExtraDB(t)
	// Missing AuditService.
	_, err := NewService(&svc.ServiceContext{DB: db}).VerifyChain(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errStoreUnavailable)

	// AuditService present but the underlying store is nil (the store is only
	// checked once the table is non-empty).
	seedAuditEntry(t, db, "a", "u", "g", "prod", "t", "success", nil)
	svcCtx := &svc.ServiceContext{DB: db, AuditService: auditcore.NewAuditService(nil, nil)}
	_, err = NewService(svcCtx).VerifyChain(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errStoreUnavailable)
}

func TestVerifyChain_CountError(t *testing.T) {
	db := newAuditExtraDB(t)
	auditStore, err := auditcore.NewSQLAuditStore(db)
	require.NoError(t, err)
	svcCtx := &svc.ServiceContext{DB: db, AuditService: auditcore.NewAuditService(auditStore, nil)}
	require.NoError(t, db.Migrator().DropTable("audit_records"))

	_, err = NewService(svcCtx).VerifyChain(context.Background())
	require.Error(t, err)
}

func TestVerifyChain_ChainRangeUnmarshalError(t *testing.T) {
	db := newAuditExtraDB(t)
	auditStore, err := auditcore.NewSQLAuditStore(db)
	require.NoError(t, err)
	svcCtx := &svc.ServiceContext{DB: db, AuditService: auditcore.NewAuditService(auditStore, nil)}
	seedAuditEntry(t, db, "a", "u", "g", "prod", "t", "success", nil)
	// Corrupt actor_json → GetChainRange fails to unmarshal.
	require.NoError(t, db.Exec("UPDATE audit_records SET actor_json = 'not-json'").Error)

	_, err = NewService(svcCtx).VerifyChain(context.Background())
	require.Error(t, err)
}

// ---- buildAuditItem ----

func TestBuildAuditItem_MetadataAssembly(t *testing.T) {
	item := buildAuditItem(&auditRawRow{
		AuditID:      "aud-1",
		Timestamp:    time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC),
		EventType:    "function.invoke",
		Outcome:      "error",
		ActorJSON:    []byte(`{"id":"alice","userAgent":"Mozilla/5.0"}`),
		ResourceJSON: []byte(`{"id":"player.ban"}`),
		DetailsJSON:  []byte(`{"trace_id":"t-1","extra":"v"}`),
		ErrorMessage: "boom",
		GameID:       "demo",
		Env:          "prod",
		IP:           "10.0.0.1",
		ActorID:      "alice",
	})
	assert.Equal(t, "aud-1", item.ID)
	assert.Equal(t, "alice", item.UserID)
	assert.Equal(t, "demo", item.GameID)
	assert.Equal(t, "prod", item.Env)
	assert.Equal(t, "player.ban", item.Target)
	assert.Equal(t, "t-1", item.TraceID, "traceId 缺失时回退 trace_id")
	assert.Equal(t, "Mozilla/5.0", item.Metadata["userAgent"])
	assert.Equal(t, "10.0.0.1", item.Metadata["ip"])
	assert.Equal(t, "v", item.Metadata["extra"])
	assert.Equal(t, "boom", item.Metadata["error"], "错误信息进 metadata")
	assert.NotEmpty(t, item.CreatedAt)
}

func TestBuildAuditItem_EmptyRowKeepsZeroValues(t *testing.T) {
	item := buildAuditItem(&auditRawRow{})
	assert.Empty(t, item.Target)
	assert.Empty(t, item.TraceID)
	assert.Empty(t, item.GameID)
	assert.Empty(t, item.Env)
	assert.Empty(t, item.Metadata["error"])
}

// ---- auditScopeSet.allows ----

func TestAuditScopeSet_Allows(t *testing.T) {
	scopes := auditScopeSet{"game-a": {"prod": {}, "dev": {}}}
	assert.True(t, scopes.allows("Game-A", " PROD "), "大小写与空白归一后匹配")
	assert.True(t, scopes.allows("game-a", "dev"))
	assert.False(t, scopes.allows("game-b", "prod"), "未授权 game 拒绝")
	assert.False(t, scopes.allows("game-a", "staging"), "未授权 env 拒绝")
	assert.False(t, scopes.allows("", "prod"), "空 gameID 拒绝")
	assert.False(t, scopes.allows("game-a", ""), "空 env 拒绝")
	assert.False(t, auditScopeSet{}.allows("game-a", "prod"), "空可见域拒绝")
}

// ---- resolveVisibleScopes / GetAuditLogs ----

func TestGetAuditLogs_ScopeResolutionErrors(t *testing.T) {
	db := newAuditExtraDB(t)
	_, username := seedAuditViewer(t, db, "auditor")
	svc := NewService(&svc.ServiceContext{
		DB: db, AdminModel: model.NewAdminModel(db), RoleModel: model.NewRoleModel(db),
	})

	// No username → permission resolution fails.
	_, err := svc.GetAuditLogs(context.Background(), &AuditRequest{})
	require.Error(t, err)

	// Missing GameModel → game model unavailable.
	_, err = svc.GetAuditLogs(context.WithValue(context.Background(), "username", username), &AuditRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "game model unavailable")
}

func TestResolveVisibleScopes_AdminRoleUnrestricted(t *testing.T) {
	db := newAuditExtraDB(t)
	_, username := seedAuditViewer(t, db, "super_admin")
	svc := NewService(&svc.ServiceContext{
		DB: db, AdminModel: model.NewAdminModel(db), RoleModel: model.NewRoleModel(db),
		GameModel: model.NewGameModel(db),
	})

	scopes, unrestricted, err := svc.resolveVisibleScopes(context.WithValue(context.Background(), "username", username))
	require.NoError(t, err)
	assert.True(t, unrestricted, "super_admin 角色直通")
	assert.Nil(t, scopes)
}

func TestResolveVisibleScopes_EnvScopesErrorAndBlankEntries(t *testing.T) {
	db := newAuditExtraDB(t)
	admin, username := seedAuditViewer(t, db, "auditor")
	adminModel := model.NewAdminModel(db)
	svc := NewService(&svc.ServiceContext{
		DB: db, AdminModel: adminModel, RoleModel: model.NewRoleModel(db),
		GameModel: model.NewGameModel(db),
	})
	ctx := context.WithValue(context.Background(), "username", username)

	// Missing admin_game_env_scopes table → GetAdminEnvScopes fails.
	require.NoError(t, db.Migrator().DropTable("admin_game_env_scopes"))
	_, _, err := svc.resolveVisibleScopes(ctx)
	require.Error(t, err)
	require.NoError(t, db.Migrator().CreateTable(&model.AdminGameEnvScope{}))

	// A valid scope plus dirty data with a blank env: the valid entry enters the
	// visible scopes, the blank-env entry is skipped.
	game := &model.Game{GameID: "game-a", Name: "Game A"}
	require.NoError(t, svc.svcCtx.GameModel.Create(context.Background(), game))
	require.NoError(t, adminModel.SetGameEnvScope(ctx, admin.ID, game.ID, "prod"))
	require.NoError(t, db.Create(&model.AdminGameEnvScope{AdminID: admin.ID, GameID: game.ID, Env: " "}).Error)

	scopes, unrestricted, err := svc.resolveVisibleScopes(ctx)
	require.NoError(t, err)
	assert.False(t, unrestricted)
	assert.Contains(t, scopes, "game-a")
	assert.Len(t, scopes["game-a"], 1)
}

func TestResolveVisibleScopes_GameWithBlankGameID(t *testing.T) {
	db := newAuditExtraDB(t)
	admin, username := seedAuditViewer(t, db, "auditor")
	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	svc := NewService(&svc.ServiceContext{
		DB: db, AdminModel: adminModel, RoleModel: model.NewRoleModel(db),
		GameModel: gameModel,
	})
	ctx := context.WithValue(context.Background(), "username", username)

	// The game record exists but GameID is blank (map insert bypasses the
	// BeforeCreate derivation logic) → skipped, so visible scopes are empty.
	require.NoError(t, db.Table("games").Create(map[string]interface{}{
		"created_at": time.Now(), "updated_at": time.Now(),
		"game_id": "", "name": "Blank",
	}).Error)
	var gameID uint
	require.NoError(t, db.Raw("SELECT id FROM games LIMIT 1").Scan(&gameID).Error)
	require.NoError(t, adminModel.SetGameEnvScope(ctx, admin.ID, gameID, "prod"))

	scopes, unrestricted, err := svc.resolveVisibleScopes(ctx)
	require.NoError(t, err)
	assert.False(t, unrestricted)
	assert.Empty(t, scopes)
}

func TestGetAuditLogs_KindAliasFilterAndCountError(t *testing.T) {
	db := newAuditExtraDB(t)
	seedAuditEntry(t, db, "auth.login", "u1", "g1", "prod", "t1", "success", nil)
	seedAuditEntry(t, db, "function.invoke", "u2", "g1", "dev", "t2", "success", nil)
	svc := NewService(&svc.ServiceContext{DB: db})

	resp, err := svc.GetAuditLogs(context.Background(), &AuditRequest{Kind: "login", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "auth.login", resp.Items[0].Action)

	require.NoError(t, db.Migrator().DropTable("audit_records"))
	_, err = svc.GetAuditLogs(context.Background(), &AuditRequest{})
	require.Error(t, err)
}

// Documented behavior: when a restricted user (has audit:read, no game scope at
// all) goes through GetAuditLogs, buildAuditQuery returns a nil query while
// Count runs before the nil check → panic. This was a historical production
// defect (in ExportRows the nil check came before Count; GetAuditLogs had the
// order inverted), since fixed to short-circuit with an empty list; this test
// pins that behavior.
func TestGetAuditLogs_EmptyVisibleScopes_ReturnsEmpty(t *testing.T) {
	db := newAuditExtraDB(t)
	admin, username := seedAuditViewer(t, db, "auditor")
	require.NoError(t, model.NewAdminModel(db).SetGameEnvScope(
		context.Background(), admin.ID, 424242, "prod"))
	svc := NewService(&svc.ServiceContext{
		DB: db, AdminModel: model.NewAdminModel(db), RoleModel: model.NewRoleModel(db),
		GameModel: model.NewGameModel(db),
	})

	// Historical defect: query.Count ran before the nil check and panicked;
	// fixed to short-circuit with an empty list
	resp, err := svc.GetAuditLogs(context.WithValue(context.Background(), "username", username), &AuditRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 0, resp.Total)
}
