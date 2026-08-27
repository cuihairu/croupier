package hotpatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newFixtureWithDB 与 newFixture 相同，但暴露原始 gorm.DB 以便破坏表结构
// 触发模型层错误分支。
func newFixtureWithDB(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:hp_extra_%d?mode=memory&cache=shared", hpSeq+1)), &gorm.Config{})
	require.NoError(t, err)
	hpSeq++
	require.NoError(t, model.AutoMigrate(db))
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	return NewService(&svc.ServiceContext{
		HotpatchModel: model.NewHotpatchModel(db),
		ObjectStore:   store,
	}), db
}

func TestService_ListFiltersAndPagination(t *testing.T) {
	ctx := context.Background()
	svcSrv, _ := newFixtureWithDB(t)

	for _, spec := range []struct {
		game, env, framework, status string
	}{
		{"demo", "prod", "skynet", "draft"},
		{"demo", "stage", "jvm", "draft"},
		{"other", "prod", "skynet", "draft"},
	} {
		_, err := svcSrv.Create(ctx, &CreateRequest{
			GameID: spec.game, Env: spec.env, Framework: spec.framework,
			BugID: 7, Title: "t",
		})
		require.NoError(t, err)
	}

	// gameId + env 过滤。
	list, err := svcSrv.List(ctx, &ListRequest{GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, list.Total)
	assert.Equal(t, "skynet", list.Items[0].Framework)

	// framework 大小写归一。
	list, err = svcSrv.List(ctx, &ListRequest{Framework: " JVM "})
	require.NoError(t, err)
	assert.EqualValues(t, 1, list.Total)

	// status 过滤。
	list, err = svcSrv.List(ctx, &ListRequest{Status: "draft"})
	require.NoError(t, err)
	assert.EqualValues(t, 3, list.Total)

	// 分页越界 → 空列表。
	list, err = svcSrv.List(ctx, &ListRequest{Page: 5, PageSize: 2})
	require.NoError(t, err)
	assert.Empty(t, list.Items)
	assert.EqualValues(t, 3, list.Total)

	// 模型层错误（表被删）。
	svcSrv2, db := newFixtureWithDB(t)
	require.NoError(t, db.Migrator().DropTable("hotpatches"))
	_, err = svcSrv2.List(ctx, &ListRequest{})
	require.Error(t, err)
}

func TestService_CreateFieldsAndUsername(t *testing.T) {
	ctx := context.WithValue(context.Background(), "username", "alice")
	svcSrv, _ := newFixtureWithDB(t)

	res, err := svcSrv.Create(ctx, &CreateRequest{
		GameID: "demo", Env: "prod", Framework: " Skynet ",
		Targets:   []string{"node-a", "node-b"},
		EntrySpec: map[string]interface{}{"entryFile": "hotfix.lua"},
		BugID:     9, Title: " 修复登录 ",
	})
	require.NoError(t, err)
	assert.Equal(t, "skynet", res.Framework, "framework 应小写归一")
	assert.Equal(t, "alice", res.CreatedBy)
	assert.Equal(t, []string{"node-a", "node-b"}, res.Targets)
	assert.Equal(t, map[string]interface{}{"entryFile": "hotfix.lua"}, res.EntrySpec)
	assert.Equal(t, model.HotpatchStatusDraft, res.Status)
	assert.NotEmpty(t, res.CreatedAt)

	// 无 username 的 ctx → CreatedBy 回落 system。
	res, err = svcSrv.Create(context.Background(), &CreateRequest{
		GameID: "demo", Framework: "skynet", BugID: 1, Title: "x",
	})
	require.NoError(t, err)
	assert.Equal(t, "system", res.CreatedBy)

	// 标题仅空白 → 400。
	_, err = svcSrv.Create(ctx, &CreateRequest{Framework: "skynet", BugID: 1, Title: "   "})
	require.ErrorContains(t, err, "标题")

	// 模型层错误（表被删）。
	svcSrv2, db := newFixtureWithDB(t)
	require.NoError(t, db.Migrator().DropTable("hotpatches"))
	_, err = svcSrv2.Create(ctx, &CreateRequest{Framework: "skynet", BugID: 1, Title: "x"})
	require.Error(t, err)
}

func TestService_UploadPackageErrorBranches(t *testing.T) {
	ctx := context.Background()
	svcSrv, _ := newFixtureWithDB(t)
	hp := seedDraft(t, svcSrv)
	id := fmt.Sprint(hp.Id)

	// id 非法形态。
	for _, bad := range []string{"", "abc", "0", "-1"} {
		_, err := svcSrv.UploadPackage(ctx, &UploadRequest{ID: bad, Data: nil, Size: 1})
		require.Error(t, err, bad)
	}

	// 不存在的 id → gorm ErrRecordNotFound。
	_, err := svcSrv.UploadPackage(ctx, &UploadRequest{ID: "999999", Data: nil, Size: 1})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// 对象存储未配置。
	noStore := NewService(&svc.ServiceContext{HotpatchModel: svcSrv.svcCtx.HotpatchModel})
	_, err = noStore.UploadPackage(ctx, &UploadRequest{ID: id, Data: nil, Size: 1})
	require.ErrorContains(t, err, "对象存储")

	// Put 失败：在 base/hotpatches 放同名文件阻断 MkdirAll。
	blockingFixture, _ := newFixtureWithDB(t)
	blocked := seedDraft(t, blockingFixture)
	base := t.TempDir()
	blockedStore, err := objstore.OpenFile(ctx, objstore.Config{BaseDir: base})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(base, "hotpatches"), nil, 0o644))
	blockedSvc := NewService(&svc.ServiceContext{
		HotpatchModel: blockingFixture.svcCtx.HotpatchModel,
		ObjectStore:   blockedStore,
	})
	_, err = blockedSvc.UploadPackage(ctx, &UploadRequest{
		ID: fmt.Sprint(blocked.Id), Data: os.NewFile(0, ""), Size: 1,
	})
	require.ErrorContains(t, err, "上传对象存储失败")
}

func TestService_TransitionIDAndActionValidation(t *testing.T) {
	ctx := context.Background()
	svcSrv, _ := newFixtureWithDB(t)

	for _, bad := range []string{"", "abc", "0"} {
		_, err := svcSrv.Transition(ctx, &TransitionRequest{ID: bad, Action: "approve"})
		require.Error(t, err, bad)
	}
	hp := seedDraft(t, svcSrv)
	_, err := svcSrv.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "unknown"})
	require.ErrorContains(t, err, "无效的操作")
}

func TestService_ReportResultNotFound(t *testing.T) {
	ctx := context.Background()
	svcSrv, _ := newFixtureWithDB(t)
	err := svcSrv.ReportResult(ctx, &ResultRequest{ID: "999999", AgentID: "a", Status: "ok"})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestStreamReader_Seek(t *testing.T) {
	s := &streamReader{}
	n, err := s.Seek(10, 0)
	assert.EqualValues(t, 0, n)
	assert.NoError(t, err)
}
