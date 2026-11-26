package model

import (
	"context"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

// TestDeploymentModelCRUD 覆盖 OPS-004 部署包/发版单模型 CRUD 与表名。
func TestDeploymentModelCRUD(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&DeployPackage{}, &Deployment{}))

	pkgModel := NewDeployPackageModel(db)
	pkg := &DeployPackage{PackageID: "pkg-x", Name: "game", Version: "1.0.0", ObjectKey: "deploy/game/1.0.0.bin", Checksum: "abc", Role: "game-server"}
	require.NoError(t, pkgModel.Create(context.Background(), pkg))
	got, err := pkgModel.FindByID(context.Background(), "pkg-x")
	require.NoError(t, err)
	require.Equal(t, "pkg-x", got.PackageID)
	require.Equal(t, "deploy_packages", DeployPackage{}.TableName())

	depModel := NewDeploymentModel(db)
	dep := &Deployment{DeploymentID: "dep-x", PackageID: "pkg-x", ProcessName: "game", BatchPercent: 100, BatchCount: 1, State: DeploymentStateDraft, TargetsJSON: JSON([]byte(`[{"agentId":"a","batch":1,"state":"pending"}]`))}
	require.NoError(t, depModel.Create(context.Background(), dep))
	gotDep, err := depModel.FindByID(context.Background(), "dep-x")
	require.NoError(t, err)
	require.Equal(t, "dep-x", gotDep.DeploymentID)
	require.Equal(t, "deployments", Deployment{}.TableName())
}

// CompareAndSwapState：draft→deploying 原子 CAS。并发/二次调用第二次必须失败。

// CompareAndSwapState：draft→deploying 原子 CAS。并发/二次调用第二次必须失败。
func TestDeploymentModelCompareAndSwapState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(modelTestDBName()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Deployment{}))
	m := NewDeploymentModel(db)
	dep := &Deployment{DeploymentID: "dep-cas", PackageID: "pkg", ProcessName: "game", BatchPercent: 100, BatchCount: 1, State: DeploymentStateDraft}
	require.NoError(t, m.Create(context.Background(), dep))

	ok, row, err := m.CompareAndSwapState(context.Background(), "dep-cas", DeploymentStateDraft, DeploymentStateDeploying)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, DeploymentStateDeploying, row.State)
	// CAS 成功后 updated_at 必须持久化（此前仅 Update("state", to) 不动时间戳）。
	require.False(t, row.UpdatedAt.IsZero(), "CAS 必须写 updated_at")
	require.True(t, row.UpdatedAt.After(dep.CreatedAt), "CAS updated_at 应晚于创建时间")
	// 回读确认落库。
	persisted, err := m.FindByID(context.Background(), "dep-cas")
	require.NoError(t, err)
	require.Equal(t, row.UpdatedAt.Unix(), persisted.UpdatedAt.Unix(), "updated_at 必须随 CAS 持久化")

	// 第二次（状态已 deploying）必须 CAS 失败。
	ok2, _, err := m.CompareAndSwapState(context.Background(), "dep-cas", DeploymentStateDraft, DeploymentStateDeploying)
	require.NoError(t, err)
	require.False(t, ok2)
}
