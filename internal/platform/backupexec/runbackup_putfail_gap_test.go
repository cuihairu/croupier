package backupexec

// runbackup_putfail_gap_test.go 补齐 Executor.RunBackup 的对象存储
// Put 失败分支：dump 成功但上传失败时必须落库 status=failed 且返回错误。
//
// 本包其余未满分支：无（open 失败分支见 runbackup_openfail_gap_test.go）。

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
)

func TestRunBackup_StorePutFails_I(t *testing.T) {
	m, db := newBackupTestModel(t)
	createBackupRow(t, m, "bk-putfail")

	dir := t.TempDir()
	src := filepath.Join(dir, "game.db")
	require.NoError(t, os.WriteFile(src, []byte("sqlite-payload"), 0o600))

	putErr := errors.New("object store unavailable")
	store := newFakeObjStore(putErr)
	e := New("sqlite", src, m, store, "backups/")

	err := e.RunBackup(context.Background(), "bk-putfail", "daily", "full")
	require.ErrorIs(t, err, putErr, "Put failure must propagate")

	// 失败必须落库：status=failed、ErrorMessage 带原因、无 Location；
	// completed_at 仅在 succeeded 时写入，失败路径保持空。
	var got model.Backup
	require.NoError(t, db.Where("backup_id = ?", "bk-putfail").First(&got).Error)
	assert.Equal(t, "failed", got.Status)
	assert.Empty(t, got.Location)
	assert.Equal(t, int64(0), got.Size)
	assert.Contains(t, got.ErrorMessage, "object store unavailable")
	assert.Nil(t, got.CompletedAt, "failed backup must not be marked completed")
}
