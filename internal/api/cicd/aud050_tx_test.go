package cicd

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// aud050DeleteEnv 组装一条接入 + 两条挂靠构建记录（AUD-050 级联删除）。
func aud050DeleteEnv(t *testing.T) (*Service, *gorm.DB, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&model.CicdIntegration{}, &model.CicdBuild{}))
	svcCtx := &svc.ServiceContext{
		DB:                   db,
		CicdIntegrationModel: model.NewCicdIntegrationModel(db),
		CicdBuildModel:       model.NewCicdBuildModel(db),
	}
	row := &model.CicdIntegration{
		GameID: "demo", Env: "prod", Kind: "generic", Name: "aud050-ci",
		Endpoint: "https://ci.example.com", Token: "aud050-token", Enabled: true,
	}
	require.NoError(t, db.Create(row).Error)
	for _, ext := range []string{"b-1", "b-2"} {
		require.NoError(t, db.Create(&model.CicdBuild{
			IntegrationID: row.ID, ExternalID: ext, Kind: "generic", Status: "success",
		}).Error)
	}
	return NewService(svcCtx), db, row.ID
}

func aud050BuildCount(t *testing.T, db *gorm.DB, integrationID uint) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&model.CicdBuild{}).Where("integration_id = ?", integrationID).Count(&count).Error)
	return count
}

// 第二步（删接入行）失败时，第一步已删的 builds 必须随事务回滚：级联
// 删除要么整体发生、要么整体不发生（AUD-050 半提交回归）。
func TestAud050_DeleteIntegrationRollsBackBuilds(t *testing.T) {
	s, db, id := aud050DeleteEnv(t)
	require.EqualValues(t, 2, aud050BuildCount(t, db, id))

	// 仅对 cicd_integrations 注入删除失败：builds 先删成功，接入行删除失败
	// → 事务回滚，builds 恢复。
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("aud050_fail_int_delete", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "cicd_integrations" {
			_ = tx.AddError(errors.New("injected integration delete failure"))
		}
	}))

	err := s.Delete(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected integration delete failure")

	// 回滚面：接入行仍在、builds 仍在（半提交被事务挡住）。
	row, err := s.integrationByID(context.Background(), id)
	require.NoError(t, err, "接入行应随回滚保留")
	assert.Equal(t, id, row.ID)
	assert.EqualValues(t, 2, aud050BuildCount(t, db, id), "已删的 builds 应随回滚恢复")
}

// 无注入时级联删除整体成功（对照）。
func TestAud050_DeleteIntegrationCascadesCleanly(t *testing.T) {
	s, db, id := aud050DeleteEnv(t)

	require.NoError(t, s.Delete(context.Background(), id))
	_, err := s.integrationByID(context.Background(), id)
	require.Error(t, err, "接入行应已删除")
	assert.EqualValues(t, 0, aud050BuildCount(t, db, id), "挂靠 builds 应已级联删除")
}
