package registry_test

// 覆盖目标（组 E）：store_fnfloor.go / store_sdkfloor.go 的 DB 路径与错误
// 分支——批量/单查失败降级、写入前置查询失败、抬升 UPDATE 与首行 CREATE
// 失败只告警、nil receiver 守卫。本包禁止 import internal/model 的约束只针对
// 包内文件（import cycle），外部测试包 registry_test 不受影响（既有测试同款）。
// 错误注入用 gorm 回调（Before 阶段按表名注错），确定性、无时序依赖。

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
)

// covEMatchTable 在回调触发点解析语句目标表：Query 链中 Statement.Table 可能
// 尚未解析（gorm 在 Execute 前把 Dest 回填为 Model，需要手动 Parse）。
func covEMatchTable(tx *gorm.DB, table string) bool {
	if tx.Statement == nil {
		return false
	}
	if tx.Statement.Table == "" && tx.Statement.Model != nil {
		_ = tx.Statement.Parse(tx.Statement.Model)
	}
	return tx.Statement.Table == table
}

// covEInjectQueryFail 只让指定表的 SELECT 失败（Query/Row 链），写放行——
// 用于「读失败降级」与「写路径前置查询失败」两类分支。
func covEInjectQueryFail(db *gorm.DB, table string) (remove func()) {
	name := "test.cov_e.failquery." + table
	fail := func(tx *gorm.DB) {
		if covEMatchTable(tx, table) {
			_ = tx.AddError(errors.New("injected query failure for " + table))
		}
	}
	_ = db.Callback().Query().Before("gorm:query").Register(name, fail)
	_ = db.Callback().Row().Before("gorm:row").Register(name, fail)
	return func() {
		_ = db.Callback().Query().Remove(name)
		_ = db.Callback().Row().Remove(name)
	}
}

// covEInjectWriteFail 只让指定表的写（Create/Update 回调链）失败。
// 高水位的抬升是 Update("sdk_version")、首行落库是 Create，均走这两条链。
func covEInjectWriteFail(db *gorm.DB, table string) (remove func()) {
	name := "test.cov_e.failwrite." + table
	fail := func(tx *gorm.DB) {
		if covEMatchTable(tx, table) {
			_ = tx.AddError(errors.New("injected write failure for " + table))
		}
	}
	_ = db.Callback().Create().Before("gorm:create").Register(name, fail)
	_ = db.Callback().Update().Before("gorm:update").Register(name, fail)
	return func() {
		_ = db.Callback().Create().Remove(name)
		_ = db.Callback().Update().Remove(name)
	}
}

// nil receiver 守卫：零值 Store 指针（裁剪构造）下各门槛读写入口短路——
// 读返回空值、观测 no-op、设置返回显式错误。
func TestCoverageE_NilStoreGuards(t *testing.T) {
	var s *registry.Store
	assert.Empty(t, s.GetFunctionVersionFloors("demo", "dev"))
	assert.Empty(t, s.GetFunctionVersionFloor("demo", "dev", "player.ban"))
	assert.NoError(t, s.DeleteFunctionVersionFloor("demo", "dev", "player.ban"))
	assert.Empty(t, s.GetSDKVersionFloor("demo", "dev", "go"))
	s.ObserveSDKVersion("demo", "dev", "go", "1.0.0") // 不得 panic
	assert.Error(t, s.SetFunctionVersionFloor("demo", "dev", "player.ban", "1.0.0", "admin"))
}

// function_version_floors 的 DB 错误分支：批量查失败降级空表（门槛缺失只
// 放宽判定不阻断注册）、单查失败返回空串、写入路径前置 First 的非 NotFound
// 错误原样上抛（调用方可区分「查不到」与「查坏了」）。
func TestCoverageE_FnFloorDBErrorBranches(t *testing.T) {
	db := setupFnFloorDB(t)
	s := registry.NewStoreWithDB(db)

	remove := covEInjectQueryFail(db, "function_version_floors")
	assert.Empty(t, s.GetFunctionVersionFloors("demo", "dev"), "批量查询失败降级空表")
	assert.Empty(t, s.GetFunctionVersionFloor("demo", "dev", "player.ban"), "单查询失败返回空串")
	err := s.SetFunctionVersionFloor("demo", "dev", "player.ban", "1.0.0", "admin")
	require.Error(t, err, "写入前置查询失败原样上抛")
	assert.Contains(t, err.Error(), "injected query failure")
	remove()

	// 注错解除后行为恢复（证明注错没有破坏库本身）。
	require.NoError(t, s.SetFunctionVersionFloor("demo", "dev", "player.ban", "1.0.0", "admin"))
	assert.Equal(t, "1.0.0", s.GetFunctionVersionFloor("demo", "dev", "player.ban"))
}

// sdk_version_highwatermarks 的 DB 错误分支：查询失败返回 ""；既有行抬升
// UPDATE 失败与首行 CREATE 失败都只告警不落半态（门槛状态缺失只影响下次
// 判定，不阻断注册）；观测前置查询失败（非 NotFound）放弃本次观测。
func TestCoverageE_SDKFloorDBErrorBranches(t *testing.T) {
	db := setupSDKFloorDB(t)
	s := registry.NewStoreWithDB(db)

	// 查询失败：读返回空、观测放弃（不落行）。
	removeQ := covEInjectQueryFail(db, "sdk_version_highwatermarks")
	assert.Empty(t, s.GetSDKVersionFloor("demo", "dev", "go"))
	s.ObserveSDKVersion("demo", "dev", "go", "1.0.0")
	removeQ()
	var count int64
	require.NoError(t, db.Table("sdk_version_highwatermarks").Count(&count).Error)
	assert.Zero(t, count, "前置查询失败的观测不得落行")

	// 既有行抬升失败：UPDATE 注错，行保持旧值（不落半态）。
	require.NoError(t, db.Create(&model.SDKVersionHighwatermark{
		GameID: "demo", Env: "dev", SdkLanguage: "go", SdkVersion: "0.3.0",
	}).Error)
	removeW := covEInjectWriteFail(db, "sdk_version_highwatermarks")
	s.ObserveSDKVersion("demo", "dev", "go", "1.0.0")
	removeW()
	var stored string
	require.NoError(t, db.Raw(`SELECT sdk_version FROM sdk_version_highwatermarks WHERE sdk_language = 'go'`).Scan(&stored).Error)
	assert.Equal(t, "0.3.0", stored, "抬升失败不写半态")

	// 无行首观测 CREATE 失败：只告警。
	removeW2 := covEInjectWriteFail(db, "sdk_version_highwatermarks")
	s.ObserveSDKVersion("demo", "dev", "python", "1.0.0")
	removeW2()
	require.NoError(t, db.Table("sdk_version_highwatermarks").
		Where("sdk_language = ?", "python").Count(&count).Error)
	assert.Zero(t, count, "首行创建失败不得残留半行")
}
