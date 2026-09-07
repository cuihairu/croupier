package backupexec

import (
	"context"
	"os"
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

// dumpSQLite 的 os.Create(out) 失败分支（executor.go:197）：
// 源文件存在（os.Open(e.dsn) 成功），out 父目录不存在 → 创建失败。
func TestDumpSQLite_CreateOutputFileFailure(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.db")
	require.NoError(t, os.WriteFile(src, []byte("sqlite-bytes"), 0o600))

	e := New("sqlite", src, nil, nil, "")
	out := filepath.Join(t.TempDir(), "missing-dir", "out.sql")

	err := e.dumpSQLite(context.Background(), out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}
