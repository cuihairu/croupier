package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/core/backoff"
)

// 退避旋钮回归（原 newReconnectBackOff/newSyncBackOff 断言随 K2 上收迁移至
// core/backoff 与 core/register 默认档）：此处只断言 core/backoff 对外构造器
// 仍产出 sidecar 历史同款参数，防上收后默认档漂移。

func TestCoreBackoffMatchesLegacyAgentKnobs(t *testing.T) {
	d := backoff.Exponential(5*time.Second, 60*time.Second, 1.5)
	require.Equal(t, 5*time.Second, d.InitialInterval)
	require.Equal(t, 1.5, d.Multiplier)
	require.Equal(t, 60*time.Second, d.MaxInterval)
	require.Equal(t, 0.5, d.RandomizationFactor)

	s := backoff.Exponential(200*time.Millisecond, 2*time.Second, 0)
	require.Equal(t, 200*time.Millisecond, s.InitialInterval)
	require.Equal(t, 2*time.Second, s.MaxInterval)
}
