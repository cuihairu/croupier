package configsync

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type payload struct {
	Items []string
}

func TestPullOnceAppliesFirstAndSkipsSameVersion(t *testing.T) {
	var applies atomic.Int32
	p := New[string](0,
		func(ctx context.Context) (*Versioned[string], error) {
			return &Versioned[string]{Version: "v1", Payload: "cfg"}, nil
		},
		func(ctx context.Context, s string) error {
			applies.Add(1)
			return nil
		},
	)

	applied, err := p.PullOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, "v1", p.LastVersion())

	applied, err = p.PullOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, applied, "same version must skip apply")
	assert.Equal(t, int32(1), applies.Load())
}

func TestPullOnceAppliesWhenVersionEmpty(t *testing.T) {
	var applies atomic.Int32
	p := New[string](0,
		func(ctx context.Context) (*Versioned[string], error) {
			return &Versioned[string]{Payload: "no-version"}, nil
		},
		func(ctx context.Context, s string) error { applies.Add(1); return nil },
	)
	for i := 0; i < 3; i++ {
		applied, err := p.PullOnce(context.Background())
		require.NoError(t, err)
		assert.True(t, applied)
	}
	assert.Equal(t, int32(3), applies.Load())
}

func TestPullOnceVersionCursorNotAdvancedOnApplyError(t *testing.T) {
	var calls atomic.Int32
	p := New[string](0,
		func(ctx context.Context) (*Versioned[string], error) {
			calls.Add(1)
			return &Versioned[string]{Version: "v1", Payload: "cfg"}, nil
		},
		func(ctx context.Context, s string) error { return errors.New("boom") },
	)
	// 应用失败：错误传播 + 游标不推进
	applied, err := p.PullOnce(context.Background())
	require.Error(t, err)
	assert.False(t, applied)
	assert.Equal(t, "", p.LastVersion())

	// 修复 apply 后同版本仍能生效
	p.apply = func(ctx context.Context, s string) error { return nil }
	applied, err = p.PullOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, 2, int(calls.Load()), "fetch called twice, version retried")
}

func TestPullOnceFetchErrorPropagates(t *testing.T) {
	hooked := make(chan error, 2)
	p := New[string](0,
		func(ctx context.Context) (*Versioned[string], error) {
			return nil, errors.New("network down")
		},
		func(ctx context.Context, s string) error { return nil },
		WithOnError[string](func(err error) { hooked <- err }),
	)
	applied, err := p.PullOnce(context.Background())
	require.Error(t, err)
	assert.False(t, applied)
	select {
	case e := <-hooked:
		assert.Contains(t, e.Error(), "network down")
	default:
		t.Fatal("onError hook not invoked")
	}
}

func TestPullOnceNilFetchResultAndUninitialized(t *testing.T) {
	p := New[string](0,
		func(ctx context.Context) (*Versioned[string], error) { return nil, nil },
		func(ctx context.Context, s string) error { return nil },
	)
	applied, err := p.PullOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, applied)

	var bare *Puller[string]
	_, err = bare.PullOnce(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "", bare.LastVersion())
	bare.Start(context.Background()) // 不 panic
}

func TestNewDefaultsAndOptions(t *testing.T) {
	p := New[string](0, nil, nil)
	assert.Equal(t, DefaultInterval, p.interval)
	p2 := New[string](time.Second, nil, nil, WithOnApply[string](func(string) {}))
	assert.Equal(t, time.Second, p2.interval)
}

func TestStartImmediateAndPeriodic(t *testing.T) {
	var mu sync.Mutex
	versions := []string{"v1", "v2", "v2", "v3"}
	idx := 0
	var applies atomic.Int32
	var appliedVersions sync.Map

	p := New[string](30*time.Millisecond,
		func(ctx context.Context) (*Versioned[string], error) {
			mu.Lock()
			defer mu.Unlock()
			v := versions[idx%len(versions)]
			idx++
			return &Versioned[string]{Version: v, Payload: "cfg"}, nil
		},
		func(ctx context.Context, s string) error {
			applies.Add(1)
			return nil
		},
		WithOnApply[string](func(v string) { appliedVersions.Store(v, true) }),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && applies.Load() < 3 {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	assert.GreaterOrEqual(t, int64(applies.Load()), int64(3), "immediate + periodic applies expected")
	_, ok := appliedVersions.Load("v3")
	assert.True(t, ok)
	assert.Equal(t, "v3", p.LastVersion())
}

func TestStartLoopSurvivesErrors(t *testing.T) {
	var fails atomic.Int64
	p := New[string](20*time.Millisecond,
		func(ctx context.Context) (*Versioned[string], error) {
			fails.Add(1)
			return nil, errors.New("flaky")
		},
		func(ctx context.Context, s string) error { return nil },
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && fails.Load() < 3 {
		time.Sleep(5 * time.Millisecond)
	}
	assert.GreaterOrEqual(t, fails.Load(), int64(3), "loop keeps polling after failures")
}
