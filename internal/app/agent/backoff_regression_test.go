package agent

import (
	"context"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/stretchr/testify/require"
)

// 能力矩阵 #1（Batch A）回归：退避参数在注入点钉死，不做真实时序等待
// （prefer-injection-over-timing-tests）。

func TestNewReconnectBackOff_Params(t *testing.T) {
	b := newReconnectBackOff()
	require.Equal(t, 5*time.Second, b.InitialInterval)
	require.Equal(t, 1.5, b.Multiplier)
	require.Equal(t, 60*time.Second, b.MaxInterval)
	require.Equal(t, 0.5, b.RandomizationFactor)
}

func TestNewSyncBackOff_Params(t *testing.T) {
	b := newSyncBackOff()
	require.Equal(t, 200*time.Millisecond, b.InitialInterval)
	require.Equal(t, 2*time.Second, b.MaxInterval)
}

// 抖动窗 [d×(1-r), d×(1+r)]，r=0.5：任何取值不越过 [Initial/2, Max×1.5]；
// Multiplier=1.5 下 40 次抽样必然抵达 MaxInterval（最大取值 ≥ Max/2）。
func TestNewReconnectBackOff_SequenceGrowsWithinBounds(t *testing.T) {
	b := newReconnectBackOff()
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

func TestSleepBackoff_CancelledContextReturnsFalse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.False(t, sleepBackoff(ctx, time.Hour))
}

func TestSleepBackoff_StopAndZeroNoWait(t *testing.T) {
	require.True(t, sleepBackoff(context.Background(), backoff.Stop))
	require.True(t, sleepBackoff(context.Background(), 0))
}

func TestSleepBackoff_ReturnsTrueAfterWait(t *testing.T) {
	require.True(t, sleepBackoff(context.Background(), time.Millisecond))
}
