package cluster

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Lifecycle manages this instance's cluster lifecycle: registration, lease
// renewal, discovery, and exit.
type Lifecycle struct {
	cfg    Config
	member Membership
	mesh   *MeshInterconnect
	stopCh chan struct{}
	doneCh chan struct{}
	epoch  uint64
}

// Config is Lifecycle's runtime parameters (normalized from
// config.ClusterConfig).
type Config struct {
	InstanceID        string
	AdvertiseAddr     string
	HeartbeatInterval time.Duration
	LeaseTTL          time.Duration
	PeerPollInterval  time.Duration
	// Readiness is the readiness surface (ARCH-004): registration/renewal
	// status is written to it; nil = not tracked (single instance, or callers
	// without a readiness probe).
	Readiness *Readiness
}

// NormalizeConfig normalizes config.ClusterConfig into Lifecycle parameters,
// filling in defaults.
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

// Start registers the member, starts the heartbeat/discovery loops, and
// returns the interconnect handle.
// It returns nil when disabled or advertiseAddr is missing (single-instance
// behavior).
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
		cfg.Readiness.SetJoined(false)
		cfg.Readiness.RecordDegrade("register: " + err.Error())
		return nil
	}
	mesh.SetEpoch(epoch)
	cfg.Readiness.SetJoined(true)

	lc := &Lifecycle{
		cfg:    cfg,
		member: member,
		mesh:   mesh,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
		epoch:  epoch,
	}
	go lc.loop()
	slog.Info("cluster: joined", "instanceId", cfg.InstanceID,
		"advertiseAddr", cfg.AdvertiseAddr, "epoch", epoch)
	return lc
}

// Mesh returns the interconnect handle (nil = single instance).
func (lc *Lifecycle) Mesh() *MeshInterconnect { return lc.mesh }

// Epoch returns this instance's term number.
func (lc *Lifecycle) Epoch() uint64 { return lc.epoch }

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
				lc.cfg.Readiness.RenewFail(err)
			} else {
				lc.cfg.Readiness.RenewOK()
			}
		case <-poll.C:
			lc.mesh.RefreshPeers(context.Background())
		}
	}
}

// Stop exits gracefully: stops the loops, closes connections, and removes the
// member record.
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
