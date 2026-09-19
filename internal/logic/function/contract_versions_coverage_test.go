package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// contract_versions_logic 的边界分支补齐：scope/权限入口、入参钳制、
// DB 未初始化与故障注入（关闭底层连接）、快照缺失/损坏的 diff 降级。

func newVersionsLogicDB(t *testing.T) (*ContractVersionsLogic, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return NewContractVersionsLogic(context.Background(), &svc.ServiceContext{DB: db}), db
}

func insertRawVersion(t *testing.T, db *gorm.DB, ver *model.FunctionContractVersion) {
	t.Helper()
	require.NoError(t, db.Create(ver).Error)
}

func TestContractVersionsLogic_RequireScope(t *testing.T) {
	logic := NewContractVersionsLogic(context.Background(), &svc.ServiceContext{})
	scope := logic.RequireScope()
	assert.Empty(t, scope.GameID)
	assert.Empty(t, scope.Env)

	scoped := NewContractVersionsLogic(svc.WithGameScope(context.Background(),
		svc.GameScope{GameID: "  g  ", Env: " e "}), &svc.ServiceContext{})
	scope = scoped.RequireScope()
	assert.Equal(t, "g", scope.GameID, "scope 应去除首尾空白")
	assert.Equal(t, "e", scope.Env)
}

func TestContractVersionsLogic_CheckAccessAnonymousAllowed(t *testing.T) {
	// 无登录名（SDK/内部调用）放行，与 DescriptorsLogic.checkReadPermission
	// 同一准入面。
	logic := NewContractVersionsLogic(context.Background(), &svc.ServiceContext{})
	assert.NoError(t, logic.CheckAccess())
}

func TestContractVersionsLogic_ListNilDeps(t *testing.T) {
	logic := NewContractVersionsLogic(context.Background(), &svc.ServiceContext{})
	_, err := logic.List("g", "e", "f", 1, 10)
	assert.Error(t, err)

	logic = NewContractVersionsLogic(context.Background(), nil)
	_, err = logic.List("g", "e", "f", 1, 10)
	assert.Error(t, err)
}

func TestContractVersionsLogic_ListClampsPaging(t *testing.T) {
	logic, db := newVersionsLogicDB(t)
	seedVersions(t, db)

	// page/pageSize 越界统一钳制回默认窗
	resp, err := logic.List("g", "e", "player.ban", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.Size)

	resp, err = logic.List("g", "e", "player.ban", 1, 101)
	require.NoError(t, err)
	assert.Equal(t, 100, resp.Size, "pageSize 上限 100")
}

func TestContractVersionsLogic_ListDBFailure(t *testing.T) {
	logic, db := newVersionsLogicDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = logic.List("g", "e", "player.ban", 1, 10)
	assert.Error(t, err)
}

func TestContractVersionsLogic_DetailMissingDepsAndRows(t *testing.T) {
	// DB 未初始化
	logic := NewContractVersionsLogic(context.Background(), &svc.ServiceContext{})
	_, err := logic.Detail("g", "e", "player.ban", 1)
	assert.Error(t, err)

	// seq 不存在 → NotFound
	logic, db := newVersionsLogicDB(t)
	seedVersions(t, db)
	_, err = logic.Detail("g", "e", "player.ban", 999)
	assert.Error(t, err)

	// DB 故障
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = logic.Detail("g", "e", "player.ban", 1)
	assert.Error(t, err)
}

func TestContractVersionsLogic_DiffFindFailures(t *testing.T) {
	logic, db := newVersionsLogicDB(t)
	seedVersions(t, db)

	// from 不存在
	_, err := logic.Diff("g", "e", "player.ban", 424242, 2)
	assert.Error(t, err)
	// to 不存在
	_, err = logic.Diff("g", "e", "player.ban", 1, 424242)
	assert.Error(t, err)
}

func TestContractVersionsLogic_DiffCorruptSnapshots(t *testing.T) {
	logic, db := newVersionsLogicDB(t)
	// 直接插行控制 Snapshot 内容：seq1 空、seq2 损坏、seq3 合法。
	insertRawVersion(t, db, &model.FunctionContractVersion{
		GameID: "g", Env: "e", FunctionID: "f", Seq: 1, Version: "1.0.0", ChangeType: "created",
	})
	insertRawVersion(t, db, &model.FunctionContractVersion{
		GameID: "g", Env: "e", FunctionID: "f", Seq: 2, Version: "1.1.0", ChangeType: "updated",
		Snapshot: model.JSON(`{invalid-json`),
	})
	insertRawVersion(t, db, &model.FunctionContractVersion{
		GameID: "g", Env: "e", FunctionID: "f", Seq: 3, Version: "1.2.0", ChangeType: "updated",
		Snapshot: model.JSON(`{"id":"f","version":"1.2.0"}`),
	})
	insertRawVersion(t, db, &model.FunctionContractVersion{
		GameID: "g", Env: "e", FunctionID: "f", Seq: 4, Version: "1.3.0", ChangeType: "updated",
		Snapshot: model.JSON(`{"id":"f","version":"1.3.0"}`),
	})

	// from 快照缺失
	_, err := logic.Diff("g", "e", "f", 1, 3)
	assert.ErrorContains(t, err, "快照")
	// to 快照损坏
	_, err = logic.Diff("g", "e", "f", 3, 2)
	assert.ErrorContains(t, err, "快照")
	// 双方合法 → 正常 diff（version 字段变化）
	diff, err := logic.Diff("g", "e", "f", 3, 4)
	require.NoError(t, err)
	assert.False(t, diff.Breaking)
}

func TestContractVersionsLogic_DiffDBFailure(t *testing.T) {
	logic, db := newVersionsLogicDB(t)
	seedVersions(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = logic.Diff("g", "e", "player.ban", 1, 2)
	assert.Error(t, err)
}
