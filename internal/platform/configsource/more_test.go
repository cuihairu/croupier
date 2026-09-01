package configsource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ---- db.go：sqlmock 预置 s.db（绕过真实 MySQL），语义与生产一致 ----

func newMockDBSource(t *testing.T) (*dbSource, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	gdb, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	return &dbSource{db: gdb}, mock
}

func TestDBSourceNewValidation(t *testing.T) {
	_, err := newDBSource(map[string]interface{}{})
	assert.ErrorContains(t, err, "dsn")

	src, err := newDBSource(map[string]interface{}{
		"dsn":    "user:pass@tcp(127.0.0.1:3306)/db",
		"tables": []interface{}{"activity", " users ", 42},
	})
	require.NoError(t, err)
	dbSrc := src.(*dbSource)
	assert.Equal(t, "db", dbSrc.Type())
	assert.True(t, dbSrc.tableAllowed("users")) // TrimSpace
	assert.False(t, dbSrc.tableAllowed("hero")) // 白名单外
	assert.True(t, dbSrc.tableAllowed("activity"))
}

func TestDBSourceTableAllowedUnrestricted(t *testing.T) {
	s := &dbSource{}
	assert.True(t, s.tableAllowed("anything"))
	assert.False(t, s.tableAllowed(""))
}

func TestDBSourceConnCachedAndError(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	gdb, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	s := &dbSource{db: gdb}
	got, err := s.conn() // 缓存命中分支
	require.NoError(t, err)
	assert.Same(t, gdb, got)

	s2 := &dbSource{dsn: "user:pass@tcp(127.0.0.1:1)/db"}
	_, err = s2.conn()
	assert.ErrorContains(t, err, "db connect")
}

func TestDBSourceList(t *testing.T) {
	s, mock := newMockDBSource(t)
	ctx := context.Background()
	mock.ExpectQuery(regexp.QuoteMeta("SHOW TABLES")).WillReturnRows(
		sqlmock.NewRows([]string{"Tables_in_db"}).AddRow("game_activity").AddRow("users").AddRow(""))

	root, err := s.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, root, 2)
	assert.Equal(t, "game_activity.csv", root[0].Path)
	assert.Equal(t, "users.csv", root[1].Path)

	sub, err := s.List(ctx, "sub")
	require.NoError(t, err)
	assert.Nil(t, sub) // 表平铺在根，无子目录

	_, err = s.List(ctx, "a/../b")
	assert.Error(t, err)

	s2, mock2 := newMockDBSource(t)
	mock2.ExpectQuery(regexp.QuoteMeta("SHOW TABLES")).WillReturnError(errors.New("boom"))
	_, err = s2.List(ctx, "")
	assert.ErrorContains(t, err, "list tables")
}

func TestDBSourceResolveAndRead(t *testing.T) {
	s, mock := newMockDBSource(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "name", "extra"}).
		AddRow(1, "Alice", []byte("b1")).
		AddRow(int64(2), nil, "z")
	mock.ExpectQuery(regexp.QuoteMeta("SHOW TABLES")).
		WillReturnRows(sqlmock.NewRows([]string{"Tables_in_db"}).AddRow("game_activity"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `game_activity` LIMIT 500")).WillReturnRows(rows)

	out, err := s.Read(ctx, "game_activity.csv")
	require.NoError(t, err)
	assert.Equal(t, "id,name,extra\n1,Alice,b1\n2,,z\n", string(out))

	// 表名不存在（resolveTable 再查一次 SHOW TABLES）
	mock.ExpectQuery(regexp.QuoteMeta("SHOW TABLES")).
		WillReturnRows(sqlmock.NewRows([]string{"Tables_in_db"}).AddRow("other"))
	_, err = s.Read(ctx, "missing.csv")
	assert.ErrorContains(t, err, "table not found")

	_, err = s.Read(ctx, "")
	assert.ErrorContains(t, err, "path required")

	_, err = s.Read(ctx, "a/../x")
	assert.Error(t, err)

	s2, mock2 := newMockDBSource(t)
	mock2.ExpectQuery(regexp.QuoteMeta("SHOW TABLES")).WillReturnRows(
		sqlmock.NewRows([]string{"Tables_in_db"}).AddRow("t1"))
	mock2.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `t1` LIMIT 500")).
		WillReturnError(errors.New("select boom"))
	_, err = s2.Read(ctx, "t1")
	assert.ErrorContains(t, err, "read table")
}

// ---- nacos.go 补充：登录/鉴权/Read/Write 错误分支 ----

func newNacosFake(t *testing.T, handler http.HandlerFunc) *nacosSource {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	src, err := newNacosSource(map[string]interface{}{"endpoint": srv.URL})
	require.NoError(t, err)
	return src.(*nacosSource)
}

func TestNacosSourceAccessTokenFlow(t *testing.T) {
	var logins int
	s := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/nacos/v1/auth/login" {
			logins++
			_, _ = w.Write([]byte(`{"accessToken":"tok-1"}`))
			return
		}
		w.WriteHeader(404)
	})
	assert.Equal(t, "nacos", s.Type())

	tok, err := s.accessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "", tok) // 无凭据

	s.username, s.password = "u", "p"
	tok, err = s.accessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok)
	tok, err = s.accessToken(context.Background()) // 缓存命中
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok)
	assert.Equal(t, 1, logins)
}

func TestNacosSourceLoginFailures(t *testing.T) {
	badJSON := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"no-token":1}`))
	})
	badJSON.username = "u"
	_, err := badJSON.accessToken(context.Background())
	assert.ErrorContains(t, err, "bad response")

	unreachable := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {})
	unreachable.endpoint = "http://127.0.0.1:1"
	unreachable.username = "u"
	_, err = unreachable.accessToken(context.Background())
	assert.ErrorContains(t, err, "nacos login")
}

func TestNacosSourceReadWithAuth(t *testing.T) {
	var gotAuth, gotTenant string
	s := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nacos/v1/auth/login":
			_, _ = w.Write([]byte(`{"accessToken":"tok-9"}`))
		case "/nacos/v1/cs/configs":
			gotAuth = r.URL.Query().Get("accessToken")
			gotTenant = r.URL.Query().Get("tenant")
			_, _ = w.Write([]byte(`on: true`))
		}
	})
	s.username, s.namespaceID = "u", "ns1"

	body, err := s.Read(context.Background(), "runtime/switch.yaml")
	require.NoError(t, err)
	assert.Equal(t, "on: true", string(body))
	assert.Equal(t, "tok-9", gotAuth)
	assert.Equal(t, "ns1", gotTenant)

	s2 := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	_, err = s2.Read(context.Background(), "a/b")
	assert.ErrorContains(t, err, "not found")

	s3 := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) })
	_, err = s3.Read(context.Background(), "a/b")
	assert.Error(t, err)

	_, err = s.Read(context.Background(), "")
	assert.ErrorContains(t, err, "path required")
	_, err = s.Read(context.Background(), "a/../b")
	assert.Error(t, err)
}

func TestNacosSourceWriteErrorsAndSuccess(t *testing.T) {
	var published string
	s := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/nacos/v1/cs/configs" && r.Method == http.MethodPost {
			_ = r.ParseForm()
			switch r.Form.Get("content") {
			case "boom":
				w.WriteHeader(500)
				return
			case "false-reply":
				_, _ = w.Write([]byte("false"))
				return
			}
			published = r.Form.Get("content")
			_, _ = w.Write([]byte("true"))
			return
		}
		w.WriteHeader(404)
	})

	ctx := context.Background()
	require.NoError(t, s.Write(ctx, "runtime/switch.yaml", []byte("on: true"), "test"))
	assert.Equal(t, "on: true", published)

	assert.ErrorContains(t, s.Write(ctx, "", nil, ""), "path required")
	big := make([]byte, 8<<20+1)
	assert.ErrorContains(t, s.Write(ctx, "big", big, ""), "too large")
	assert.ErrorContains(t, s.Write(ctx, "a/../x", []byte("x"), ""), "path")
}

// ---- git.go 补充：worktree 缓存/错误缓存、subPath、joinSub ----

func TestGitSourceWorktreeErrorCached(t *testing.T) {
	_, err := New(testBinding("git", `{}`))
	assert.ErrorContains(t, err, "repoUrl")

	src, err := New(testBinding("git", `{"repoUrl":"ssh://git@host/repo.git"}`))
	require.NoError(t, err)
	assert.Equal(t, "git", src.Type())

	_, err = src.List(context.Background(), "")
	assert.ErrorContains(t, err, "scheme")

	// 第二次调用走缓存错误分支（短 TTL）
	_, err = src.(*gitSource).worktree(context.Background())
	assert.Error(t, err)
}

func TestGitSourceWorktreeCacheHit(t *testing.T) {
	dir := newGitFixture(t)
	src, err := New(testBinding("git", fmt.Sprintf(`{"repoUrl":"file://%s"}`, dir)))
	require.NoError(t, err)
	gs := src.(*gitSource)

	wt1, err := gs.worktree(context.Background())
	require.NoError(t, err)
	wt2, err := gs.worktree(context.Background()) // TTL 内命中缓存
	require.NoError(t, err)
	assert.True(t, wt1 == wt2, "cache hit must return the same fs")

	// subPath 前缀：List("") 列出 subPath 下的文件
	subSrc, err := New(testBinding("git",
		fmt.Sprintf(`{"repoUrl":"file://%s","subPath":"gameplay"}`, dir)))
	require.NoError(t, err)
	sub, err := subSrc.List(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, sub, 2) // item.json / hero.json // item.json / hero.json

	// Read 打不开的路径
	_, err = src.Read(context.Background(), "nope.json")
	assert.ErrorContains(t, err, "open")
}

func TestJoinSubVariants(t *testing.T) {
	assert.Equal(t, "b", joinSub("", "b"))
	assert.Equal(t, "base", joinSub("base", ""))
	assert.Equal(t, "base/b", joinSub("base/", "b"))
}

func TestCroupierSourceSetModelAndType(t *testing.T) {
	m := newCroupierFixture(t)
	SetCroupierVersionModel(m)
	t.Cleanup(func() { SetCroupierVersionModel(nil) })

	src, err := newCroupierSource(map[string]interface{}{"namespaces": []interface{}{"gameplay"}}, "demo", "prod")
	require.NoError(t, err)
	assert.Equal(t, "croupier", src.Type())
}

// ---- croupier.go 错误分支 ----

func TestCroupierSourceErrorPaths(t *testing.T) {
	m := newCroupierFixture(t)
	SetCroupierVersionModel(m)
	t.Cleanup(func() { SetCroupierVersionModel(nil) })

	// 未初始化的版本模型 → 拒绝创建
	prev := croupierVersionModel
	croupierVersionModel = nil
	_, err := New(testBinding("croupier", `{}`))
	assert.ErrorContains(t, err, "not initialized")
	croupierVersionModel = prev
	_ = prev

	// namespace 白名单
	src, err := New(testBinding("croupier", `{"namespaces":["gameplay"]}`))
	require.NoError(t, err)
	cs := src.(*croupierSource)
	ctx := context.Background()

	// 禁止的 namespace：List / splitKey → 错误
	_, err = cs.List(ctx, "runtime")
	assert.ErrorContains(t, err, "not allowed")
	_, _, err = src.(*croupierSource).splitKey("runtime/k.yaml")
	assert.ErrorContains(t, err, "not allowed")
	_, err = src.Read(ctx, "runtime/k.yaml")
	assert.Error(t, err)

	// 非法路径
	_, err = src.List(ctx, "a/../b")
	assert.Error(t, err)

	// 根目录空 ns 过滤 + ListLatest 正常
	if _, err := m.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "item", Content: "{}", Format: "json", GameID: "demo", Env: "prod", Namespace: "gameplay",
	}, "t"); err != nil {
		t.Fatal(err)
	}
	entries, err := src.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestSourceFactoryBranches(t *testing.T) {
	_, err := New(nil)
	assert.ErrorContains(t, err, "binding required")

	_, err = New(testBinding("db", `{bad json`))
	assert.ErrorContains(t, err, "invalid binding config json")

	_, err = New(testBinding("unknown-type", `{}`))
	assert.ErrorContains(t, err, "unsupported")

	// 未初始化的 croupier 模型 → newCroupierSource 错误
	prev := croupierVersionModel
	croupierVersionModel = nil
	_, err = New(testBinding("croupier", `{}`))
	assert.ErrorContains(t, err, "not initialized")
	croupierVersionModel = prev
}

func TestConfigIntVariants(t *testing.T) {
	assert.Equal(t, 0, configInt(map[string]interface{}{}, "db", 0))
	assert.Equal(t, 3, configInt(map[string]interface{}{"db": "3"}, "db", 1)) // string 可解析
	assert.Equal(t, 5, configInt(map[string]interface{}{"db": 5.0}, "db", 0))
	assert.Equal(t, 7, configInt(map[string]interface{}{"db": 7}, "db", 0))
	assert.Equal(t, -1, configInt(map[string]interface{}{"db": -1}, "db", 7)) // 负数原样返回
}

func TestMaskDSNBranch(t *testing.T) {
	assert.Equal(t, "{}", MaskSecrets(`not json`))
	assert.Contains(t, MaskSecrets(`{"dsn":"tcp://user:secret@x/db"}`), "******")
}
