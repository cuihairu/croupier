package router

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"
)

// TestGameDB_InflightRecheckPrefersRepublishedEntry exercises the singleflight
// re-check branch in GameDB: after a caller misses the fast path and becomes
// the singleflight leader, another opener may publish a cache entry while it
// was winning the race; the re-check must return that entry instead of opening
// a duplicate connection.
//
// 该交错窗口内产品代码自身无挂起点（fast-path RUnlock 与 re-check RLock 之间
// 是同步代码），真实并发不可构造；历史上用「主线程持写锁泊车 caller + 填充者
// TryLock 自旋」24 轮相位对齐编排 probabilistic 触发，全量负载下会整轮落空
// （覆盖率抖动根源）。现按「时序分支用注入点」以 gameDBInflightHook 在回调
// 入口同步注入竞争条目，单轮即确定性覆盖。
func TestGameDB_InflightRecheckPrefersRepublishedEntry(t *testing.T) {
	tmp := t.TempDir()
	dbName := DefaultGameDBName("recheck", "prod")
	published := openSQLiteDB(t, filepath.Join(tmp, "published.db"))

	r, metaDB := newTestRouter(t, Config{})
	defer func() {
		_ = r.Close()
		_ = openSQLiteClose(metaDB)
	}()

	var openCalls int32
	r.cfg.Open = func(driverName, dsn string) (*gorm.DB, error) {
		atomic.AddInt32(&openCalls, 1)
		return openSQLiteDB(t, filepath.Join(tmp, "fresh.db")), nil
	}

	// 回调入口同步发布竞争条目：re-check 必须复用它，leader 不得再 open。
	gameDBInflightHook = func() {
		r.mu.Lock()
		if _, dup := r.cache[dbName]; !dup {
			r.cache[dbName] = published
			r.gameOfDB[dbName] = "recheck"
		}
		r.mu.Unlock()
	}
	t.Cleanup(func() { gameDBInflightHook = nil })

	got, err := r.GameDB(context.Background(), "recheck", "prod")
	if err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	if got != published {
		t.Fatal("re-check 必须返回已发布的连接，而不是新开的")
	}
	if n := atomic.LoadInt32(&openCalls); n != 0 {
		t.Fatalf("re-check 命中后不应触发任何 open，got %d", n)
	}
	r.mu.RLock()
	cached := r.cache[dbName]
	r.mu.RUnlock()
	if cached != published {
		t.Fatal("cache 必须保留已发布的条目")
	}
}
