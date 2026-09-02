// 覆盖目标：approvalNotifyRecipients（0%）、enforceFunctionPolicy 与
// createFunctionApproval 的错误分支。
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

func newApprovalFixtureDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.AdminRole{}))
	return db
}

func TestApprovalNotifyRecipients_NilModel_Empty(t *testing.T) {
	got, err := approvalNotifyRecipients(context.Background(), &svc.ServiceContext{})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestApprovalNotifyRecipients_ListsActiveAdminRole(t *testing.T) {
	db := newApprovalFixtureDB(t)
	require.NoError(t, db.Exec("INSERT INTO roles(name,description) VALUES('admin','a'),('viewer','v')").Error)
	require.NoError(t, db.Exec("INSERT INTO admins(username,nickname,status,password_hash) VALUES('a1','x',1,'h'),('a2','y',0,'h')").Error)
	var a1ID, a2ID, roleID uint
	require.NoError(t, db.Raw("SELECT id FROM admins WHERE username='a1'").Scan(&a1ID).Error)
	require.NoError(t, db.Raw("SELECT id FROM admins WHERE username='a2'").Scan(&a2ID).Error)
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE name='admin'").Scan(&roleID).Error)
	require.NoError(t, db.Exec("INSERT INTO admin_roles(admin_id,role_id) VALUES(?,?),(?,?)", a1ID, roleID, a2ID, roleID).Error)

	svcCtx := &svc.ServiceContext{AdminModel: model.NewAdminModel(db)}
	got, err := approvalNotifyRecipients(context.Background(), svcCtx)
	require.NoError(t, err)
	assert.Equal(t, []string{"a1"}, got, "仅 status=active 且带 admin 角色的账号")
}

func TestApprovalNotifyRecipients_StoreError(t *testing.T) {
	db := newApprovalFixtureDB(t)
	require.NoError(t, db.Migrator().DropTable("admins"))
	svcCtx := &svc.ServiceContext{AdminModel: model.NewAdminModel(db)}
	_, err := approvalNotifyRecipients(context.Background(), svcCtx)
	require.Error(t, err)
}
