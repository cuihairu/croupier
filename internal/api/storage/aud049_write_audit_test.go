package storage

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aud049StorageEnv 组装带审计内存存储的对象存储服务（AUD-049）。
func aud049StorageEnv(t *testing.T) (*Service, *audit.InMemoryAuditStore) {
	t.Helper()
	store := audit.NewInMemoryAuditStore()
	svcCtx := identitySvcCtx(t, &failingStore{})
	svcCtx.AuditService = audit.NewAuditService(store, nil)
	return NewService(svcCtx), store
}

// aud049StorageRecords 取同事件类型的全部留痕。注意 InMemoryAuditStore.List
// 底层是 map（遍历顺序随机），断言一律按 outcome/operation 子集过滤，不依赖
// 顺序。
func aud049StorageRecords(t *testing.T, store *audit.InMemoryAuditStore, event audit.AuditEventType) []*audit.AuditRecord {
	t.Helper()
	records, _, err := store.List(audit.AuditFilter{EventType: []audit.AuditEventType{event}}, audit.AuditPage{Page: 1, PageSize: 50})
	require.NoError(t, err)
	return records
}

func aud049ByOutcome(records []*audit.AuditRecord, outcome string) []*audit.AuditRecord {
	var out []*audit.AuditRecord
	for _, r := range records {
		if r.Outcome == outcome {
			out = append(out, r)
		}
	}
	return out
}

func aud049ByOperation(records []*audit.AuditRecord, operation string) []*audit.AuditRecord {
	var out []*audit.AuditRecord
	for _, r := range records {
		if op, _ := r.Details["operation"].(string); op == operation {
			out = append(out, r)
		}
	}
	return out
}

// 上传/删除/批量删除/目录变更成功留痕；零角色账号与无用户名上下文被权限
// 门禁拒绝时同事件类型落 denied。
func TestAud049_StorageWriteAudit(t *testing.T) {
	s, store := aud049StorageEnv(t)

	_, err := s.UploadObject(authCtx(), &UploadObjectRequest{Path: "aud049/a.txt", Content: "hello"})
	require.NoError(t, err)
	_, err = s.UploadObject(viewerCtx(), &UploadObjectRequest{Path: "aud049/b.txt", Content: "x"})
	require.Error(t, err)

	uploads := aud049StorageRecords(t, store, audit.EventStorageUpload)
	require.Len(t, uploads, 2)
	okUploads := aud049ByOutcome(uploads, "success")
	require.Len(t, okUploads, 1)
	assert.Equal(t, "upload", okUploads[0].Details["operation"])
	assert.Equal(t, "aud049/a.txt", okUploads[0].Resource.ID)
	assert.NotEmpty(t, okUploads[0].Actor.ID, "成功路径应携带操作者身份")
	deniedUploads := aud049ByOutcome(uploads, "denied")
	require.Len(t, deniedUploads, 1)
	assert.Equal(t, "upload", deniedUploads[0].Details["operation"])
	assert.Equal(t, "aud049/b.txt", deniedUploads[0].Resource.ID)

	_, err = s.DeleteObject(authCtx(), &DeleteObjectRequest{Path: "aud049/a.txt"})
	require.NoError(t, err)
	resp, err := s.BatchDeleteObjects(authCtx(), &BatchDeleteObjectsRequest{Paths: []string{"aud049/x.txt", "aud049/y.txt"}})
	require.NoError(t, err)
	require.Len(t, resp.Deleted, 2)
	_, err = s.BatchDeleteObjects(viewerCtx(), &BatchDeleteObjectsRequest{Paths: []string{"aud049/z.txt"}})
	require.Error(t, err)

	deletes := aud049StorageRecords(t, store, audit.EventStorageDelete)
	require.Len(t, deletes, 3)
	okDeletes := aud049ByOutcome(deletes, "success")
	require.Len(t, okDeletes, 2)
	require.Len(t, aud049ByOperation(okDeletes, "delete"), 1)
	batchDeletes := aud049ByOperation(okDeletes, "batch_delete")
	require.Len(t, batchDeletes, 1)
	assert.Equal(t, 2, batchDeletes[0].Details["deleted"])
	deniedDeletes := aud049ByOutcome(deletes, "denied")
	require.Len(t, deniedDeletes, 1)
	assert.Equal(t, "batch_delete", deniedDeletes[0].Details["operation"])

	_, err = s.CreateDirectory(authCtx(), &CreateDirectoryRequest{Prefix: "aud049/dir/"})
	require.NoError(t, err)
	_, err = s.RenameDirectory(authCtx(), &RenameDirectoryRequest{OldPrefix: "aud049/dir/", NewPrefix: "aud049/dir2/"})
	require.NoError(t, err)
	dirs := aud049StorageRecords(t, store, audit.EventStorageDirectory)
	require.Len(t, dirs, 2)
	require.Len(t, aud049ByOperation(dirs, "directory_create"), 1)
	require.Len(t, aud049ByOperation(dirs, "directory_rename"), 1)
}

// ctx 无用户名（外部 token 面）：权限门禁拒绝落 denied，actor 留空但
// 事件类型 + path 仍可回查。
func TestAud049_StorageAuditDeniedWithoutUsername(t *testing.T) {
	s, store := aud049StorageEnv(t)

	_, err := s.UploadObject(context.Background(), &UploadObjectRequest{Path: "aud049/anon.txt", Content: "x"})
	require.Error(t, err)
	uploads := aud049StorageRecords(t, store, audit.EventStorageUpload)
	require.Len(t, uploads, 1)
	assert.Equal(t, "denied", uploads[0].Outcome)
	assert.Empty(t, uploads[0].Actor.ID)
	assert.Equal(t, "aud049/anon.txt", uploads[0].Resource.ID)
}
