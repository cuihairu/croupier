package backoff

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExponentialKnobs(t *testing.T) {
	b := Exponential(5*time.Second, 60*time.Second, 1.5)
	assert.Equal(t, 5*time.Second, b.InitialInterval)
	assert.Equal(t, 60*time.Second, b.MaxInterval)
	assert.Equal(t, 1.5, b.Multiplier)

	// multiplier<=0 沿用 cenkalti 默认 1.5，不自设 0（会原地打转）。
	d := Exponential(200*time.Millisecond, 2*time.Second, 0)
	assert.Equal(t, 1.5, d.Multiplier)
	assert.Equal(t, 200*time.Millisecond, d.InitialInterval)
	assert.Equal(t, 2*time.Second, d.MaxInterval)
}

// 能力矩阵 #1（Batch A）回归（随上收自 internal/app/agent 迁入）：退避参数
// 在注入点钉死，不做真实时序等待（prefer-injection-over-timing-tests）。
// 抖动窗 [d×(1-r), d×(1+r)]，r=0.5：任何取值不越过 [Initial/2, Max×1.5]；
// Multiplier=1.5 下 40 次抽样必然抵达 MaxInterval（最大取值 ≥ Max/2）。
func TestExponentialSequenceGrowsWithinBounds(t *testing.T) {
	b := Exponential(5*time.Second, 60*time.Second, 1.5)
	lo := 5 * time.Second / 2
	hi := 60 * time.Second * 3 / 2
	maxSeen := time.Duration(0)
	for i := 0; i < 40; i++ {
		d := b.NextBackOff()
		require.GreaterOrEqual(t, d, lo)
		require.LessOrEqual(t, d, hi)
		if d > maxSeen {
			maxSeen = d
		}
	}
	require.GreaterOrEqual(t, maxSeen, 60*time.Second/2)
}

func TestConstant(t *testing.T) {
	b := Constant(time.Second)
	for range 3 {
		assert.Equal(t, time.Second, b.NextBackOff())
	}
}

func TestSleepCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.False(t, Sleep(ctx, time.Hour))
	// backoff.Stop（-1）与零值同语义：不等待，仅看 ctx 存活。
	assert.True(t, Sleep(context.Background(), Stop))
	assert.False(t, Sleep(ctx, 0))
}

func TestSleepZeroWithLiveContext(t *testing.T) {
	assert.True(t, Sleep(context.Background(), 0))
}

func TestSleepElapsed(t *testing.T) {
	start := time.Now()
	assert.True(t, Sleep(context.Background(), 5*time.Millisecond))
	assert.Less(t, time.Since(start), time.Second)
}
