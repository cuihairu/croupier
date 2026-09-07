package assignment

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var authedSeq int64

// newAuthedContext 构造带 assignments:read/write 权限的 svcCtx 与请求 ctx。
func newAuthedContext(t *testing.T, assignmentsPath string) (context.Context, *svc.ServiceContext) {
	t.Helper()
	authedSeq++
	dbName := fmt.Sprintf("authed_%d_%d.db", os.Getpid(), authedSeq)
	db, err := gorm.Open(gsqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Admin{}, &model.AdminRole{}, &model.Role{}, &model.RolePermission{}, &model.Permission{},
	))

	perms := []model.Permission{
		{ID: "admin:all", Name: "admin:all", Resource: "*", Action: "*", Category: "admin"},
		{ID: "perm_read", Name: "assignments:read", Resource: "assignments", Action: "read", Category: "assignments"},
		{ID: "perm_write", Name: "assignments:write", Resource: "assignments", Action: "write", Category: "assignments"},
	}
	for i := range perms {
		require.NoError(t, db.Where("id = ?", perms[i].ID).FirstOrCreate(&perms[i]).Error)
	}

	role := model.Role{Name: "role_" + dbName}
	require.NoError(t, db.Create(&role).Error)
	for _, pid := range []string{"admin:all", "perm_read", "perm_write"} {
		require.NoError(t, db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: pid}).Error)
	}

	username := "cover_admin_" + dbName
	admin := model.Admin{Username: username, Nickname: "Cover", Email: username + "@example.com", PasswordHash: "$2a$10$hash", Status: 1}
	require.NoError(t, db.Create(&admin).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)

	svcCtx := &svc.ServiceContext{
		DB:              db,
		RegistryStore:   registry.NewStore(),
		AdminModel:      model.NewAdminModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
		Config: config.Config{
			Registry: config.RegistryConfig{AssignmentsPath: assignmentsPath},
		},
	}
	return context.WithValue(context.Background(), "username", username), svcCtx
}

// blockingFilePath 创建一个「父路径是文件」的 assignments 路径：
// ReadFile 返回 ENOTDIR（非 ENOENT），用于触发读取失败分支。
func blockingFilePath(t *testing.T) string {
	t.Helper()
	blocking := filepath.Join(t.TempDir(), "blocking_file")
	require.NoError(t, os.WriteFile(blocking, []byte("x"), 0o644))
	return filepath.Join(blocking, "sub", "assignments.json")
}

// ---- 权限失败 ----

func TestAssignmentsHistory_PermissionDenied(t *testing.T) {
	logic := NewAssignmentsHistoryLogic(context.Background(), &svc.ServiceContext{})
	resp, err := logic.AssignmentsHistory(&AssignmentsHistoryRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestAssignmentsList_PermissionDenied(t *testing.T) {
	logic := NewAssignmentsListLogic(context.Background(), &svc.ServiceContext{})
	resp, err := logic.AssignmentsList(&AssignmentsListRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

// ---- 读取失败（父路径是文件 → ENOTDIR）----

func TestAssignmentsHistory_ReadFailure(t *testing.T) {
	ctx, svcCtx := newAuthedContext(t, blockingFilePath(t))
	logic := NewAssignmentsHistoryLogic(ctx, svcCtx)
	resp, err := logic.AssignmentsHistory(&AssignmentsHistoryRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestAssignmentsList_ReadFailure(t *testing.T) {
	ctx, svcCtx := newAuthedContext(t, blockingFilePath(t))
	logic := NewAssignmentsListLogic(ctx, svcCtx)
	resp, err := logic.AssignmentsList(&AssignmentsListRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestAssignmentsUpdate_ReadFailure(t *testing.T) {
	ctx, svcCtx := newAuthedContext(t, blockingFilePath(t))
	logic := NewAssignmentsUpdateLogic(ctx, svcCtx)
	resp, err := logic.AssignmentsUpdate(&AssignmentsUpdateRequest{GameId: "demo"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

// ---- history 过滤与分页 ----

func writeHistoryFile(t *testing.T, path string, entries []assignmentHistoryEntry) {
	t.Helper()
	data, err := json.Marshal(entries)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
}

func TestAssignmentsHistory_FiltersAndPagination(t *testing.T) {
	tmp := t.TempDir()
	assignmentsPath := filepath.Join(tmp, "assignments.json")
	historyPath := filepath.Join(tmp, "assignments_history.json")
	writeHistoryFile(t, historyPath, []assignmentHistoryEntry{
		{ID: "1", GameID: "game-a", Env: "prod", Action: "assign"},
		{ID: "2", GameID: "game-a", Env: "dev", Action: "remove"},
		{ID: "3", GameID: "game-b", Env: "prod", Action: "assign"},
	})

	ctx, svcCtx := newAuthedContext(t, assignmentsPath)
	logic := NewAssignmentsHistoryLogic(ctx, svcCtx)

	extract := func(resp *AssignmentsHistoryResponse) []assignmentHistoryEntry {
		t.Helper()
		require.NotNil(t, resp)
		data := resp.Data.(map[string]interface{})
		items := data["items"].([]assignmentHistoryEntry)
		return items
	}

	t.Run("filter by mismatching game skips entries", func(t *testing.T) {
		resp, err := logic.AssignmentsHistory(&AssignmentsHistoryRequest{GameId: "game-c"})
		require.NoError(t, err)
		assert.Empty(t, extract(resp))
	})

	t.Run("filter by env mismatch skips entries", func(t *testing.T) {
		resp, err := logic.AssignmentsHistory(&AssignmentsHistoryRequest{Env: "staging"})
		require.NoError(t, err)
		assert.Empty(t, extract(resp))
	})

	t.Run("filter by action mismatch skips entries", func(t *testing.T) {
		resp, err := logic.AssignmentsHistory(&AssignmentsHistoryRequest{Action: "rollback"})
		require.NoError(t, err)
		assert.Empty(t, extract(resp))
	})

	t.Run("filters combine to single match", func(t *testing.T) {
		resp, err := logic.AssignmentsHistory(&AssignmentsHistoryRequest{GameId: "GAME-A", Env: "dev", Action: "remove"})
		require.NoError(t, err)
		items := extract(resp)
		require.Len(t, items, 1)
		assert.Equal(t, "2", items[0].ID)
	})

	t.Run("page beyond total clamps start", func(t *testing.T) {
		resp, err := logic.AssignmentsHistory(&AssignmentsHistoryRequest{Page: 99, PageSize: 10})
		require.NoError(t, err)
		data := resp.Data.(map[string]interface{})
		assert.Equal(t, 3, data["total"])
		assert.Empty(t, data["items"])
	})
}

// ---- update：action 推断与 save 失败 ----

func TestAssignmentsUpdate_RemoveActionInferred(t *testing.T) {
	tmp := t.TempDir()
	assignmentsPath := filepath.Join(tmp, "assignments.json")
	ctx, svcCtx := newAuthedContext(t, assignmentsPath)

	// 先赋予两个函数
	upsert := NewAssignmentsUpdateLogic(ctx, svcCtx)
	_, err := upsert.AssignmentsUpdate(&AssignmentsUpdateRequest{GameId: "demo", Env: "prod", Functions: []string{"fn.a", "fn.b"}})
	require.NoError(t, err)

	// 再传空函数列表：accepted 为空、before 非空 → action 推断为 remove
	resp, err := upsert.AssignmentsUpdate(&AssignmentsUpdateRequest{GameId: "demo", Env: "prod", Functions: nil})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// 历史记录中 action 应为 remove
	entries, err := loadAssignmentHistory(filepath.Join(tmp, "assignments_history.json"))
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "remove", entries[0].Action)

	// 分配数据被清空
	data, err := loadAssignments(assignmentsPath)
	require.NoError(t, err)
	assert.Empty(t, data["demo|prod"])
}

func TestAssignmentsUpdate_SaveFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory read-only permission")
	}
	tmp := t.TempDir()
	roDir := filepath.Join(tmp, "ro")
	require.NoError(t, os.MkdirAll(roDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })
	assignmentsPath := filepath.Join(roDir, "assignments.json")

	ctx, svcCtx := newAuthedContext(t, assignmentsPath)
	logic := NewAssignmentsUpdateLogic(ctx, svcCtx)
	resp, err := logic.AssignmentsUpdate(&AssignmentsUpdateRequest{GameId: "demo", Env: "prod", Functions: []string{"fn.a"}})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

// ---- helpers 错误分支 ----

func TestLoadAssignments_ReadError(t *testing.T) {
	_, err := loadAssignments(blockingFilePath(t))
	require.Error(t, err)
}

func TestLoadAssignmentHistory_ReadError(t *testing.T) {
	_, err := loadAssignmentHistory(blockingFilePath(t))
	require.Error(t, err)
}

func TestSaveAssignmentHistory_MarshalError(t *testing.T) {
	entries := []assignmentHistoryEntry{{
		ID:      "1",
		GameID:  "g",
		Details: map[string]interface{}{"x": math.NaN()},
	}}
	err := saveAssignmentHistory(filepath.Join(t.TempDir(), "history.json"), entries)
	require.Error(t, err)
}
