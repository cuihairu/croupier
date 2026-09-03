package configsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	gosqlite "github.com/glebarez/sqlite"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

// writeNacosItemsV9 encodes a list page the way the Nacos list API does.
func writeNacosItemsV9(w http.ResponseWriter, items []nacosConfigItem) error {
	return json.NewEncoder(w).Encode(map[string]interface{}{"pageItems": items})
}

// ---- source.go：New 的 db 分支 + maskDSN 回退 ----

func TestNewSourceDBBranchV9(t *testing.T) {
	src, err := New(testBinding("db", `{"dsn":"user:pass@tcp(127.0.0.1:3306)/db","tables":["activity"]}`))
	require.NoError(t, err)
	assert.Equal(t, "db", src.Type())
	assert.False(t, IsWritable(src))
}

func TestMaskDSNFallbackV9(t *testing.T) {
	assert.Equal(t, "no-at-sign", maskDSN("no-at-sign"))
	assert.Equal(t, "useronly@host", maskDSN("useronly@host"))
	out := MaskSecrets(`{"dsn":"useronly@host"}`)
	assert.Contains(t, out, "useronly@host")
}

// ---- redis.go：构造校验、Type、错误分支、scan 边界 ----

func TestRedisSourceConstructAndTypeV9(t *testing.T) {
	_, err := newRedisSource(map[string]interface{}{})
	assert.ErrorContains(t, err, "addr")

	src, err := newRedisSource(map[string]interface{}{"addr": "127.0.0.1:6379"})
	require.NoError(t, err)
	assert.Equal(t, "redis", src.Type())
	assert.True(t, IsWritable(src))
}

func deadRedisV9() *redis.Client {
	return redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: 0})
}

func TestRedisSourceErrorPathsV9(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	src, err := New(testBinding("redis", fmt.Sprintf(`{"addr":"%s","prefix":"cfg:"}`, mr.Addr())))
	require.NoError(t, err)
	ws := src.(WritableSource)

	// 非法路径
	_, err = src.List(ctx, "a/../b")
	assert.Error(t, err)
	_, err = src.Read(ctx, "a/../b")
	assert.Error(t, err)
	assert.ErrorContains(t, ws.Write(ctx, "a/../b", []byte("x"), ""), "invalid path segment")

	// key 不存在 / 空 path
	_, err = src.Read(ctx, "missing/key")
	assert.ErrorContains(t, err, "key not found")
	_, err = src.Read(ctx, "")
	assert.ErrorContains(t, err, "path required")

	// 空 path / 超大内容
	assert.ErrorContains(t, ws.Write(ctx, "", []byte("x"), ""), "path required")
	big := make([]byte, 8<<20+1)
	assert.ErrorContains(t, ws.Write(ctx, "too/big", big, ""), "too large")

	// 连接失败：scan / get / set
	dead := &redisSource{client: deadRedisV9(), sep: "/"}
	_, err = dead.List(ctx, "")
	assert.ErrorContains(t, err, "redis scan")
	_, err = dead.Read(ctx, "some/key")
	assert.ErrorContains(t, err, "redis get")
	err = dead.Write(ctx, "some/key", []byte("v"), "")
	assert.ErrorContains(t, err, "redis set")
}

func TestRedisSourceListSkipEdgeKeysV9(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	// key 恰好等于 base（rest == ""）
	mr.Set("cfg:", "x")
	mr.Set("cfg:gameplay/item", "1")

	src, err := New(testBinding("redis", fmt.Sprintf(`{"addr":"%s","prefix":"cfg:"}`, mr.Addr())))
	require.NoError(t, err)
	root, err := src.List(ctx, "")
	require.NoError(t, err)
	names := map[string]bool{}
	for _, e := range root {
		names[e.Name] = true
	}
	assert.True(t, names["gameplay"], "root = %+v", root)
	assert.False(t, names[""], "bare base key must be skipped")

	// 无 prefix：TrimPrefix(key, "") == key → 全部跳过
	src2, err := New(testBinding("redis", fmt.Sprintf(`{"addr":"%s"}`, mr.Addr())))
	require.NoError(t, err)
	root2, err := src2.List(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, root2)
}

// ---- git.go：URL 解析错误、branch/auth 克隆选项、克隆失败、List/Read 错误、memfs 边界 ----

func TestCanonicalGitURLParseErrorV9(t *testing.T) {
	_, err := canonicalGitURL("://bad")
	assert.ErrorContains(t, err, "invalid repoUrl")
	_, err = canonicalGitURL("http://exa\x7fmple.com/repo.git")
	assert.Error(t, err)
}

func TestGitSourceCloneBranchAuthV9(t *testing.T) {
	dir := newGitFixture(t)
	src, err := New(testBinding("git", fmt.Sprintf(
		`{"repoUrl":"file://%s","branch":"master","username":"u","password":"p"}`, dir)))
	require.NoError(t, err)
	ctx := context.Background()
	root, err := src.List(ctx, "")
	require.NoError(t, err)
	assert.NotEmpty(t, root)
}

func TestGitSourceCloneFailureV9(t *testing.T) {
	missing := t.TempDir() + "/definitely-missing"
	src, err := New(testBinding("git", fmt.Sprintf(`{"repoUrl":"file://%s"}`, missing)))
	require.NoError(t, err)
	ctx := context.Background()
	_, err = src.List(ctx, "")
	assert.ErrorContains(t, err, "git clone failed")
}

func TestGitSourceListReadErrorsV9(t *testing.T) {
	dir := newGitFixture(t)
	src, err := New(testBinding("git", fmt.Sprintf(`{"repoUrl":"file://%s"}`, dir)))
	require.NoError(t, err)
	ctx := context.Background()

	_, err = src.List(ctx, "a/../b")
	assert.Error(t, err)

	_, err = src.List(ctx, "nonexistent")
	assert.ErrorContains(t, err, "read dir")

	_, err = src.Read(ctx, "")
	assert.ErrorContains(t, err, "path required")

	// worktree 返回缓存错误
	gs := src.(*gitSource)
	gs.cached, gs.cachedE = nil, errors.New("wt boom")
	gs.expires = time.Now().Add(time.Minute)
	_, err = gs.Read(ctx, "runtime/switch.yaml")
	assert.ErrorContains(t, err, "wt boom")
}

func TestGitSourceMemfsEdgeCasesV9(t *testing.T) {
	ctx := context.Background()

	// 根目录含 .git 目录 → 跳过
	fs := memfs.New()
	require.NoError(t, fs.MkdirAll(".git", 0o755))
	f, err := fs.Create("a.txt")
	require.NoError(t, err)
	_, _ = f.Write([]byte("hi"))
	require.NoError(t, f.Close())
	gs := &gitSource{cached: fs, expires: time.Now().Add(time.Minute)}

	entries, err := gs.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "a.txt", entries[0].Name)

	// 超过 8MiB 的文件
	fs2 := memfs.New()
	f2, err := fs2.Create("big.bin")
	require.NoError(t, err)
	_, _ = f2.Write(make([]byte, 8<<20+1))
	require.NoError(t, f2.Close())
	gs2 := &gitSource{cached: fs2, expires: time.Now().Add(time.Minute)}

	_, err = gs2.Read(ctx, "big.bin")
	assert.ErrorContains(t, err, "file too large")
}

// ---- nacos.go：错误分支全集 ----

func TestNacosSourceValidationV9(t *testing.T) {
	_, err := newNacosSource(map[string]interface{}{})
	assert.ErrorContains(t, err, "endpoint")
}

func TestNacosSourceLoginStatusErrorV9(t *testing.T) {
	s := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	})
	s.username, s.password = "u", "p"
	_, err := s.accessToken(context.Background())
	assert.ErrorContains(t, err, "nacos login failed: status 403")
}

func TestNacosSourceRequestBuildErrorsV9(t *testing.T) {
	ctx := context.Background()

	// accessToken：NewRequest 失败（URL 含控制字符）
	s := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {})
	s.endpoint = "http://127.0.0.1:1\x7f"
	s.username, s.password = "u", "p"
	_, err := s.accessToken(ctx)
	assert.Error(t, err)

	// listAll / Read / Write：NewRequest 失败
	s2 := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {})
	s2.endpoint = "http://127.0.0.1:1\x7f"
	_, err = s2.listAll(ctx)
	assert.Error(t, err)
	_, err = s2.Read(ctx, "a/b")
	assert.Error(t, err)
	err = s2.Write(ctx, "a/b", []byte("x"), "")
	assert.Error(t, err)
}

func TestNacosWithAuthAndListErrorsV9(t *testing.T) {
	ctx := context.Background()

	// withAuth：登录失败传播
	s := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {})
	s.endpoint = "http://127.0.0.1:1"
	s.username, s.password = "u", "p"
	err := s.withAuth(ctx, nil)
	assert.ErrorContains(t, err, "nacos login")

	// listAll：withAuth 失败
	_, err = s.listAll(ctx)
	assert.ErrorContains(t, err, "nacos login")

	// listAll：Do 失败（不可达）
	s2 := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {})
	s2.endpoint = "http://127.0.0.1:1"
	_, err = s2.listAll(ctx)
	assert.ErrorContains(t, err, "nacos list")

	// List：错误传播 + 非法路径
	_, err = s2.List(ctx, "")
	assert.ErrorContains(t, err, "nacos list")
	_, err = s2.List(ctx, "a/../b")
	assert.Error(t, err)

	// listAll：非 200
	s3 := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	_, err = s3.listAll(ctx)
	assert.ErrorContains(t, err, "nacos list failed: status 500")

	// listAll：坏 JSON
	s4 := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("not-json")) })
	_, err = s4.listAll(ctx)
	assert.ErrorContains(t, err, "bad response")
}

func TestNacosListEntryEdgesV9(t *testing.T) {
	items := []nacosConfigItem{
		{DataID: "gameplay/"},
		{DataID: "gameplay/item.json"},
		{DataID: "other/x"},
		{DataID: "root.txt"},
	}
	srv := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/nacos/v1/cs/configs" {
			w.Header().Set("Content-Type", "application/json")
			_ = writeNacosItemsV9(w, items)
			return
		}
		w.WriteHeader(404)
	})
	ctx := context.Background()

	sub, err := srv.List(ctx, "gameplay")
	require.NoError(t, err)
	require.Len(t, sub, 1)
	assert.Equal(t, "item.json", sub[0].Name)

	root, err := srv.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, root, 3)
	// 目录在前，同组内按名称排序
	assert.Equal(t, "gameplay", root[0].Name)
	assert.True(t, root[0].Dir)
	assert.Equal(t, "other", root[1].Name)
	assert.True(t, root[1].Dir)
	assert.False(t, root[2].Dir)
	assert.Equal(t, "root.txt", root[2].Name)
}

func TestNacosReadWriteTransportErrorsV9(t *testing.T) {
	ctx := context.Background()

	// Read：withAuth 失败
	s := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {})
	s.endpoint = "http://127.0.0.1:1"
	s.username, s.password = "u", "p"
	_, err := s.Read(ctx, "a/b")
	assert.ErrorContains(t, err, "nacos login")

	// Write：withAuth 失败
	err = s.Write(ctx, "a/b", []byte("x"), "")
	assert.ErrorContains(t, err, "nacos login")

	// Read / Write：Do 失败（不可达，无凭据）
	s2 := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {})
	s2.endpoint = "http://127.0.0.1:1"
	_, err = s2.Read(ctx, "a/b")
	assert.ErrorContains(t, err, "nacos get")
	err = s2.Write(ctx, "a/b", []byte("x"), "")
	assert.ErrorContains(t, err, "nacos publish")

	// Write：500 与 false 应答
	s3 := newNacosFake(t, func(w http.ResponseWriter, r *http.Request) {
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
		}
		w.WriteHeader(404)
	})
	assert.ErrorContains(t, s3.Write(ctx, "a/b", []byte("boom"), ""), "nacos publish failed")
	assert.ErrorContains(t, s3.Write(ctx, "a/b", []byte("false-reply"), ""), "nacos publish failed")
}

// ---- db.go：conn 失败与 resolveTable 失败 ----

func TestDBSourceConnAndResolveErrorsV9(t *testing.T) {
	ctx := context.Background()
	s := &dbSource{dsn: "user:pass@tcp(127.0.0.1:1)/db"}
	_, err := s.List(ctx, "")
	assert.ErrorContains(t, err, "db connect")
	_, err = s.Read(ctx, "t.csv")
	assert.ErrorContains(t, err, "db connect")

	s2, mock := newMockDBSource(t)
	mock.ExpectQuery(regexp.QuoteMeta("SHOW TABLES")).WillReturnError(errors.New("boom"))
	_, err = s2.Read(ctx, "t.csv")
	assert.ErrorContains(t, err, "list tables")
}

// ---- croupier.go：空 ns / 去重 / 排序 / 模型错误 / splitKey / 超大写入 ----

func newBrokenCroupierFixtureV9(t *testing.T) *model.ConfigVersionModel {
	t.Helper()
	db, err := gorm.Open(gosqlite.Open(t.TempDir()+"/broken.db"), &gorm.Config{})
	require.NoError(t, err)
	if err := db.AutoMigrate(&model.ConfigVersion{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return model.NewConfigVersionModel(db)
}

func TestCroupierSourceEmptyNSDedupSortV9(t *testing.T) {
	m := newCroupierFixture(t)
	prev := croupierVersionModel
	croupierVersionModel = m
	t.Cleanup(func() { croupierVersionModel = prev })

	ctx := context.Background()
	// 直接落库绕过 normalize：空 namespace → 默认 runtime
	rec := &model.ConfigVersion{Key: "k1", Version: 1, Value: "v1",
		GameID: "demo", Env: "prod", Namespace: ""}
	if err := m.DB().Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	// 同 namespace 两条 key：根列表去重 + 子目录排序比较器
	for i, key := range []string{"a_item", "b_item"} {
		if _, err := m.CreateWithMeta(ctx, model.ConfigVersionPayload{
			Key: key, Content: fmt.Sprintf("v-%d", i), Format: "json",
			GameID: "demo", Env: "prod", Namespace: "gameplay",
		}, "t"); err != nil {
			t.Fatal(err)
		}
	}

	src, err := New(testBinding("croupier", `{}`))
	require.NoError(t, err)

	root, err := src.List(ctx, "")
	require.NoError(t, err)
	names := map[string]int{}
	for _, e := range root {
		names[e.Name]++
	}
	assert.Equal(t, 1, names["runtime"], "empty ns defaults to runtime: %+v", root)
	assert.Equal(t, 1, names["gameplay"], "dup ns collapsed: %+v", root)

	sub, err := src.List(ctx, "gameplay")
	require.NoError(t, err)
	require.Len(t, sub, 2)
	assert.Equal(t, "a_item.json", sub[0].Name)
	assert.Equal(t, "b_item.json", sub[1].Name)
}

func TestCroupierSourceModelErrorsV9(t *testing.T) {
	m := newBrokenCroupierFixtureV9(t)
	prev := croupierVersionModel
	croupierVersionModel = m
	t.Cleanup(func() { croupierVersionModel = prev })

	src, err := New(testBinding("croupier", `{}`))
	require.NoError(t, err)
	cs := src.(*croupierSource)
	ctx := context.Background()

	_, err = src.List(ctx, "")
	assert.Error(t, err)
	_, err = src.List(ctx, "runtime")
	assert.Error(t, err)

	_, err = src.Read(ctx, "runtime/k")
	assert.ErrorContains(t, err, "config not found")

	// splitKey 错误：无斜杠 / 非法路径
	_, _, err = cs.splitKey("noslash")
	assert.ErrorContains(t, err, "<namespace>/<key>")
	_, _, err = cs.splitKey("a/../b")
	assert.Error(t, err)
	_, err = src.Read(ctx, "a/../b")
	assert.Error(t, err)

	// Write：splitKey 错误 + CreateWithMeta 错误
	assert.Error(t, src.(WritableSource).Write(ctx, "noslash", []byte("x"), "m"))
	assert.Error(t, src.(WritableSource).Write(ctx, "a/../b", []byte("x"), "m"))
	assert.Error(t, src.(WritableSource).Write(ctx, "runtime/k", []byte("x"), "m"))
}

func TestCroupierSourceWriteTooLargeV9(t *testing.T) {
	m := newCroupierFixture(t)
	prev := croupierVersionModel
	croupierVersionModel = m
	t.Cleanup(func() { croupierVersionModel = prev })

	src, err := New(testBinding("croupier", `{}`))
	require.NoError(t, err)
	big := make([]byte, 8<<20+1)
	err = src.(WritableSource).Write(context.Background(), "runtime/k", big, "m")
	assert.ErrorContains(t, err, "too large")
}
