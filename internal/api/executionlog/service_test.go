package executionlog

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

func newExecLogTestService(t *testing.T, username string) (*Service, *svc.ServiceContext, context.Context) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExecutionLog{}, &model.Admin{}, &model.Role{}, &model.AdminRole{}, &model.RolePermission{}, &model.Permission{}))

	admin := model.Admin{Username: "alice", Status: 1}
	require.NoError(t, db.Create(&admin).Error)
	other := model.Admin{Username: "bob", Status: 1}
	require.NoError(t, db.Create(&other).Error)

	svcCtx := &svc.ServiceContext{
		DB:                     db,
		AdminModel:             model.NewAdminModel(db),
		RoleModel:              model.NewRoleModel(db),
		PermissionModel:        model.NewPermissionModel(db),
		PublishedPageSpecModel: model.NewPublishedPageSpecModel(db),
		AuditService:           audit.NewAuditService(audit.NewInMemoryAuditStore(), nil),
		Cache:                  cache.NewNullCache(),
		CacheHelper:            cache.NewCacheHelper(cache.NewNullCache()),
		ExecutionLogModel:      model.NewExecutionLogModel(db),
	}
	ctx := context.WithValue(context.Background(), "username", username)
	return NewService(svcCtx), svcCtx, ctx
}

func seedExecLog(t *testing.T, svcCtx *svc.ServiceContext, actor, functionID string) model.ExecutionLog {
	t.Helper()
	item := model.ExecutionLog{
		GameID: "demo-game", Env: "development", Source: "invoke",
		FunctionID: functionID, Actor: actor, Status: "ok",
		RequestPayload: model.JSON(`{"keyword":"k1"}`),
		ResponseBody:   model.JSON(`{"ok":true}`),
		CreatedAt:      time.Now().UTC(),
	}
	require.NoError(t, svcCtx.ExecutionLogModel.Create(context.Background(), &item))
	return item
}

func TestListMineReturnsOnlyOwnRecords(t *testing.T) {
	s, svcCtx, ctx := newExecLogTestService(t, "alice")
	seedExecLog(t, svcCtx, "alice", "mail.send")
	seedExecLog(t, svcCtx, "bob", "player.ban")

	resp, err := s.List(ctx, &ListRequest{Mine: true})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "alice", resp.Items[0].Actor)
}

func TestListMineWithoutUsernameReturnsEmpty(t *testing.T) {
	s, svcCtx, _ := newExecLogTestService(t, "alice")
	seedExecLog(t, svcCtx, "alice", "mail.send")

	resp, err := s.List(context.Background(), &ListRequest{Mine: true})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestListAllRequiresAuditPermission(t *testing.T) {
	s, svcCtx, ctx := newExecLogTestService(t, "alice")
	seedExecLog(t, svcCtx, "bob", "player.ban")

	// 无权限 → 403
	_, err := s.List(ctx, &ListRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看执行留痕")
}

func TestListScopeIsolation(t *testing.T) {
	s, svcCtx, _ := newExecLogTestService(t, "alice")
	item := seedExecLog(t, svcCtx, "bob", "player.ban")
	// 另一 scope 的记录
	other := item
	other.ID = 0
	other.GameID = "other-game"
	require.NoError(t, svcCtx.ExecutionLogModel.Create(context.Background(), &other))

	// alice 的 scope 是 demo-game（X-Game-ID 上下文）
	ctx := context.WithValue(context.Background(), "username", "alice")
	ctx = svc.WithGameScope(ctx, svc.GameScope{GameID: "demo-game", Env: "development"})
	// alice 无审计权限，这里直接用模型构造权限：给 alice 授 audit:read
	grantAuditRead(t, svcCtx)

	resp, err := s.List(ctx, &ListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "demo-game", resp.Items[0].GameID)
}

func TestGetOwnRecordIncludesPayload(t *testing.T) {
	s, svcCtx, ctx := newExecLogTestService(t, "alice")
	seeded := seedExecLog(t, svcCtx, "alice", "mail.send")

	detail, err := s.Get(ctx, &GetRequest{ID: seeded.ID})
	require.NoError(t, err)
	assert.Equal(t, "alice", detail.Actor)
	require.JSONEq(t, `{"keyword":"k1"}`, string(detail.RequestPayload))
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(detail.ResponseBody, &body))
	assert.Equal(t, true, body["ok"])
}

func TestGetOthersRecordRequiresPermission(t *testing.T) {
	s, svcCtx, ctx := newExecLogTestService(t, "alice")
	seeded := seedExecLog(t, svcCtx, "bob", "player.ban")

	_, err := s.Get(ctx, &GetRequest{ID: seeded.ID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看执行留痕")
}

func grantAuditRead(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()
	perm := model.Permission{ID: "audit:read", Name: "audit read"}
	require.NoError(t, svcCtx.DB.Where("id = ?", perm.ID).FirstOrCreate(&perm).Error)
	// 找到 alice 的角色或直接建角色绑定
	role := model.Role{Name: "auditor", Description: "auditor"}
	require.NoError(t, svcCtx.DB.Where("name = ?", role.Name).FirstOrCreate(&role).Error)
	var admin model.Admin
	require.NoError(t, svcCtx.DB.Where("username = ?", "alice").First(&admin).Error)
	require.NoError(t, svcCtx.DB.FirstOrCreate(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}, model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	require.NoError(t, svcCtx.DB.FirstOrCreate(&model.RolePermission{RoleID: role.ID, PermissionID: perm.ID}, model.RolePermission{RoleID: role.ID, PermissionID: perm.ID}).Error)
	_ = strings.TrimSpace("")
}
