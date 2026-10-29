package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Lifecycle 管理本实例的集群生命周期：注册、续租、发现、退出。
type Lifecycle struct {
	cfg    Config
	member Membership
	mesh   *MeshInterconnect
	stopCh chan struct{}
	doneCh chan struct{}
	epoch  uint64

	// 租约健康跟踪（ARCH-004 就绪探针）：注册成功后置健康，续租失败/超
	// TTL 未续即报告不健康。mu 护三个字段（loop 协程写、探针读）。
	mu           sync.Mutex
	lastRenewOK  bool
	lastRenewErr error
	lastRenewAt  time.Time
}

// Config 是 Lifecycle 的运行参数（由 config.ClusterConfig 归一化）。
type Config struct {
	InstanceID        string
	AdvertiseAddr     string
	HeartbeatInterval time.Duration
	LeaseTTL          time.Duration
	PeerPollInterval  time.Duration
}

// NormalizeConfig 把 config.ClusterConfig 归一为 Lifecycle 参数并补默认值。
func NormalizeConfig(in struct {
	Enabled           bool
	InstanceID        string
	AdvertiseAddr     string
	HeartbeatInterval string
	LeaseTTL          string
	PeerPollInterval  string
}) (Config, error) {
	cfg := Config{
		InstanceID:        in.InstanceID,
		AdvertiseAddr:     in.AdvertiseAddr,
		HeartbeatInterval: parseDurationDefault(in.HeartbeatInterval, 5*time.Second),
		LeaseTTL:          parseDurationDefault(in.LeaseTTL, 15*time.Second),
		PeerPollInterval:  parseDurationDefault(in.PeerPollInterval, 10*time.Second),
	}
	if cfg.InstanceID == "" {
		cfg.InstanceID = "server-" + uuid.NewString()[:8]
	}
	return cfg, nil
}

func parseDurationDefault(v string, def time.Duration) time.Duration {
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// Start 注册成员、启动心跳/发现循环并返回互联句柄。
// 未启用/缺 advertiseAddr 时返回 nil（单实例行为）。
func Start(ctx context.Context, cfg Config, member Membership, resolver OwnerStore, dial dialFunc) *Lifecycle {
	if member == nil || cfg.AdvertiseAddr == "" {
		return nil
	}

	self := PeerInfo{
		InstanceID:    cfg.InstanceID,
		AdvertiseAddr: cfg.AdvertiseAddr,
		StartedAt:     time.Now().UTC(),
	}
	mesh := NewMeshInterconnect(self, resolver, member)
	if dial != nil {
		mesh.dial = dial
	}
	if resolver != nil {
		resolver.SetMesh(mesh)
	}

	epoch, err := member.Register(ctx, self)
	if err != nil {
		slog.Error("cluster: register failed, running standalone", "error", err)
		return nil
	}
	mesh.SetEpoch(epoch)

	lc := &Lifecycle{
		cfg:         cfg,
		member:      member,
		mesh:        mesh,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
		epoch:       epoch,
		lastRenewOK: true, // 刚 Register 成功，租约视为有效直至首个续租失败
		lastRenewAt: time.Now().UTC(),
	}
	go lc.loop()
	slog.Info("cluster: joined", "instanceId", cfg.InstanceID,
		"advertiseAddr", cfg.AdvertiseAddr, "epoch", epoch)
	return lc
}

// Mesh 返回互联句柄（nil = 单实例）。
func (lc *Lifecycle) Mesh() *MeshInterconnect { return lc.mesh }

// Epoch 返回本实例任期号。
func (lc *Lifecycle) Epoch() uint64 { return lc.epoch }

// Healthy 报告集群依赖健康（ARCH-004 就绪探针）：已注册且租约续期未失败
// 未超 TTL。返回 nil 表示健康，否则为原因。nil 接收者视为「未要求集群」
// （单实例），恒健康。
func (lc *Lifecycle) Healthy() error {
	if lc == nil {
		return nil
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if !lc.lastRenewOK {
		if lc.lastRenewErr != nil {
			return fmt.Errorf("cluster: lease renew failing: %w", lc.lastRenewErr)
		}
		return fmt.Errorf("cluster: lease not yet renewed")
	}
	if age := time.Since(lc.lastRenewAt); age > lc.cfg.LeaseTTL {
		return fmt.Errorf("cluster: lease expired (%s since last renew, TTL %s)", age, lc.cfg.LeaseTTL)
	}
	return nil
}

func (lc *Lifecycle) loop() {
	defer close(lc.doneCh)
	heartbeat := time.NewTicker(lc.cfg.HeartbeatInterval)
	defer heartbeat.Stop()
	poll := time.NewTicker(lc.cfg.PeerPollInterval)
	defer poll.Stop()
	for {
		select {
		case <-lc.stopCh:
			return
		case <-heartbeat.C:
			if err := lc.member.Renew(context.Background(), lc.cfg.InstanceID); err != nil {
				slog.Warn("cluster: renew failed", "error", err)
				lc.mu.Lock()
				lc.lastRenewOK = false
				lc.lastRenewErr = err
				lc.mu.Unlock()
			} else {
				lc.mu.Lock()
				lc.lastRenewOK = true
				lc.lastRenewErr = nil
				lc.lastRenewAt = time.Now().UTC()
				lc.mu.Unlock()
			}
		case <-poll.C:
			lc.mesh.RefreshPeers(context.Background())
		}
	}
}

// Stop 优雅退出：停循环、关连接、清成员记录。
func (lc *Lifecycle) Stop(ctx context.Context) {
	if lc == nil {
		return
	}
	close(lc.stopCh)
	<-lc.doneCh
	lc.mesh.Close()
	if err := lc.member.Resign(ctx, lc.cfg.InstanceID); err != nil {
		slog.Warn("cluster: resign failed", "error", err)
	}
	slog.Info("cluster: left", "instanceId", lc.cfg.InstanceID)
}
