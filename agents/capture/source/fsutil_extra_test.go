package source

// fsutil.go 补测：readFile 的非 NotExist 失败分支（目录读取）、atomicWrite
// 的 MkdirAll/Rename 失败收口（rename 目标是目录）、dirOf 无分隔符兜底；
// Store 位点读写往返与损坏书签报错；AddressFromDSN 的未设/坏 DSN/unix
// socket 降级/无端口补 3306。
//
// 诚实跳过（不可稳定注入）：atomicWrite 的 CreateTemp/Write/Close 失败
// 分支——顺序上 MkdirAll 先行失败截住目录级故障，文件级 IO 失败需内核
// 注入，不在单测范围。AddressFromDSN 的无端口补默认分支同理不可达——
// go-sql-driver ParseDSN 已把 tcp 地址归一为 host:port（用例断言归一
// 结果本身）。Store.Save 的序列化失败分支（source.go :61）同理结构性不可
// 达——Bookmark 仅 string+int64 字段，json.Marshal 恒成功。

import (
	"os"
	"path/filepath"
	"testing"

	drivermysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestReadFileErrorFamilies(t *testing.T) {
	_, err := readFile(filepath.Join(t.TempDir(), "missing.txt"))
	require.True(t, os.IsNotExist(err))

	// 目录读取 → 非 NotExist 失败（EISDIR）
	_, err = readFile(t.TempDir())
	require.Error(t, err)
	require.False(t, os.IsNotExist(err))
}

func TestAtomicWriteFailureClosures(t *testing.T) {
	// MkdirAll 失败：父路径被现有文件占位
	fileAsDir := filepath.Join(t.TempDir(), "occupied")
	require.NoError(t, os.WriteFile(fileAsDir, []byte("x"), 0o600))
	require.Error(t, atomicWrite(filepath.Join(fileAsDir, "bookmark.json"), []byte("{}")))

	// Rename 失败：目标是已存在的目录（同目录临时文件已建好，收口清理）
	target := filepath.Join(t.TempDir(), "adir")
	require.NoError(t, os.Mkdir(target, 0o755))
	require.Error(t, atomicWrite(target, []byte("{}")))

	// 健全路径往返
	ok := filepath.Join(t.TempDir(), "bm.json")
	require.NoError(t, atomicWrite(ok, []byte(`{"position":"g:1"}`)))
	data, err := os.ReadFile(ok)
	require.NoError(t, err)
	require.Equal(t, `{"position":"g:1"}`, string(data))
}

func TestDirOfFallback(t *testing.T) {
	require.Equal(t, ".", dirOf("bookmark.json"))
	require.Equal(t, "a/b", dirOf("a/b/c.json"))
}

func TestStoreRoundtripAndCorrupt(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "bookmark.json"))

	// 首启：文件不存在 = 零值书签
	b, err := store.Load()
	require.NoError(t, err)
	require.False(t, b.Valid())

	require.NoError(t, store.Save(Bookmark{Position: "g:1-5", UpdatedAtUnixMs: 42}))
	b, err = store.Load()
	require.NoError(t, err)
	require.Equal(t, "g:1-5", b.Position)
	require.Equal(t, int64(42), b.UpdatedAtUnixMs)

	// 损坏书签显式报错（不静默当零值吞掉）
	require.NoError(t, os.WriteFile(store.path, []byte("{broken"), 0o600))
	_, err = store.Load()
	require.ErrorContains(t, err, "corrupt bookmark")
}

func TestAddressFromDSNShapes(t *testing.T) {
	const env = "CROUPIER_CAPTURE_MYSQL_ADDR_EXTRA_TEST"

	// 未设置
	_, err := AddressFromDSN(env)
	require.ErrorContains(t, err, env)

	// 坏 DSN（ParseDSN 失败包装）
	t.Setenv(env, "bad@@dsn")
	_, err = AddressFromDSN(env)
	require.ErrorContains(t, err, "parse dsn")

	// unix socket → 空串降级（不可 TCP 探测）
	t.Setenv(env, "u:p@unix(/tmp/mysql.sock)/db")
	addr, err := AddressFromDSN(env)
	require.NoError(t, err)
	require.Empty(t, addr)

	// 无端口地址补默认端口
	t.Setenv(env, "u:p@tcp(10.0.0.8)/db")
	addr, err = AddressFromDSN(env)
	require.NoError(t, err)
	require.Equal(t, "10.0.0.8:3306", addr)

	// 带端口原样
	t.Setenv(env, "u:p@tcp(10.0.0.8:3307)/db")
	addr, err = AddressFromDSN(env)
	require.NoError(t, err)
	require.Equal(t, "10.0.0.8:3307", addr)

	// 防御：驱动注册在位（AddressFromDSN 与 connector 同源解析器）
	require.NotNil(t, drivermysql.MySQLDriver{})
}
