// 覆盖目标：RunBackup 成功/失败落库链路（finish）、dumpPostgres 参数装配与真实子进程
// 路径（伪造 PATH 脚本）、run 未注入替身分支、DSN 解析边界（orDefault/hiddenPassword/
// URL 解析失败/key=value 容错）、fileSHA256/dumpSQLite 错误分支。
package backupexec

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- 测试替身 ----

// fakeObjStore 实现 objstore.Store 全接口，可注入 Put 错误。
type fakeObjStore struct {
	mu       sync.Mutex
	putErr   error
	putKeys  []string
	putSizes []int64
}

func newFakeObjStore(putErr error) *fakeObjStore {
	return &fakeObjStore{putErr: putErr}
}

func (f *fakeObjStore) Put(ctx context.Context, key string, r objstore.ReadSeeker, size int64, contentType string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putKeys = append(f.putKeys, key)
	f.putSizes = append(f.putSizes, size)
	return f.putErr
}

func (f *fakeObjStore) SignedURL(ctx context.Context, key string, method string, expiry time.Duration) (string, error) {
	return "", nil
}

func (f *fakeObjStore) Delete(ctx context.Context, key string) error { return nil }

func (f *fakeObjStore) List(ctx context.Context, prefix, marker, delimiter string, limit int) (objstore.ListResult, error) {
	return objstore.ListResult{}, nil
}

func (f *fakeObjStore) CreatePrefix(ctx context.Context, prefix string) error { return nil }

func (f *fakeObjStore) RenamePrefix(ctx context.Context, oldPrefix, newPrefix string) error {
	return nil
}

func newBackupTestModel(t *testing.T) (*model.BackupModel, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/backup.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return model.NewBackupModel(db), db
}

func createBackupRow(t *testing.T, m *model.BackupModel, backupID string) {
	t.Helper()
	require.NoError(t, m.Create(context.Background(), &model.Backup{
		BackupID: backupID, Name: "n-" + backupID, Type: "full", Status: "pending",
	}))
}

// ---- RunBackup 全链路 ----

func TestRunBackup_SQLiteSuccess(t *testing.T) {
	m, db := newBackupTestModel(t)
	createBackupRow(t, m, "bk-ok")

	dir := t.TempDir()
	src := filepath.Join(dir, "game.db")
	require.NoError(t, os.WriteFile(src, []byte("sqlite-payload"), 0o600))

	store := newFakeObjStore(nil)
	e := New("sqlite", src, m, store, "backups/")
	require.NoError(t, e.RunBackup(context.Background(), "bk-ok", "daily", "full"))

	// 对象存储收到 sqlite 后缀 key，大小与内容一致。
	require.Len(t, store.putKeys, 1)
	assert.Regexp(t, `^backups/\d{8}/full-bk-ok\.sqlite$`, store.putKeys[0])
	require.Len(t, store.putSizes, 1)
	assert.Equal(t, int64(len("sqlite-payload")), store.putSizes[0])

	var got model.Backup
	require.NoError(t, db.Where("backup_id = ?", "bk-ok").First(&got).Error)
	assert.Equal(t, "succeeded", got.Status)
	assert.Equal(t, store.putKeys[0], got.Location)
	assert.Equal(t, int64(len("sqlite-payload")), got.Size)
	assert.Len(t, got.Checksum, 64)
	assert.Empty(t, got.ErrorMessage)
	require.NotNil(t, got.CompletedAt)
}

func TestRunBackup_NilExecutor(t *testing.T) {
	var e *Executor
	err := e.RunBackup(context.Background(), "bk", "n", "full")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRunBackup_NilBackupModel(t *testing.T) {
	e := New("sqlite", "x", nil, newFakeObjStore(nil), "")
	err := e.RunBackup(context.Background(), "bk", "n", "full")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRunBackup_BackupNotFound(t *testing.T) {
	m, _ := newBackupTestModel(t)
	e := New("sqlite", "whatever", m, newFakeObjStore(nil), "")
	err := e.RunBackup(context.Background(), "bk-missing", "n", "full")
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestRunBackup_DumpFailureMarksFailed(t *testing.T) {
	m, db := newBackupTestModel(t)
	createBackupRow(t, m, "bk-dump-fail")

	e := New("mysql", "garbage-dsn", m, newFakeObjStore(nil), "")
	err := e.RunBackup(context.Background(), "bk-dump-fail", "n", "full")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dump failed")

	var got model.Backup
	require.NoError(t, db.Where("backup_id = ?", "bk-dump-fail").First(&got).Error)
	assert.Equal(t, "failed", got.Status)
	assert.Empty(t, got.Location)
	assert.Contains(t, got.ErrorMessage, "unsupported mysql DSN form")
	assert.Nil(t, got.CompletedAt)
}

func TestRunBackup_StorePutFailureMarksFailed(t *testing.T) {
	m, db := newBackupTestModel(t)
	createBackupRow(t, m, "bk-put-fail")

	dir := t.TempDir()
	src := filepath.Join(dir, "game.db")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0o600))

	putErr := fmt.Errorf("bucket unavailable")
	e := New("sqlite", src, m, newFakeObjStore(putErr), "")
	err := e.RunBackup(context.Background(), "bk-put-fail", "n", "full")
	require.Error(t, err)
	assert.ErrorIs(t, err, putErr)

	var got model.Backup
	require.NoError(t, db.Where("backup_id = ?", "bk-put-fail").First(&got).Error)
	assert.Equal(t, "failed", got.Status)
	assert.Contains(t, got.ErrorMessage, "bucket unavailable")
}

// ---- dumpPostgres ----

func TestDumpPostgres_ArgsAssembled(t *testing.T) {
	e := New("postgres", "postgres://u:pw@pg.host:5433/gamedb", nil, nil, "")
	var gotName string
	var gotArgs []string
	e.execCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		gotName, gotArgs = name, args
		return nil, nil
	}
	_, _, _, err := e.dump(context.Background(), "bk-pg")
	require.NoError(t, err)
	assert.Equal(t, "pg_dump", gotName)
	joined := strings.Join(gotArgs, " ")
	assert.Contains(t, joined, "--host=pg.host")
	assert.Contains(t, joined, "--port=5433")
	assert.Contains(t, joined, "--username=u")
	assert.Contains(t, joined, "--no-owner")
	assert.Contains(t, joined, "gamedb")
}

func TestDumpPostgres_BadDSN(t *testing.T) {
	e := New("pg", "host=only", nil, nil, "")
	_, _, _, err := e.dump(context.Background(), "bk-pg-bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dump failed")
}

func TestDumpPostgres_RealExec(t *testing.T) {
	dir := prepareToolScript(t, "pg_dump", "#!/bin/sh\nprintf 'pgdump-body'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	e := New("postgres", "host=h user=u password=p dbname=d", nil, nil, "")
	path, size, sum, err := e.dump(context.Background(), "bk-pg-real")
	require.NoError(t, err)
	defer os.Remove(path)
	assert.Equal(t, int64(len("pgdump-body")), size)
	assert.Len(t, sum, 64)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "pgdump-body", string(content))
}

func TestDumpMySQL_BadDSN(t *testing.T) {
	e := New("mysql", "no-tcp-here", nil, nil, "")
	_, _, _, err := e.dump(context.Background(), "bk-my-bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported mysql DSN form")
}

// prepareToolScript 在临时目录生成可执行 stub 脚本，返回目录路径。
func prepareToolScript(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(body), 0o755))
	return dir
}

func TestRunBackup_MySQLRealExecScript(t *testing.T) {
	m, db := newBackupTestModel(t)
	createBackupRow(t, m, "bk-mysql-real")

	script := "#!/bin/sh\nfor a in \"$@\"; do\n  case \"$a\" in\n    --result-file=*) printf 'real-dump' > \"${a#--result-file=}\" ;;\n  esac\ndone\n"
	dir := prepareToolScript(t, "mysqldump", script)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	store := newFakeObjStore(nil)
	e := New("mysql", "u:pw@tcp(127.0.0.1:3306)/gamedb", m, store, "backups")
	require.NoError(t, e.RunBackup(context.Background(), "bk-mysql-real", "daily", "full"))

	var got model.Backup
	require.NoError(t, db.Where("backup_id = ?", "bk-mysql-real").First(&got).Error)
	assert.Equal(t, "succeeded", got.Status)
	assert.Equal(t, int64(len("real-dump")), got.Size)
	require.Len(t, store.putKeys, 1)
	assert.Regexp(t, `^backups/\d{8}/full-bk-mysql-real\.sql$`, store.putKeys[0])
}

func TestDump_MissingBinary(t *testing.T) {
	// PATH 指向空目录：exec 找不到二进制 → run 报错。
	t.Setenv("PATH", t.TempDir())
	e := New("mysql", "u:pw@tcp(127.0.0.1:3306)/db", nil, nil, "")
	_, _, _, err := e.dump(context.Background(), "bk-no-binary")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mysqldump")
}

// ---- dumpSQLite / fileSHA256 错误分支 ----

func TestDumpSQLite_SourceMissing(t *testing.T) {
	e := New("sqlite", filepath.Join(t.TempDir(), "missing.db"), nil, nil, "")
	_, _, _, err := e.dump(context.Background(), "bk-sqlite-missing")
	require.Error(t, err)
}

func TestFileSHA256(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(p, []byte("abc"), 0o600))
	sum, err := fileSHA256(p)
	require.NoError(t, err)
	assert.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", sum)

	_, err = fileSHA256(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}

// ---- DSN 工具函数 ----

func TestOrDefault(t *testing.T) {
	assert.Equal(t, "3306", orDefault("", "3306"))
	assert.Equal(t, "5432", orDefault("  ", "5432"))
	assert.Equal(t, "v", orDefault("v", "d"))
}

func TestHiddenPassword(t *testing.T) {
	u, err := url.Parse("postgres://h/db")
	require.NoError(t, err)
	assert.Empty(t, hiddenPassword(u)) // 无 userinfo

	u, err = url.Parse("postgres://u@h/db")
	require.NoError(t, err)
	assert.Empty(t, hiddenPassword(u)) // 有用户无密码

	u, err = url.Parse("postgres://u:pw@h/db")
	require.NoError(t, err)
	assert.Equal(t, "pw", hiddenPassword(u))
}

func TestParsePostgresDSN_EdgeCases(t *testing.T) {
	// URL 解析失败（非法转义）。
	_, _, _, _, _, err := parsePostgresDSN("postgres://h/%zz")
	require.Error(t, err)

	// key=value：无 "=" 的 token 被跳过；端口缺省补 5432。
	host, port, user, pass, db, err := parsePostgresDSN("host=h junk-token user=u password=p dbname=d")
	require.NoError(t, err)
	assert.Equal(t, "h", host)
	assert.Equal(t, "5432", port)
	assert.Equal(t, "u", user)
	assert.Equal(t, "p", pass)
	assert.Equal(t, "d", db)
}

func TestParseMySQLDSN_EdgeCases(t *testing.T) {
	// URL 形态缺省端口补 3306；无密码。
	host, port, user, pass, db, err := parseMySQLDSN("mysql://u@host/dbname")
	require.NoError(t, err)
	assert.Equal(t, "host", host)
	assert.Equal(t, "3306", port)
	assert.Equal(t, "u", user)
	assert.Empty(t, pass)
	assert.Equal(t, "dbname", db)

	// go-sql-driver 形态缺省端口 + 查询参数剥离（IPv6 无端口形态见交付报告：
	// parseMySQLDSN 用 LastIndex(":") 切分会误切 [::1]，此处不覆盖该疑似 bug）。
	host, port, user, pass, db, err = parseMySQLDSN("u:p@tcp(db.internal)/db?charset=utf8")
	require.NoError(t, err)
	assert.Equal(t, "db.internal", host)
	assert.Equal(t, "3306", port)
	assert.Equal(t, "u", user)
	assert.Equal(t, "p", pass)
	assert.Equal(t, "db", db)

	// tcp( 后缺少 ')'。
	_, _, _, _, _, err = parseMySQLDSN("u@tcp(host/db")
	require.Error(t, err)
}
