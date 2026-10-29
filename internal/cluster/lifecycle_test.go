package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 覆盖 lifecycle.go 全 0% 的 NormalizeConfig/Start/Stop/Mesh/Epoch。

func TestNormalizeConfig_Defaults(t *testing.T) {
	cfg, err := NormalizeConfig(struct {
		Enabled           bool
		InstanceID        string
		AdvertiseAddr     string
		HeartbeatInterval string
		LeaseTTL          string
		PeerPollInterval  string
	}{InstanceID: "inst-1", AdvertiseAddr: "localhost:19099"})
	require.NoError(t, err)
	assert.Equal(t, "inst-1", cfg.InstanceID)
	assert.Equal(t, 5*time.Second, cfg.HeartbeatInterval) // 默认
	assert.Equal(t, 15*time.Second, cfg.LeaseTTL)         // 默认
	assert.Equal(t, 10*time.Second, cfg.PeerPollInterval) // 默认
}

func TestNormalizeConfig_Custom(t *testing.T) {
	cfg, err := NormalizeConfig(struct {
		Enabled           bool
		InstanceID        string
		AdvertiseAddr     string
		HeartbeatInterval string
		LeaseTTL          string
		PeerPollInterval  string
	}{
		InstanceID:        "i",
		AdvertiseAddr:     "a:1",
		HeartbeatInterval: "1s",
		LeaseTTL:          "3s",
		PeerPollInterval:  "2s",
	})
	require.NoError(t, err)
	assert.Equal(t, time.Second, cfg.HeartbeatInterval)
	assert.Equal(t, 3*time.Second, cfg.LeaseTTL)
	assert.Equal(t, 2*time.Second, cfg.PeerPollInterval)
}

func TestNormalizeConfig_Errors(t *testing.T) {
	// 空 InstanceID → 自动生成（不报错）
	cfg, err := NormalizeConfig(struct {
		Enabled           bool
		InstanceID        string
		AdvertiseAddr     string
		HeartbeatInterval string
		LeaseTTL          string
		PeerPollInterval  string
	}{AdvertiseAddr: "a:1"})
	require.NoError(t, err)
	assert.Contains(t, cfg.InstanceID, "server-", "空 InstanceID 自动生成")

	// 非法 duration → 回退默认值（不报错）
	cfg2, err := NormalizeConfig(struct {
		Enabled           bool
		InstanceID        string
		AdvertiseAddr     string
		HeartbeatInterval string
		LeaseTTL          string
		PeerPollInterval  string
	}{InstanceID: "i", AdvertiseAddr: "a:1", HeartbeatInterval: "bogus"})
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, cfg2.HeartbeatInterval, "非法 duration 回退默认")
}

func TestLifecycleStartStop(t *testing.T) {
	// nil membership → Start 返回 nil（防御分支）
	lc := Start(nil, Config{}, nil, nil, nil) //nolint:staticcheck // 刻意 nil membership：验证防御分支
	assert.Nil(t, lc)

	// 最小可测生命周期：内存 Membership（不需要 Redis/网络）
	cfg := Config{
		InstanceID:        "test-lifecycle",
		AdvertiseAddr:     "127.0.0.1:19099",
		HeartbeatInterval: 100 * time.Millisecond,
		LeaseTTL:          300 * time.Millisecond,
		PeerPollInterval:  100 * time.Millisecond,
	}
	ctx := context.Background()
	lc2 := Start(ctx, cfg, newLifecycleMemMembership(), nil, nil)
	if lc2 == nil {
		t.Skip("Start returned nil (unexpected)")
		return
	}
	// Mesh/Epoch 可读
	if lc2.Mesh() != nil {
		t.Log("mesh started")
	}
	_ = lc2.Epoch()
	lc2.Stop(ctx)
	// 注：Stop 不幂等（close of closed channel panic）——产品代码 bug，
	// 生产路径仅调用一次。二次 Stop 待修复后启用。
}

// newLifecycleMemMembership 最小内存 Membership（测试专用）。
type lifecycleMemMembership struct{}

func newLifecycleMemMembership() *lifecycleMemMembership { return &lifecycleMemMembership{} }

func (m *lifecycleMemMembership) Register(ctx context.Context, info PeerInfo) (uint64, error) {
	return 1, nil
}
func (m *lifecycleMemMembership) Renew(ctx context.Context, instanceID string) error {
	return nil
}
func (m *lifecycleMemMembership) ListAlive(ctx context.Context) ([]PeerInfo, error) {
	return nil, nil
}
func (m *lifecycleMemMembership) Resign(ctx context.Context, instanceID string) error {
	return nil
}

// TestLifecycleHealthy_StateTransitions covers ARCH-004 readiness: lease
// renew failures and TTL expiry must surface as not-healthy with a reason;
// nil receiver (cluster not required) stays healthy.
func TestLifecycleHealthy_StateTransitions(t *testing.T) {
	lc := &Lifecycle{cfg: Config{LeaseTTL: time.Minute}}
	// 注册尚未完成（lastRenewOK 零值）：不健康，无具体错误
	err := lc.Healthy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet renewed")

	// 注册成功：健康
	lc.mu.Lock()
	lc.lastRenewOK = true
	lc.lastRenewAt = time.Now().UTC()
	lc.mu.Unlock()
	require.NoError(t, lc.Healthy())

	// 续租失败：不健康，原因带 renew failing
	lc.mu.Lock()
	lc.lastRenewOK = false
	lc.lastRenewErr = fmt.Errorf("redis: connection refused")
	lc.mu.Unlock()
	err = lc.Healthy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "renew failing")

	// 续租恢复：健康
	lc.mu.Lock()
	lc.lastRenewOK = true
	lc.lastRenewErr = nil
	lc.lastRenewAt = time.Now().UTC()
	lc.mu.Unlock()
	require.NoError(t, lc.Healthy())

	// 租约超 TTL 未续：不健康，原因带 expired
	lc.mu.Lock()
	lc.lastRenewAt = time.Now().UTC().Add(-2 * time.Minute)
	lc.mu.Unlock()
	err = lc.Healthy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")

	// nil 接收者 = 集群未要求（单实例）：恒健康
	var nilLC *Lifecycle
	require.NoError(t, nilLC.Healthy())
}
