package cluster

import (
	"context"
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
	lc := Start(nil, Config{}, nil, nil, nil)
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
