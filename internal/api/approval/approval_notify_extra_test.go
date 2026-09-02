// 覆盖目标：notify.go 的 notifyApprovalEvent 主链路与守卫分支、
// approvalRecipients（admin 角色解析）、dedupe；以及 currentApprovalScope
// 与 cloneApprovalMetadata 的分支。
package approval

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	notify "github.com/cuihairu/croupier/internal/service/notify"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newNotifyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func seedAdminWithRole(t *testing.T, db *gorm.DB, username, roleName string) {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	admin := &model.Admin{Username: username, Status: 1}
	require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	role := &model.Role{Name: roleName}
	require.NoError(t, roleModel.Create(ctx, role))
	require.NoError(t, adminModel.AssignRole(ctx, admin.ID, role.ID))
}

func TestNotifyApprovalEvent_DeliversToAdminsAndActor(t *testing.T) {
	db := newNotifyTestDB(t)
	seedAdminWithRole(t, db, "boss", "admin")

	s := NewService(&svc.ServiceContext{
		AdminModel:    model.NewAdminModel(db),
		NotifyService: notify.New(nil, model.NewMessageModel(db)),
	})
	record := &approvals.Approval{ID: "ap-1", State: "approved", Actor: "tester", FunctionID: "f.x"}
	s.notifyApprovalEvent(context.Background(), "approval.approved", record, "标题", "内容")

	var messages []model.Message
	require.NoError(t, db.Find(&messages).Error)
	recipients := map[string]bool{}
	for _, msg := range messages {
		recipients[msg.To] = true
		assert.Equal(t, "approval.approved", msg.Type)
	}
	assert.True(t, recipients["boss"], "admin 角色用户应收站内信: %v", recipients)
	assert.True(t, recipients["tester"], "发起人应收站内信: %v", recipients)
	assert.Len(t, messages, 2)
}

func TestNotifyApprovalEvent_NilGuards(t *testing.T) {
	var nilSvc *Service
	assert.NotPanics(t, func() {
		nilSvc.notifyApprovalEvent(context.Background(), "e", nil, "t", "m")
	})

	s := NewService(&svc.ServiceContext{})
	assert.NotPanics(t, func() {
		// svcCtx 非 nil 但 NotifyService 缺失 / record 缺失。
		s.notifyApprovalEvent(context.Background(), "e", nil, "t", "m")
		s.notifyApprovalEvent(context.Background(), "e", &approvals.Approval{ID: "x"}, "t", "m")
	})
}

func TestNotifyApprovalEvent_RecipientResolutionFailureFallsBackToActor(t *testing.T) {
	db := newNotifyTestDB(t)
	seedAdminWithRole(t, db, "boss", "admin")
	// admins 表缺失 → recipients 解析失败 → 仅剩 actor 收信。
	require.NoError(t, db.Migrator().DropTable("admins"))

	s := NewService(&svc.ServiceContext{
		AdminModel:    model.NewAdminModel(db),
		NotifyService: notify.New(nil, model.NewMessageModel(db)),
	})
	assert.NotPanics(t, func() {
		s.notifyApprovalEvent(context.Background(), "e", &approvals.Approval{Actor: "tester"}, "t", "m")
	})

	var messages []model.Message
	require.NoError(t, db.Find(&messages).Error)
	require.Len(t, messages, 1)
	assert.Equal(t, "tester", messages[0].To)
}

func TestApprovalRecipients(t *testing.T) {
	db := newNotifyTestDB(t)
	seedAdminWithRole(t, db, "boss", "admin")
	seedAdminWithRole(t, db, "root", "super_admin")

	s := NewService(&svc.ServiceContext{AdminModel: model.NewAdminModel(db)})
	recipients, err := s.approvalRecipients(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"boss"}, recipients, "仅 admin 角色被列为接收人")

	// AdminModel 缺失 → nil, nil。
	s2 := NewService(&svc.ServiceContext{})
	recipients, err = s2.approvalRecipients(context.Background())
	require.NoError(t, err)
	assert.Nil(t, recipients)
}

func TestDedupe(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, dedupe([]string{"", "a", "a", "b", "", "a"}))
	assert.Empty(t, dedupe(nil))
	assert.Empty(t, dedupe([]string{"", ""}))
}

func TestCurrentApprovalScope(t *testing.T) {
	// 无 scope → BadRequest。
	_, err := currentApprovalScope(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scope")

	// 带 scope → 原样返回（去空白）。
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: " demo ", Env: " prod "})
	scope, err := currentApprovalScope(ctx)
	require.NoError(t, err)
	assert.Equal(t, svc.GameScope{GameID: "demo", Env: "prod"}, scope)
}

func TestCloneApprovalMetadata_DropsBlankEntries(t *testing.T) {
	out := cloneApprovalMetadata(map[string]string{
		"valid": "v", " ": "v", "k": " ", "": "v",
	})
	assert.Equal(t, map[string]string{"valid": "v"}, out)
	assert.Empty(t, cloneApprovalMetadata(nil))
}
