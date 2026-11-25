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

// aud050DeleteEnv assembles one integration row plus two attached build
// records (AUD-050 cascade delete).
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

// When the second step (deleting the integration row) fails, the builds
// already deleted in the first step must roll back with the transaction: the
// cascade delete happens entirely or not at all (AUD-050 half-commit
// regression).
func TestAud050_DeleteIntegrationRollsBackBuilds(t *testing.T) {
	s, db, id := aud050DeleteEnv(t)
	require.EqualValues(t, 2, aud050BuildCount(t, db, id))

	// Inject the delete failure only for cicd_integrations: the builds are
	// deleted successfully first, then the integration row delete fails
	// -> the transaction rolls back and the builds are restored.
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("aud050_fail_int_delete", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "cicd_integrations" {
			_ = tx.AddError(errors.New("injected integration delete failure"))
		}
	}))

	err := s.Delete(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected integration delete failure")

	// Rollback surface: the integration row is still there and the builds are
	// still there (the half-commit was blocked by the transaction).
	row, err := s.integrationByID(context.Background(), id)
	require.NoError(t, err, "接入行应随回滚保留")
	assert.Equal(t, id, row.ID)
	assert.EqualValues(t, 2, aud050BuildCount(t, db, id), "已删的 builds 应随回滚恢复")
}

// With no injection the cascade delete succeeds as a whole (control case).
func TestAud050_DeleteIntegrationCascadesCleanly(t *testing.T) {
	s, db, id := aud050DeleteEnv(t)

	require.NoError(t, s.Delete(context.Background(), id))
	_, err := s.integrationByID(context.Background(), id)
	require.Error(t, err, "接入行应已删除")
	assert.EqualValues(t, 0, aud050BuildCount(t, db, id), "挂靠 builds 应已级联删除")
}
