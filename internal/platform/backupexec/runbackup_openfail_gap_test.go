package backupexec

// runbackup_openfail_gap_test.go 补齐 Executor.RunBackup 的 os.Open
// 失败分支（executor.go RunBackup 打开 dump 产物处）：dump 成功但打开
// 产物失败时必须落库 status=failed 且返回错误。正常路径下 dump 刚
// CreateTemp/写内容/Stat 成功，os.Open 同一临时文件无现实失败方式，
// 经 openFile 注入口触达（与 execCommand 同款测试替身模式）。

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

func TestRunBackup_OpenDumpFails(t *testing.T) {
	m, db := newBackupTestModel(t)
	createBackupRow(t, m, "bk-openfail")

	dir := t.TempDir()
	src := filepath.Join(dir, "game.db")
	require.NoError(t, os.WriteFile(src, []byte("sqlite-payload"), 0o600))

	store := newFakeObjStore(nil)
	e := New("sqlite", src, m, store, "backups/")
	openErr := errors.New("open dump file failed")
	e.openFile = func(name string) (*os.File, error) { return nil, openErr }

	err := e.RunBackup(context.Background(), "bk-openfail", "daily", "full")
	require.ErrorIs(t, err, openErr, "open failure must propagate")

	// 打开失败也必须落库 failed，且对象存储不应收到任何上传。
	var got model.Backup
	require.NoError(t, db.Where("backup_id = ?", "bk-openfail").First(&got).Error)
	assert.Equal(t, "failed", got.Status)
	assert.Empty(t, got.Location)
	assert.Contains(t, got.ErrorMessage, "open dump file failed")
	assert.Nil(t, got.CompletedAt, "failed backup must not be marked completed")
	assert.Empty(t, store.putKeys, "no object may be uploaded when open fails")
}
