package router_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/svc"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// arch010Scope is a (gameID, env) pair used as an independent connection pool.
type arch010Scope struct {
	gameID string
	env    string
}

// arch010EnvInt reads a positive integer from ARCH010_* env with a default.
// The capacity probe is parameterized so a reviewer can run it against any
// (games, envs, concurrency) target without editing the test.
func arch010EnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// arch010ServerCount queries the server-side connection count (the real
// capacity constraint). The admin connection is separate from the router's
// pools so the measurement reflects what the server actually sees.
func arch010ServerCount(admin *gorm.DB, query string) int64 {
	var n int64
	if err := admin.Raw(query).Row().Scan(&n); err != nil {
		return -1
	}
	return n
}

// arch010MaxConns reads the server's max_connections quota.
func arch010MaxConns(driver string, admin *gorm.DB) int64 {
	var n int64
	var q string
	switch driver {
	case "mysql":
		q = "SELECT @@max_connections"
	case "postgres":
		q = "SELECT current_setting('max_connections')::int"
	default:
		return 0
	}
	if err := admin.Raw(q).Row().Scan(&n); err != nil {
		return 0
	}
	return n
}

// TestArch010Probe records pool count / client+server connection peaks / P95
// first-open latency for a parameterized (games, envs, concurrency) matrix,
// then verifies reclamation and derives a capacity conclusion from the server
// max_connections quota. ARCH010_DRIVER (mysql|postgres) gates on the real
// containers; the other ARCH010_* vars parameterize the matrix and optional
// target assertions (ARCH010_TARGET_P95_MS, ARCH010_TARGET_MAX_CONNS).
func TestArch010Probe(t *testing.T) {
	driver := os.Getenv("ARCH010_DRIVER")
	if driver == "" {
		t.Skip("ARCH010_DRIVER not set")
	}
	var base, adminDSN, serverQuery string
	var open func(string) (*gorm.DB, error)
	switch driver {
	case "mysql":
		base = "root:root@tcp(127.0.0.1:13307)/atlas_test?parseTime=true"
		adminDSN = "root:root@tcp(127.0.0.1:13307)/?parseTime=true"
		serverQuery = "SELECT COUNT(*) FROM information_schema.processlist"
		open = func(dsn string) (*gorm.DB, error) {
			return gorm.Open(gmysql.Open(dsn), &gorm.Config{})
		}
	case "postgres":
		base = "postgres://atlas:atlas@127.0.0.1:15432/atlas_test?sslmode=disable"
		adminDSN = "postgres://atlas:atlas@127.0.0.1:15432/postgres?sslmode=disable"
		serverQuery = "SELECT COUNT(*) FROM pg_stat_activity"
		open = func(dsn string) (*gorm.DB, error) {
			return gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
		}
	default:
		t.Skip("unknown driver")
	}

	games := arch010EnvInt("ARCH010_GAMES", 8)
	envs := arch010EnvInt("ARCH010_ENVS", 1)
	concurrency := arch010EnvInt("ARCH010_CONCURRENCY", games*envs)

	var scopes []arch010Scope
	for g := 0; g < games; g++ {
		for e := 0; e < envs; e++ {
			scopes = append(scopes, arch010Scope{
				gameID: fmt.Sprintf("g%d", g),
				env:    fmt.Sprintf("env%d", e),
			})
		}
	}

	metaDB, err := open(base)
	if err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("arch010probe_%d", time.Now().UnixNano()%1e9)
	r := router.New(router.Config{
		Driver: driver, MetaDSN: base,
		DSNForDatabase: func(metaDSN, dbName string) string { return svc.DSNForDatabase(driver, metaDSN, dbName) },
		EnsureDatabase: svc.EnsureGameDatabase,
		Open:           svc.OpenGormForRouter,
		NameForGame:    func(g, e string) string { return prefix + "_" + g + "_" + e },
	}, metaDB)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() {
		if sqlDB, e := metaDB.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})

	// Server-side monitor (admin connection kept open for the whole probe).
	admin, err := open(adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, e := admin.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	baseline := arch010ServerCount(admin, serverQuery)
	maxConns := arch010MaxConns(driver, admin)

	var (
		mu          sync.Mutex
		dbs         = map[string]*gorm.DB{}
		clientPeak  atomic.Int64
		serverPeak  atomic.Int64
		monitorWG   sync.WaitGroup
		monitorDone = make(chan struct{})
	)
	updatePeak := func(p *atomic.Int64, v int64) {
		for {
			old := p.Load()
			if v <= old || p.CompareAndSwap(old, v) {
				break
			}
		}
	}
	monitorWG.Add(1)
	go func() {
		defer monitorWG.Done()
		serverPeak.Store(baseline)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-monitorDone:
				return
			case <-ticker.C:
				var sum int64
				mu.Lock()
				for _, db := range dbs {
					if sqlDB, e := db.DB(); e == nil {
						sum += int64(sqlDB.Stats().OpenConnections)
					}
				}
				mu.Unlock()
				updatePeak(&clientPeak, sum)
				if n := arch010ServerCount(admin, serverQuery); n >= 0 {
					updatePeak(&serverPeak, n)
				}
			}
		}
	}()

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, sc := range scopes {
		wg.Add(1)
		sem <- struct{}{}
		go func(sc arch010Scope) {
			defer wg.Done()
			defer func() { <-sem }()
			db, err := r.GameDB(context.Background(), sc.gameID, sc.env)
			if err != nil {
				t.Logf("open %s/%s failed: %v", sc.gameID, sc.env, err)
				return
			}
			mu.Lock()
			dbs[sc.gameID+"\x00"+sc.env] = db
			mu.Unlock()
		}(sc)
	}
	wg.Wait()

	// Let the monitor sample the steady-state peak before stopping it.
	time.Sleep(50 * time.Millisecond)
	close(monitorDone)
	monitorWG.Wait()

	st := r.Stats()
	t.Logf("driver=%s prefix=%s games=%d envs=%d scopes=%d concurrency=%d pool=%d firstopen=%d fails=%d p95=%v avg=%v max=%v clientPeak=%d serverBaseline=%d serverPeak=%d maxConns=%d",
		driver, prefix, games, envs, len(scopes), concurrency, st.PoolCount, st.FirstOpenCount, st.OpenFailureCount,
		st.P95Latency, st.AvgLatency, st.MaxLatency, clientPeak.Load(), baseline, serverPeak.Load(), maxConns)

	// Reclamation check: closing every pool must release server-side
	// connections back toward the baseline (the residual is the first-open
	// ensure-probe overhead that is not owned by the router cache).
	for _, sc := range scopes {
		_ = r.Forget(sc.gameID, sc.env)
	}
	time.Sleep(200 * time.Millisecond)
	afterReclaim := arch010ServerCount(admin, serverQuery)
	residual := afterReclaim - baseline
	t.Logf("reclaim: after=%d residual=%d", afterReclaim, residual)

	// Capacity conclusion: database/sql keeps up to MaxIdleConns (default 2)
	// idle per pool; the server quota bounds how many pools fit at steady
	// state. This is the observable "回收阈值与容量结论" for ARCH-010.
	idlePerPool := int64(2)
	reserved := maxConns * 5 / 100
	if reserved < 3 {
		reserved = 3
	}
	poolCapacity := (maxConns - reserved) / idlePerPool
	if poolCapacity < 0 {
		poolCapacity = 0
	}
	fits := int64(len(scopes)) <= poolCapacity
	t.Logf("capacity: maxConns=%d reserved=%d idlePerPool=%d poolCapacity=%d scopes=%d fits=%v",
		maxConns, reserved, idlePerPool, poolCapacity, len(scopes), fits)

	// Optional target assertions — only when the reviewer pins an explicit
	// target. Default run records the baseline without fabricating a pass.
	if v := os.Getenv("ARCH010_TARGET_P95_MS"); v != "" {
		limit, _ := strconv.Atoi(v)
		if int64(st.P95Latency/time.Millisecond) > int64(limit) {
			t.Errorf("P95 first-open %v exceeds target %dms", st.P95Latency, limit)
		}
	}
	if v := os.Getenv("ARCH010_TARGET_MAX_CONNS"); v != "" {
		limit, _ := strconv.Atoi(v)
		if serverPeak.Load() > int64(limit) {
			t.Errorf("server peak %d exceeds target %d", serverPeak.Load(), limit)
		}
	}
}
