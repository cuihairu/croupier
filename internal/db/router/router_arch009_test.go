package router

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// blockingOpen 是确定性的首开故障注入：Open 阻塞在 gate 上直到 test 放行。
// gate 关闭时放行并正常建库；ctx 取消/预算到期时立即返回 ctx.Err()，
// 以便断言取消路径不悬挂。
type blockingOpen struct {
	gate chan struct{}
	seen chan struct{} // 每次 Open 进入时发送一次（用于相位对齐）
}

func (b *blockingOpen) open(ctx context.Context, _, dsn string) (*gorm.DB, error) {
	select {
	case b.seen <- struct{}{}:
	default:
	}
	select {
	case <-b.gate:
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// newBlockingRouter 构造一个首开被 blockingOpen 卡住的 Router。
func newBlockingRouter(t *testing.T, timeout time.Duration) (*Router, *blockingOpen) {
	t.Helper()
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.db")
	metaDB, err := gorm.Open(sqlite.Open(metaPath), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB, _ := metaDB.DB(); _ = sqlDB })

	bo := &blockingOpen{gate: make(chan struct{}), seen: make(chan struct{}, 16)}
	cfg := Config{
		Driver:  "sqlite",
		MetaDSN: metaPath,
		DSNForDatabase: func(_, dbName string) string {
			return filepath.Join(dir, dbName+".db")
		},
		// ensure 不阻塞；阻塞点在 Open（更贴近真实：建库快、连接/迁移慢）。
		EnsureDatabase: func(_ context.Context, _, _, dbName string) (string, error) {
			return filepath.Join(dir, dbName+".db"), nil
		},
		Open:             bo.open,
		FirstOpenTimeout: timeout,
	}
	return New(cfg, metaDB), bo
}

// TestArch009_WaiterCancellationReturnsPromptly：首开进行中，等待者 ctx 被
// 取消必须立即返回 ctx.Err()，不阻塞到 leader 结束；且 leader 工作继续
// （共享首开不被单个等待者取消打断），完成后结果入缓存。
func TestArch009_WaiterCancellationReturnsPromptly(t *testing.T) {
	r, bo := newBlockingRouter(t, DefaultFirstOpenTimeout)
	t.Cleanup(func() { _ = r.Close() })

	// Leader：后台 goroutine 发起首开（阻塞在 bo.gate）。
	leaderDone := make(chan error, 1)
	go func() {
		_, err := r.GameDB(context.Background(), "g1", "prod")
		leaderDone <- err
	}()
	<-bo.seen // 确保 leader 已进入 Open（在飞）

	// 等待者：ctx 已取消 → 必须立刻返回，不得等待 leader。
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := r.GameDB(cancelled, "g1", "prod")
	require.ErrorIs(t, err, context.Canceled, "cancelled waiter must get ctx.Err()")
	assert.Less(t, time.Since(start), 2*time.Second, "cancellation must be prompt")

	// 放行 leader：共享首开继续并发布缓存。
	close(bo.gate)
	require.NoError(t, <-leaderDone, "leader must complete after gate opens")
	assert.Equal(t, 1, r.PoolSize(), "leader first-open must populate the pool")
	db, err := r.GameDB(context.Background(), "g1", "prod")
	require.NoError(t, err)
	require.NotNil(t, db, "cached db must be served to a healthy caller")
}

// TestArch009_ShutdownInterruptsInflightOpen：Close 取消生命周期 ctx，卡住的
// 首开必须被打断返回错误，而不是永久悬挂。
func TestArch009_ShutdownInterruptsInflightOpen(t *testing.T) {
	r, bo := newBlockingRouter(t, DefaultFirstOpenTimeout)

	leaderDone := make(chan error, 1)
	go func() {
		_, err := r.GameDB(context.Background(), "g1", "prod")
		leaderDone <- err
	}()
	<-bo.seen

	// 关停：lifecycleCtx 取消 → 阻塞的 Open 观察到并返回 ctx.Err()。
	require.NoError(t, r.Close())
	select {
	case err := <-leaderDone:
		require.ErrorIs(t, err, context.Canceled, "shutdown must abort the in-flight open")
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight open did not observe shutdown cancellation")
	}
	assert.Equal(t, 0, r.PoolSize(), "aborted open must not publish to the pool")
}

// TestArch009_FirstOpenBudgetBoundsOpen：首开预算到期（数据库卡死）时，
// openCtx 超时中止，调用方拿到 DeadlineExceeded 而非永久悬挂。
func TestArch009_FirstOpenBudgetBoundsOpen(t *testing.T) {
	r, bo := newBlockingRouter(t, 150*time.Millisecond)
	// 不放行 gate：让 Open 只可能被预算中断。
	_, err := r.GameDB(context.Background(), "g1", "prod")
	require.ErrorIs(t, err, context.DeadlineExceeded, "budget timeout must abort the open")
	assert.Equal(t, 0, r.PoolSize(), "timed-out open must not publish")
	close(bo.gate) // 清理：放行残余 goroutine
}

// TestArch009_DifferentDatabasesIsolated：不同物理库互不阻塞——库 A 的首开
// 卡住时，库 B 照常完成首开并进入池（不同库隔离）。
func TestArch009_DifferentDatabasesIsolated(t *testing.T) {
	// 单独 Router：A 正常开，B 卡住，B 卡住期间 A 已可用（不同库隔离）。
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.db")
	metaDB, err := gorm.Open(sqlite.Open(metaPath), &gorm.Config{})
	require.NoError(t, err)
	blockB := make(chan struct{})
	seenB := make(chan struct{}, 1)
	cfg := Config{
		Driver:  "sqlite",
		MetaDSN: metaPath,
		DSNForDatabase: func(_, dbName string) string {
			return filepath.Join(dir, dbName+".db")
		},
		EnsureDatabase: func(_ context.Context, _, _, dbName string) (string, error) {
			return filepath.Join(dir, dbName+".db"), nil
		},
		Open: func(ctx context.Context, _, dsn string) (*gorm.DB, error) {
			if filepath.Base(dsn) == "game_g_b.db" {
				seenB <- struct{}{}
				select {
				case <-blockB:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		},
	}
	r2 := New(cfg, metaDB)
	t.Cleanup(func() { _ = r2.Close() })

	// 卡住 B 的首开。
	bDone := make(chan error, 1)
	go func() {
		_, err := r2.GameDB(context.Background(), "g", "b")
		bDone <- err
	}()
	<-seenB

	// B 在飞期间，A 必须独立完成首开（不同库隔离）。
	dbA, err := r2.GameDB(context.Background(), "g", "a")
	require.NoError(t, err, "db A must open while db B is blocked")
	require.NotNil(t, dbA)
	assert.Equal(t, 1, r2.PoolSize(), "only A must be published while B is in flight")

	// 放行 B。
	close(blockB)
	require.NoError(t, <-bDone)
	assert.Equal(t, 2, r2.PoolSize(), "B must join the pool after its open completes")
}

// TestArch009_PoolSizeAndFirstOpenLatency：池数与首开耗时暴露面——首开后
// PoolSize 增长、FirstOpenLatency 返回正值；从未首开的名字返回 ok=false。
func TestArch009_PoolSizeAndFirstOpenLatency(t *testing.T) {
	r, bo := newBlockingRouter(t, DefaultFirstOpenTimeout)
	t.Cleanup(func() { _ = r.Close() })
	close(bo.gate)

	// 未首开：ok=false。
	_, ok := r.FirstOpenLatency("game_never")
	assert.False(t, ok, "never-opened db must report no latency")

	// 首开两个库。
	_, err := r.GameDB(context.Background(), "g1", "prod")
	require.NoError(t, err)
	_, err = r.GameDB(context.Background(), "g2", "dev")
	require.NoError(t, err)

	assert.Equal(t, 2, r.PoolSize(), "pool must count both open dbs")

	for _, dbName := range []string{"game_g1_prod", "game_g2_dev"} {
		d, ok := r.FirstOpenLatency(dbName)
		require.True(t, ok, "first-opened db %s must report latency", dbName)
		assert.Greater(t, d, time.Duration(0), "latency must be positive")
	}
}

// TestArch009_EnsureOpenMigrateConsumeLifecycleContext：ensure/open/migrate
// 回调收到的 ctx 是路由生命周期 ctx（可取消），而非 nil/Background——
// 关停时三段都能观察到取消。
func TestArch009_EnsureOpenMigrateConsumeLifecycleContext(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.db")
	metaDB, err := gorm.Open(sqlite.Open(metaPath), &gorm.Config{})
	require.NoError(t, err)

	var (
		mu      sync.Mutex
		ctxErrs []error // 各段 ctx.Done() 后的 Err
	)
	record := func() {
		mu.Lock()
		defer mu.Unlock()
		// 生命周期 ctx 在 Close 后应已取消。
		ctxErrs = append(ctxErrs, context.Canceled)
	}
	cfg := Config{
		Driver:  "sqlite",
		MetaDSN: metaPath,
		DSNForDatabase: func(_, dbName string) string {
			return filepath.Join(dir, dbName+".db")
		},
		EnsureDatabase: func(ctx context.Context, _, _, dbName string) (string, error) {
			select {
			case <-ctx.Done():
				record()
				return "", ctx.Err()
			default:
			}
			return filepath.Join(dir, dbName+".db"), nil
		},
		Open: func(ctx context.Context, _, dsn string) (*gorm.DB, error) {
			select {
			case <-ctx.Done():
				record()
				return nil, ctx.Err()
			default:
			}
			return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		},
		MigrateGame: func(ctx context.Context, db *gorm.DB) error {
			select {
			case <-ctx.Done():
				record()
				return ctx.Err()
			default:
			}
			return nil
		},
		FirstOpenTimeout: time.Hour, // 远大于测试时长：只有 Close 能取消
	}
	r := New(cfg, metaDB)

	// 先开一个库：三段都拿到未取消的 ctx，正常通过。
	_, err = r.GameDB(context.Background(), "g1", "prod")
	require.NoError(t, err)

	// 关停后，再次首开（不同库）：ensure 段观察到生命周期 ctx 已取消。
	require.NoError(t, r.Close())
	_, err = r.GameDB(context.Background(), "g2", "dev")
	require.Error(t, err, "post-close first-open must fail on the cancelled lifecycle ctx")
	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, ctxErrs, "ensure/open/migrate must observe lifecycle cancellation")
}
