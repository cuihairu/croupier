package backupexec

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dumpPostgres 的 os.Create(out) 失败分支（executor.go:163）：
// 直接以父目录不存在的 out 路径调用未导出方法（DSN 合法、
// CreateTemp 环节被绕过），得到确定性的 ENOENT。
func TestDumpPostgres_CreateOutputFileFailure(t *testing.T) {
	e := New("postgres", "host=127.0.0.1 port=5432 user=postgres dbname=demo", nil, nil, "")
	out := filepath.Join(t.TempDir(), "missing-dir", "out.sql")

	err := e.dumpPostgres(context.Background(), out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

// dumpSQLite 的输出目标不可创建分支（AUD-009 后为 VACUUM INTO 的写侧失败）：
// 源库真实存在，但 out 父目录不存在 → SQLite 无法打开目标库 → 确定性失败，
// 绝不留下「空库冒充成功」的产物。
func TestDumpSQLite_OutputTargetUnwritable(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.db")
	writeSQLiteDB(t, src)

	e := New("sqlite", src, nil, nil, "")
	out := filepath.Join(t.TempDir(), "missing-dir", "out.sqlite")

	err := e.dumpSQLite(context.Background(), out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to open database")
}
