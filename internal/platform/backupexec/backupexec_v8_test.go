// 覆盖目标：finish 落库失败告警、CreateTemp 失败（TMPDIR 不可写）、dump 中
// 临时文件被外部删除导致 Stat 失败、dumpSQLite 源为目录时 ReadFrom 失败、
// fileSHA256 对目录的 WriteTo 失败。
package backupexec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// finish：UpdateByBackupID 失败 → 仅记录告警，不影响返回链路。
func TestRunBackup_FinishUpdateFailureWarns_V8(t *testing.T) {
	m, db := newBackupTestModel(t)
	createBackupRow(t, m, "bk-finish-fail")

	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("v8:update_err", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "backups" {
			_ = tx.AddError(errors.New("injected update failure"))
		}
	}))

	// dump 阶段失败 → finish 走 failed 更新 → Update 报错被吞掉并告警。
	e := New("mysql", "garbage", m, newFakeObjStore(nil), "")
	err := e.RunBackup(context.Background(), "bk-finish-fail", "n", "full")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dump failed")
}

// dump：CreateTemp 失败（TMPDIR 指向不存在的目录）。
func TestDump_CreateTempFailure_V8(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "no-such-dir"))
	e := New("sqlite", "whatever", nil, nil, "")
	_, _, _, err := e.dump(context.Background(), "bk-tmp-fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no-such-dir")
}

// dump：命令成功但输出文件被删除 → Stat 失败。
func TestDump_StatFailureAfterCommand_V8(t *testing.T) {
	e := New("mysql", "u:pw@tcp(127.0.0.1:3306)/db", nil, nil, "")
	e.execCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		for _, a := range args {
			if v, ok := stringsCutPrefix(a, "--result-file="); ok {
				// 模拟外部把导出文件删掉。
				_ = os.Remove(v)
			}
		}
		return nil, nil
	}
	_, _, _, err := e.dump(context.Background(), "bk-stat-fail")
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// dumpSQLite：数据源路径是目录 → ReadFrom 读目录失败。
func TestDumpSQLite_SourceIsDir_V8(t *testing.T) {
	e := New("sqlite", t.TempDir(), nil, nil, "")
	_, _, _, err := e.dump(context.Background(), "bk-dir-src")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}

// fileSHA256：路径是目录 → WriteTo 读目录失败。
func TestFileSHA256_DirWriteToFailure_V8(t *testing.T) {
	_, err := fileSHA256(t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}
