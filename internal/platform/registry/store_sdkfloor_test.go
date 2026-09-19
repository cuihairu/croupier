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

// setupSDKFloorDB 迁出带 sdk_version_highwatermarks 的 sqlite 内存库。
func setupSDKFloorDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SDKVersionHighwatermark{}))
	return db
}

func TestSDKVersionFloor_MemoryMode(t *testing.T) {
	s := registry.NewStore()

	// 无观测记录：空 floor（门槛不生效）
	assert.Empty(t, s.GetSDKVersionFloor("demo", "prod", "go"))

	// 抬升 + 读回（per (game_id, env, language) 隔离）
	s.ObserveSDKVersion("demo", "prod", "go", "0.3.1")
	assert.Equal(t, "0.3.1", s.GetSDKVersionFloor("demo", "prod", "go"))
	assert.Empty(t, s.GetSDKVersionFloor("demo", "staging", "go"))
	assert.Empty(t, s.GetSDKVersionFloor("demo", "prod", "python"))

	// 低版本不降级、同版本幂等
	s.ObserveSDKVersion("demo", "prod", "go", "0.2.9")
	assert.Equal(t, "0.3.1", s.GetSDKVersionFloor("demo", "prod", "go"))
	s.ObserveSDKVersion("demo", "prod", "go", "0.3.1")
	assert.Equal(t, "0.3.1", s.GetSDKVersionFloor("demo", "prod", "go"))

	// 更高版本继续抬升
	s.ObserveSDKVersion("demo", "prod", "go", "0.4.0")
	assert.Equal(t, "0.4.0", s.GetSDKVersionFloor("demo", "prod", "go"))

	// 不可解析版本（unknown/空/垃圾）不抬升不落记
	s.ObserveSDKVersion("demo", "prod", "python", "unknown")
	s.ObserveSDKVersion("demo", "prod", "python", "")
	s.ObserveSDKVersion("demo", "prod", "python", "v1.2")
	assert.Empty(t, s.GetSDKVersionFloor("demo", "prod", "python"))

	// 空 language 是无效观测
	s.ObserveSDKVersion("demo", "prod", "", "1.0.0")
	assert.Empty(t, s.GetSDKVersionFloor("demo", "prod", ""))
}

func TestSDKVersionFloor_DBMode(t *testing.T) {
	db := setupSDKFloorDB(t)
	s := registry.NewStoreWithDB(db)

	assert.Empty(t, s.GetSDKVersionFloor("demo", "prod", "cpp"))

	s.ObserveSDKVersion("demo", "prod", "cpp", "0.3.0")
	assert.Equal(t, "0.3.0", s.GetSDKVersionFloor("demo", "prod", "cpp"))

	// 低版本不覆盖，高版本抬升
	s.ObserveSDKVersion("demo", "prod", "cpp", "0.1.4")
	assert.Equal(t, "0.3.0", s.GetSDKVersionFloor("demo", "prod", "cpp"))
	s.ObserveSDKVersion("demo", "prod", "cpp", "1.0.0")
	assert.Equal(t, "1.0.0", s.GetSDKVersionFloor("demo", "prod", "cpp"))

	// 不可解析版本不落行
	s.ObserveSDKVersion("demo", "prod", "csharp", "unknown")
	assert.Empty(t, s.GetSDKVersionFloor("demo", "prod", "csharp"))

	// 行数：仅 cpp 一行
	var count int64
	require.NoError(t, db.Table("sdk_version_highwatermarks").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
