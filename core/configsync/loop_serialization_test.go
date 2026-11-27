package configsync

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pullResult struct {
	applied bool
	err     error
}

func startPull(p *Puller[string]) <-chan pullResult {
	resCh := make(chan pullResult, 1)
	go func() {
		ok, err := p.PullOnce(context.Background())
		resCh <- pullResult{applied: ok, err: err}
	}()
	return resCh
}

func await(t *testing.T, pull <-chan pullResult) bool {
	t.Helper()
	res := <-pull
	require.NoError(t, res.err)
	return res.applied
}

// gatedFetch 是一份可控响应：fetch 进入时通知 entered，随后等待 gate 放行才
// 返回 version/payload。用于精确控制两轮拉取的完成顺序。
type gatedFetch struct {
	entered chan struct{}
	gate    chan struct{}
	version string
	payload string
}

func newGatedFetch(version, payload string) *gatedFetch {
	return &gatedFetch{
		entered: make(chan struct{}, 1),
		gate:    make(chan struct{}),
		version: version,
		payload: payload,
	}
}

func (f *gatedFetch) fetch(ctx context.Context) (*Versioned[string], error) {
	select {
	case f.entered <- struct{}{}:
	default:
	}
	select {
	case <-f.gate:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &Versioned[string]{Version: f.version, Payload: f.payload}, nil
}

func (f *gatedFetch) release() { close(f.gate) }

// applyLog 记录每次 apply 的内容与生效时读到的游标，用于验证内容-游标一致性。
type applyLog struct {
	mu      sync.Mutex
	entries []applyEntry
	errs    map[string]error
}

type applyEntry struct {
	payload string
	cursor  string
}

func newApplyLog() *applyLog { return &applyLog{errs: map[string]error{}} }

func (l *applyLog) apply(p *Puller[string]) ApplyFunc[string] {
	return func(ctx context.Context, payload string) error {
		l.mu.Lock()
		defer l.mu.Unlock()
		if err := l.errs[payload]; err != nil {
			return err
		}
		l.entries = append(l.entries, applyEntry{payload, p.LastVersion()})
		return nil
	}
}

func (l *applyLog) snapshot() []applyEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]applyEntry(nil), l.entries...)
}

// dispatchFetch 依次把第 N 次 fetch 分派给 gatedFetch 列表；用完的轮次复用最后一个。
func dispatchFetch(fetches []*gatedFetch) FetchFunc[string] {
	var mu sync.Mutex
	calls := 0
	return func(ctx context.Context) (*Versioned[string], error) {
		mu.Lock()
		i := calls
		calls++
		mu.Unlock()
		if i >= len(fetches) {
			i = len(fetches) - 1
		}
		return fetches[i].fetch(ctx)
	}
}

// TestOverlappingPulls_NewerCompletesFirst: 较新请求（第 2 次发起，版本
// "zulu"）先返回并生效；较旧请求（第 1 次发起，版本 "alpha"）迟到响应随后
// 到达，不得覆盖新配置。版本是不可排序的 opaque 字符串。
func TestOverlappingPulls_NewerCompletesFirst(t *testing.T) {
	old := newGatedFetch("alpha", "stale")
	newerFetch := newGatedFetch("zulu", "fresh")
	log := newApplyLog()
	var p *Puller[string]
	p = New[string](0, dispatchFetch([]*gatedFetch{old, newerFetch}), log.apply(p))

	oldPull := startPull(p)
	<-old.entered // 第 1 轮 fetch 已挂起
	newerPull := startPull(p)

	// 串行化下较新轮排队，旧轮必须先放行；放行旧轮后新轮 fetch 才进入。
	old.release()
	select {
	case <-newerFetch.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("newer fetch never started")
	}
	newerFetch.release()

	require.True(t, await(t, oldPull))
	require.True(t, await(t, newerPull))

	entries := log.snapshot()
	require.Len(t, entries, 2)
	assert.Equal(t, []string{"stale", "fresh"}, []string{entries[0].payload, entries[1].payload})
	assert.Equal(t, "zulu", p.LastVersion())
}

// TestOverlappingPulls_OlderCompletesFirst: 较旧轮先完成生效，较新轮随后落地；
// 最终配置与游标仍为较新轮。覆盖两种完成顺序中的另一种。
func TestOverlappingPulls_OlderCompletesFirst(t *testing.T) {
	old := newGatedFetch("alpha", "stale")
	newerFetch := newGatedFetch("zulu", "fresh")
	log := newApplyLog()
	var p *Puller[string]
	p = New[string](0, dispatchFetch([]*gatedFetch{old, newerFetch}), log.apply(p))

	oldPull := startPull(p)
	<-old.entered
	newerPull := startPull(p)
	old.release()
	select {
	case <-newerFetch.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("newer fetch never started")
	}
	newerFetch.release()

	require.True(t, await(t, oldPull))
	require.True(t, await(t, newerPull))
	entries := log.snapshot()
	require.Len(t, entries, 2)
	assert.Equal(t, []string{"stale", "fresh"}, []string{entries[0].payload, entries[1].payload})
	assert.Equal(t, "zulu", p.LastVersion())
}

// TestOverlappingPulls_LateResponseCannotOverwrite: 较新请求发起较晚但 fetch
// 立即返回，较旧请求发起较早但 fetch 缓慢。无串行化时较新配置先落地、较旧的
// 迟到响应随后覆盖；串行化较新轮必须等旧轮完成，最终较新配置胜出。
func TestOverlappingPulls_LateResponseCannotOverwrite(t *testing.T) {
	old := newGatedFetch("alpha", "stale") // 发起较早、响应慢
	newerImmediate := &Versioned[string]{Version: "zulu", Payload: "fresh"}
	log := newApplyLog()
	var p *Puller[string]

	fetchCalls := 0
	var mu sync.Mutex
	p = New[string](0, func(ctx context.Context) (*Versioned[string], error) {
		mu.Lock()
		call := fetchCalls
		fetchCalls++
		mu.Unlock()
		if call == 0 {
			return old.fetch(ctx)
		}
		return newerImmediate, nil // 较新轮 fetch 立即返回
	}, log.apply(p))

	oldPull := startPull(p)
	<-old.entered
	newerPull := startPull(p)

	// 给较新轮一个短窗口：无串行化时它会立即 apply；串行化时它被槽位阻塞。
	newerDone := false
	select {
	case res := <-newerPull:
		require.NoError(t, res.err)
		newerDone = true
	case <-time.After(150 * time.Millisecond):
	}

	// 放行较旧轮。若较新轮已完成（无串行化），较旧迟到响应随后覆盖；
	// 若较新轮在排队（串行化），旧轮先 apply，随后新轮 apply 并胜出。
	old.release()
	if newerDone {
		require.True(t, await(t, oldPull))
	} else {
		require.True(t, await(t, oldPull))
		require.True(t, await(t, newerPull))
	}

	entries := log.snapshot()
	require.Len(t, entries, 2, "both pulls apply")
	assert.Equal(t, "fresh", entries[len(entries)-1].payload,
		"late older response must not overwrite newer config")
	assert.Equal(t, "zulu", p.LastVersion(), "cursor must match newest applied config")
}

// TestOverlappingPulls_ContentCursorConsistency: apply 执行时游标未推进；apply
// 返回后才提交；同版本跳过 apply。
func TestOverlappingPulls_ContentCursorConsistency(t *testing.T) {
	log := newApplyLog()
	var p *Puller[string]
	p = New[string](0,
		func(ctx context.Context) (*Versioned[string], error) {
			return &Versioned[string]{Version: "zulu", Payload: "fresh"}, nil
		},
		log.apply(p),
	)

	require.True(t, await(t, startPull(p)))
	require.False(t, await(t, startPull(p)), "same version must skip apply")
	require.False(t, await(t, startPull(p)))

	entries := log.snapshot()
	require.Len(t, entries, 1)
	assert.Equal(t, "fresh", entries[0].payload)
	assert.Equal(t, "", entries[0].cursor, "cursor advances only after apply returns")
	assert.Equal(t, "zulu", p.LastVersion())
}

// TestOverlappingPulls_ApplyFailureAndRetry: apply 失败时游标不推进；较晚轮
// 不受失败影响；修复后同版本重试成功。
func TestOverlappingPulls_ApplyFailureAndRetry(t *testing.T) {
	old := newGatedFetch("alpha", "bad")
	newerFetch := newGatedFetch("zulu", "good")
	log := newApplyLog()
	log.errs["bad"] = errors.New("apply boom")
	var p *Puller[string]
	p = New[string](0, dispatchFetch([]*gatedFetch{old, newerFetch}), log.apply(p))

	oldPull := startPull(p)
	<-old.entered
	newerPull := startPull(p)

	// 旧轮 apply 失败；较新轮随后生效。
	old.release()
	resOld := <-oldPull
	require.Error(t, resOld.err)
	assert.False(t, resOld.applied)
	select {
	case <-newerFetch.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("newer fetch never started")
	}
	newerFetch.release()
	require.True(t, await(t, newerPull))
	assert.Equal(t, "zulu", p.LastVersion(), "failed apply did not corrupt newer pull")

	// 修复后重试失败的 alpha/bad（第三轮 fetch 复用 newerFetch 的已关闭 gate，
	// 因此立即返回 zulu/good；改为新 gate 保证重试拿到 alpha/bad）。
	retryFetch := newGatedFetch("alpha", "bad")
	retryFetch.release()
	var retryCalls int
	var mu sync.Mutex
	p.fetch = func(ctx context.Context) (*Versioned[string], error) {
		mu.Lock()
		retryCalls++
		mu.Unlock()
		return retryFetch.fetch(ctx)
	}
	delete(log.errs, "bad")
	require.True(t, await(t, startPull(p)))
	assert.Equal(t, "alpha", p.LastVersion(), "retry of failed version succeeds")
	assert.Equal(t, 1, retryCalls)
	entries := log.snapshot()
	assert.Equal(t, []string{"good", "bad"}, []string{entries[0].payload, entries[1].payload},
		"failed apply left cursor unchanged; retry applied the failed payload")
}

// TestPullOnce_ContextCancelWhileQueued: 排队等待串行槽位时取消 ctx 快速返回，
// 不死锁、不影响在途拉取。
func TestPullOnce_ContextCancelWhileQueued(t *testing.T) {
	gate := make(chan struct{})
	done := make(chan struct{})
	p := New[string](0,
		func(ctx context.Context) (*Versioned[string], error) {
			<-gate
			return &Versioned[string]{Version: "alpha", Payload: "hold"}, nil
		},
		func(ctx context.Context, s string) error { return nil },
	)
	go func() { _, _ = p.PullOnce(context.Background()); close(done) }()
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := p.PullOnce(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), time.Second, "queued pull returns promptly on cancel")

	close(gate)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight pull stuck after gate release")
	}
}

// TestPullOnce_CallbackReentrancyNoDeadlock: onApply 回调内重入 PullOnce 不死锁。
func TestPullOnce_CallbackReentrancyNoDeadlock(t *testing.T) {
	done := make(chan struct{})
	var secondApplied bool
	var secondErr error
	var p *Puller[string]
	p = New[string](0,
		func(ctx context.Context) (*Versioned[string], error) {
			return &Versioned[string]{Version: "alpha", Payload: "cfg"}, nil
		},
		func(ctx context.Context, s string) error { return nil },
		WithOnApply[string](func(string) {
			secondApplied, secondErr = p.PullOnce(context.Background())
			close(done)
		}),
	)
	require.True(t, await(t, startPull(p)))
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("reentrant PullOnce from onApply deadlocked")
	}
	assert.False(t, secondApplied, "same version must skip on reentrant pull")
	assert.NoError(t, secondErr)
}
