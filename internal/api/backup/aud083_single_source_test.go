// AUD-083 regression: the backup list used to prefer the legacy extension-mirror
// snapshot (config key "backups" of the official.backup-advanced installation)
// while the executor advanced database rows, so a created backup stayed pending
// in the list forever and scheduler/ops backups (database-only writers) were
// invisible. The database backup row is now the single record source of truth:
//   - list reflects the executor-written database status (real async run,
//     deterministically held in pending until the store is released);
//   - download/delete key on the database row only: mirror entries can neither
//     satisfy identity nor override database state;
//   - underlying database errors surface on list (legacy import, drain) instead
//     of being swallowed;
//   - legacy mirror-only entries migrate into the database one-way and
//     idempotently; the snapshot drains, so deleting an imported row cannot
//     resurrect it.
package backup

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aud083BlockingPutStore holds the executor at the object-store upload so the
// async run cannot advance the record until the test releases it: the list is
// then deterministically read while the row is still pending.
type aud083BlockingPutStore struct {
	execObjStoreStub
	release chan struct{}
	once    sync.Once
}

func (s *aud083BlockingPutStore) Put(ctx context.Context, key string, r objstore.ReadSeeker, size int64, contentType string) error {
	<-s.release
	return s.execObjStoreStub.Put(ctx, key, r, size, contentType)
}

func (s *aud083BlockingPutStore) Release() {
	s.once.Do(func() { close(s.release) })
}

// (a) with the extension installed and a stale mirror, create -> list reads the
// database row's pending -> succeeded after the executor's real async run.
func TestAud083_ListReadsExecutorWrittenDBStatus(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	// Stale legacy mirror: the snapshot claims a world without the new record.
	installBackupAdvancedExtension(t, env, map[string]any{"backups": []Backup{
		{
			Id:        "aud083-legacy-completed",
			Name:      "legacy",
			Type:      "full",
			Status:    "completed",
			CreatedAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
		},
	}})

	store := &aud083BlockingPutStore{release: make(chan struct{})}
	env.svcCtx.ObjectStore = store
	defer store.Release()

	resp, err := env.service.Create(ctx, &BackupCreateRequest{Name: "aud083-live", Type: "full"})
	require.NoError(t, err, "执行器可用时创建必须照常接受")
	require.Equal(t, "pending", resp.Status)

	// The list is served while the run is parked in the upload: the created id
	// must appear (the mirror no longer hides database rows) with the database
	// row's pending status, next to the migrated legacy history.
	listed, err := env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Items, 2)
	byID := map[string]Backup{}
	for _, item := range listed.Items {
		byID[item.Id] = item
	}
	live, ok := byID[resp.Id]
	require.True(t, ok, "新建备份必须出现在列表中（真源为数据库行，镜像不得遮蔽）")
	assert.Equal(t, "pending", live.Status, "执行器未推进前列表必须读出数据库行的 pending")
	assert.Equal(t, "completed", byID["aud083-legacy-completed"].Status, "镜像历史经一次性迁移后同列展示")

	// The migrated legacy entry was drained from the snapshot and now lives in
	// the database only.
	items, mirrorOK, err := env.service.loadBackupsFromExtensionInstallation(ctx)
	require.NoError(t, err)
	require.False(t, mirrorOK, "迁移完成后镜像快照必须被清空")
	require.Empty(t, items)

	// The executor's real async run advances the database row, and the same list
	// path now reports succeeded — one source of truth end to end.
	store.Release()
	waitBackupStatus(t, env, resp.Id, "succeeded")

	listed, err = env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Items, 2)
	byID = map[string]Backup{}
	for _, item := range listed.Items {
		byID[item.Id] = item
	}
	assert.Equal(t, "succeeded", byID[resp.Id].Status, "列表必须反映执行器写回的数据库状态")
	row, err := env.svcCtx.BackupModel.FindByBackupID(ctx, resp.Id)
	require.NoError(t, err)
	assert.Equal(t, row.Status, byID[resp.Id].Status, "列表状态必须与数据库行一致")
	assert.NotEmpty(t, row.Location)
}

// (b) download/delete use the database record identity: a mirror-only entry can
// neither be downloaded nor deleted, and the database state (not any mirror
// copy) decides whether a download payload exists.
func TestAud083_DownloadAndDeleteKeyOnDBRowOnly(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	installBackupAdvancedExtension(t, env, map[string]any{"backups": []Backup{
		{Id: "aud083-phantom", Name: "phantom", Type: "full", Status: "succeeded"},
	}})
	// A live database row still pending: no downloadable object exists yet, no
	// matter what any historical mirror copy claimed.
	createBackupRow(t, env, &model.Backup{BackupID: "aud083-dbrow", Name: "dbrow", Type: "full", Status: "pending"})
	// A completed database row with a redirect-style location.
	createBackupRow(t, env, &model.Backup{BackupID: "aud083-dl", Name: "dl", Type: "full", Status: "succeeded",
		Location: "https://storage.example.com/backups/aud083-dl.tar"})

	_, err := env.service.Download(ctx, &BackupDownloadRequest{ID: "aud083-phantom"})
	require.Error(t, err, "镜像独有记录不得满足下载身份")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code)

	err = env.service.Delete(ctx, &BackupDeleteRequest{ID: "aud083-phantom"})
	require.Error(t, err, "镜像独有记录不得满足删除身份")
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code)

	_, err = env.service.Download(ctx, &BackupDownloadRequest{ID: "aud083-dbrow"})
	require.Error(t, err, "数据库行未生成文件时下载必须拒绝")
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Contains(t, err.Error(), "备份尚未生成可下载文件")

	payload, err := env.service.Download(ctx, &BackupDownloadRequest{ID: "aud083-dl"})
	require.NoError(t, err)
	assert.Equal(t, "https://storage.example.com/backups/aud083-dl.tar", payload.RedirectURL)

	require.NoError(t, env.service.Delete(ctx, &BackupDeleteRequest{ID: "aud083-dl"}))
	_, err = env.svcCtx.BackupModel.FindByBackupID(ctx, "aud083-dl")
	require.Error(t, err, "删除必须移除数据库行")
	_, err = env.service.Download(ctx, &BackupDownloadRequest{ID: "aud083-dl"})
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code, "删除后下载必须 404（身份真源已消失）")

	// The mirror-only phantom is not silently dropped either: the documented
	// migration story brings it into the database on the next list.
	listed, err := env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, item := range listed.Items {
		ids[item.Id] = true
	}
	assert.True(t, ids["aud083-phantom"], "镜像独有历史必须经一次性迁移进入数据库")
	assert.False(t, ids["aud083-dl"], "已删除的备份不得出现在列表中")
}

// (c) underlying database errors are observable on the list path: the legacy
// import surfaces lookup/insert failures and the snapshot drain surfaces
// extension-config write failures instead of swallowing them.
func TestAud083_LegacyMirrorDBErrorsSurface(t *testing.T) {
	t.Run("镜像迁移的数据库查询错误必须上抛", func(t *testing.T) {
		env := newExecTestEnv(t, "sqlite", "/dev/null", &execObjStoreStub{})
		installBackupAdvancedExtension(t, env, map[string]any{"backups": []Backup{
			{Id: "aud083-err", Name: "err", Type: "full", Status: "completed"},
		}})
		require.NoError(t, env.db.Migrator().DropTable("backups"))

		_, err := env.service.List(env.authCtx(t), &BackupsListRequest{})
		require.Error(t, err, "迁移查询失败不得被吞掉")
	})

	t.Run("镜像清空（drain）写失败必须上抛", func(t *testing.T) {
		env := newExecTestEnv(t, "sqlite", "/dev/null", &execObjStoreStub{})
		installBackupAdvancedExtension(t, env, map[string]any{"backups": []Backup{
			{Id: "aud083-drain", Name: "drain", Type: "full", Status: "completed"},
		}})
		injectBackupCallback(t, env.db, "update")

		_, err := env.service.List(env.authCtx(t), &BackupsListRequest{})
		require.Error(t, err, "迁移后的镜像清空写失败不得被吞掉")
	})
}

// (d) legacy mirror-only entries migrate once: fields are preserved, the import
// is idempotent, unusable-id entries stay put, and already-imported ids are not
// duplicated.
func TestAud083_LegacyMirrorOnlyEntriesMigrateOnce(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	oldCreated := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	installBackupAdvancedExtension(t, env, map[string]any{"backups": []Backup{
		{Id: "aud083-mig-completed", Name: "mig-completed", Type: " FULL ", Status: "completed",
			Size: 123, CreatedAt: oldCreated},
		{Id: "aud083-mig-noname", Status: "succeeded", CreatedAt: oldCreated},
		{Id: "   ", Name: "ghost-no-id", Type: "full", Status: "completed"},
	}})

	listed, err := env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Items, 2, "无可用 id 的镜像条目不可迁移也不可列出")
	byID := map[string]Backup{}
	for _, item := range listed.Items {
		byID[item.Id] = item
	}
	require.Contains(t, byID, "aud083-mig-completed")
	assert.Equal(t, "full", byID["aud083-mig-completed"].Type, "迁移归一类型大小写")
	assert.Equal(t, int64(123), byID["aud083-mig-completed"].Size)
	require.Contains(t, byID, "aud083-mig-noname")
	assert.Equal(t, "aud083-mig-noname", byID["aud083-mig-noname"].Name, "缺名字的镜像条目以 id 兜底")

	// Field-level migration fidelity on the database rows.
	row, err := env.svcCtx.BackupModel.FindByBackupID(ctx, "aud083-mig-completed")
	require.NoError(t, err)
	assert.Equal(t, "completed", row.Status)
	assert.WithinDuration(t, time.Now().Add(-2*time.Hour), row.CreatedAt, 5*time.Minute, "迁移保留原 createdAt")

	// The snapshot drained to the unusable leftovers only, and a second list is
	// idempotent: no duplicates, no resurrection.
	items, mirrorOK, err := env.service.loadBackupsFromExtensionInstallation(ctx)
	require.NoError(t, err)
	require.True(t, mirrorOK)
	require.Len(t, items, 1)
	assert.Equal(t, "   ", items[0].Id, "无可用 id 的条目按文档边界原样保留")

	listed, err = env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Items, 2, "重复读取不得产生重复迁移")
	assert.Equal(t, int64(2), listed.Total)
}
