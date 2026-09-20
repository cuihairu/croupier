package registry_test

import (
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
)

// setupFnFloorDB 迁出带 function_version_floors 的 sqlite 内存库。
func setupFnFloorDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FunctionVersionFloor{}))
	return db
}

func TestFunctionVersionFloor_MemoryMode(t *testing.T) {
	s := registry.NewStore()

	// 无配置：空 floor
	assert.Empty(t, s.GetFunctionVersionFloor("demo", "dev", "player.ban"))
	assert.Empty(t, s.GetFunctionVersionFloors("demo", "dev"))

	// 设置 + 读回；per (game_id, env, function_id) 隔离
	require.NoError(t, s.SetFunctionVersionFloor("demo", "dev", "player.ban", "0.3.0", "admin"))
	require.NoError(t, s.SetFunctionVersionFloor("demo", "dev", "player.list", "0.2.0", "admin"))
	require.NoError(t, s.SetFunctionVersionFloor("demo", "prod", "player.ban", "9.9.9", "admin"))

	assert.Equal(t, "0.3.0", s.GetFunctionVersionFloor("demo", "dev", "player.ban"))
	floors := s.GetFunctionVersionFloors("demo", "dev")
	assert.Equal(t, map[string]string{"player.ban": "0.3.0", "player.list": "0.2.0"}, floors)
	assert.Equal(t, "9.9.9", s.GetFunctionVersionFloor("demo", "prod", "player.ban"), "env 隔离")

	// 更新
	require.NoError(t, s.SetFunctionVersionFloor("demo", "dev", "player.ban", "0.4.0", "admin"))
	assert.Equal(t, "0.4.0", s.GetFunctionVersionFloor("demo", "dev", "player.ban"))

	// 清空
	require.NoError(t, s.DeleteFunctionVersionFloor("demo", "dev", "player.ban"))
	assert.Empty(t, s.GetFunctionVersionFloor("demo", "dev", "player.ban"))
	assert.Equal(t, map[string]string{"player.list": "0.2.0"}, s.GetFunctionVersionFloors("demo", "dev"))
}

func TestFunctionVersionFloor_DBMode(t *testing.T) {
	db := setupFnFloorDB(t)
	s := registry.NewStoreWithDB(db)

	require.NoError(t, s.SetFunctionVersionFloor("demo", "dev", "player.ban", "0.3.0", "admin"))
	require.NoError(t, s.SetFunctionVersionFloor("demo", "dev", "player.list", "0.2.0", "admin"))
	require.NoError(t, s.SetFunctionVersionFloor("demo", "prod", "player.ban", "9.9.9", "admin"))

	assert.Equal(t, "0.3.0", s.GetFunctionVersionFloor("demo", "dev", "player.ban"))
	assert.Equal(t,
		map[string]string{"player.ban": "0.3.0", "player.list": "0.2.0"},
		s.GetFunctionVersionFloors("demo", "dev"))
	assert.Equal(t, "9.9.9", s.GetFunctionVersionFloor("demo", "prod", "player.ban"))

	// 更新同行（唯一键幂等，不插重）
	require.NoError(t, s.SetFunctionVersionFloor("demo", "dev", "player.ban", "0.4.1", "ops"))
	assert.Equal(t, "0.4.1", s.GetFunctionVersionFloor("demo", "dev", "player.ban"))
	var count int64
	require.NoError(t, db.Model(&model.FunctionVersionFloor{}).
		Where("game_id = 'demo' AND env = 'dev' AND function_id = 'player.ban'").
		Count(&count).Error)
	assert.Equal(t, int64(1), count, "更新不得产生重复行")

	// 清空（物理删除）
	require.NoError(t, s.DeleteFunctionVersionFloor("demo", "dev", "player.ban"))
	assert.Empty(t, s.GetFunctionVersionFloor("demo", "dev", "player.ban"))
	require.NoError(t, db.Model(&model.FunctionVersionFloor{}).
		Where("game_id = 'demo' AND env = 'dev' AND function_id = 'player.ban'").
		Count(&count).Error)
	assert.Equal(t, int64(0), count, "清空必须是物理删除（唯一索引不被软删残留占住）")
}

func TestFunctionVersionFloor_Validation(t *testing.T) {
	s := registry.NewStore()

	// 不可解析版本拒绝入库（门槛两端可解析才生效，死配置无意义）
	assert.Error(t, s.SetFunctionVersionFloor("demo", "dev", "player.ban", "unknown", "admin"))
	assert.Error(t, s.SetFunctionVersionFloor("demo", "dev", "player.ban", "", "admin"))
	assert.Error(t, s.SetFunctionVersionFloor("demo", "dev", "", "0.3.0", "admin"))
	// 合法变体：1-3 段数字、预发布前缀
	assert.NoError(t, s.SetFunctionVersionFloor("demo", "dev", "player.ban", "1.2", "admin"))
	assert.NoError(t, s.SetFunctionVersionFloor("demo", "dev", "player.ban", "0.3.0-rc1", "admin"))
	assert.Equal(t, "0.3.0-rc1", s.GetFunctionVersionFloor("demo", "dev", "player.ban"))
}
