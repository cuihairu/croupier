package message

// List receiving-side visibility scope tests (incident-reports §6 + plugin
// mechanism §5.1): admins see everything; ordinary users = direct receipts ∪
// fallback-group broadcasts without ownership ∪ scope notifications of visible
// categories (leader/audience); other people's point-to-point messages stay
// private.

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var visibilityDBSeq uint64

func newVisibilityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("message_vis_%d", atomic.AddUint64(&visibilityDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

// seedVisMessage lands a message with title/source/scope (the messages shape
// of the report distribution chain; the title is the assertion key).
func seedVisMessage(t *testing.T, db *gorm.DB, to, title, source string, scope model.JSON) {
	t.Helper()
	msg := &model.Message{
		To:      to,
		Type:    "report",
		Title:   title,
		Content: "content",
		Status:  dbenum.MessageStatusUnread,
		Source:  source,
		Scope:   scope,
	}
	require.NoError(t, db.Create(msg).Error)
}

func visibilityService(db *gorm.DB, withAdmin bool) *Service {
	svcCtx := &svc.ServiceContext{MessageModel: model.NewMessageModel(db), DB: db}
	if withAdmin {
		svcCtx.AdminModel = model.NewAdminModel(db)
	}
	return NewService(svcCtx)
}

func listTitles(t *testing.T, s *Service, ctx context.Context, username string) map[string]bool {
	t.Helper()
	resp, err := s.List(ctx, username, &MessagesListRequest{Page: 1, PageSize: 50})
	require.NoError(t, err)
	out := map[string]bool{}
	for _, it := range resp.Items {
		out[fmt.Sprint(it.Title)] = true
	}
	return out
}

func TestServiceListVisibility_DirectAndPrivacy(t *testing.T) {
	db := newVisibilityTestDB(t)
	seedVisMessage(t, db, "alice", "to-alice", "", nil)
	seedVisMessage(t, db, "bob", "to-bob", "", nil)

	// Degrade without AdminModel / without a login state: filter by recipient,
	// others' point-to-point messages are not visible.
	got := listTitles(t, visibilityService(db, false), context.Background(), "alice")
	assert.True(t, got["to-alice"])
	assert.False(t, got["to-bob"])
}

func TestServiceListVisibility_FallbackBroadcast(t *testing.T) {
	db := newVisibilityTestDB(t)
	seedVisMessage(t, db, "group:gm-ops", "broadcast", "system", nil)           // fallback broadcast, empty scope
	seedVisMessage(t, db, "group:gm-ops", "report-agg", "incident_report", nil) // aggregate form
	seedVisMessage(t, db, "alice", "to-alice", "", nil)

	got := listTitles(t, visibilityService(db, false), context.Background(), "alice")
	assert.True(t, got["broadcast"], "fallback broadcast must not fall into a void")
	assert.True(t, got["report-agg"])
	assert.True(t, got["to-alice"])
}

func TestServiceListVisibility_CategoryScopeAudience(t *testing.T) {
	db := newVisibilityTestDB(t)
	require.NoError(t, db.Create(&model.IncidentCategory{
		Name: "客户端", Slug: "client", Leader: "zhu", Enabled: true,
		Audience: model.JSON(`{"users":["alice"],"roles":["qa"]}`),
	}).Error)
	// Role-audience user bob: qa role
	require.NoError(t, db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.AdminRole{}))
	require.NoError(t, db.Create(&model.Role{Name: "qa"}).Error)
	var qaRole model.Role
	require.NoError(t, db.Where("name = ?", "qa").First(&qaRole).Error)
	require.NoError(t, db.Create(&model.Admin{Username: "bob", Nickname: "Bob", Status: 1}).Error)
	var bob model.Admin
	require.NoError(t, db.Where("username = ?", "bob").First(&bob).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: bob.ID, RoleID: qaRole.ID}).Error)
	ctxBob := context.WithValue(context.Background(), "username", "bob")

	// leader slice: recipient=zhu, scope=client
	seedVisMessage(t, db, "zhu", "client-slice", "incident_report", model.JSON(`{"categories":["client"]}`))
	// Another category's slice (control group)
	seedVisMessage(t, db, "qa-lead", "qa-slice", "incident_report", model.JSON(`{"categories":["qa"]}`))

	// leader sees their own slice
	zhuGot := listTitles(t, visibilityService(db, false), context.Background(), "zhu")
	assert.True(t, zhuGot["client-slice"], "leader must see own slice")
	// audience.users can see it
	aliceGot := listTitles(t, visibilityService(db, false), context.Background(), "alice")
	assert.True(t, aliceGot["client-slice"], "audience user must see the slice")
	assert.False(t, aliceGot["qa-slice"], "other category slice stays hidden")
	// Role audience can see it (bob needs AdminModel so LoadCurrentAdmin resolves roles)
	bobGot := listTitles(t, visibilityService(db, true), ctxBob, "bob")
	assert.True(t, bobGot["client-slice"], "audience role must see the slice")
	// An unrelated user cannot see another leader's slice
	carolGot := listTitles(t, visibilityService(db, false), context.Background(), "carol")
	assert.False(t, carolGot["client-slice"], "unrelated user must not see other leader's slice")
}

func TestServiceListVisibility_AdminSeesAll(t *testing.T) {
	db := newVisibilityTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.AdminRole{}))
	require.NoError(t, db.Create(&model.Role{Name: "admin"}).Error)
	var adminRole model.Role
	require.NoError(t, db.Where("name = ?", "admin").First(&adminRole).Error)
	require.NoError(t, db.Create(&model.Admin{Username: "boss", Nickname: "Boss", Status: 1}).Error)
	var boss model.Admin
	require.NoError(t, db.Where("username = ?", "boss").First(&boss).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: boss.ID, RoleID: adminRole.ID}).Error)
	ctxBoss := context.WithValue(context.Background(), "username", "boss")

	seedVisMessage(t, db, "alice", "to-alice", "", nil)
	seedVisMessage(t, db, "bob", "to-bob", "", nil)

	got := listTitles(t, visibilityService(db, true), ctxBoss, "boss")
	assert.True(t, got["to-alice"], "admin sees all")
	assert.True(t, got["to-bob"], "admin sees all")
}

func TestServiceListVisibility_PaginationAfterFilter(t *testing.T) {
	db := newVisibilityTestDB(t)
	require.NoError(t, db.Create(&model.IncidentCategory{
		Name: "客户端", Slug: "client", Leader: "zhu", Enabled: true,
		Audience: model.JSON(`{"users":["alice"]}`),
	}).Error)
	// 3 visible (1 direct + 2 category slices), 2 hidden (other people's direct receipts)
	seedVisMessage(t, db, "alice", "direct", "", nil)
	seedVisMessage(t, db, "zhu", "slice-1", "incident_report", model.JSON(`{"categories":["client"]}`))
	seedVisMessage(t, db, "zhu", "slice-2", "incident_report", model.JSON(`{"categories":["client"]}`))
	seedVisMessage(t, db, "bob", "hidden-1", "", nil)
	seedVisMessage(t, db, "carol", "hidden-2", "", nil)

	s := visibilityService(db, false)
	resp, err := s.List(context.Background(), "alice", &MessagesListRequest{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), resp.Total, "total counts filtered set")
	require.Len(t, resp.Items, 2)
	resp2, err := s.List(context.Background(), "alice", &MessagesListRequest{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), resp2.Total)
	require.Len(t, resp2.Items, 1)
}
