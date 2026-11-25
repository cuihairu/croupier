package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// AgentOwnerRecord records agent connection ownership (who holds that
// Agent's TCP session). In multi-instance mode the instance holding the
// connection writes it when the Agent registers/reconnects and clears it on
// disconnect.
type AgentOwnerRecord struct {
	ID         uint              `gorm:"primaryKey"`
	AgentID    string            `gorm:"size:128;uniqueIndex;not null"`
	InstanceID string            `gorm:"size:64;index;not null"`
	OwnerEpoch uint64            `gorm:"not null;default:1"`
	GameID     string            `gorm:"size:64"`
	Env        string            `gorm:"size:64"`
	Labels     string            `gorm:"type:text"`
	LabelsMap  map[string]string `gorm:"-:all"`
	LastSeenAt time.Time         `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (AgentOwnerRecord) TableName() string { return "cluster_agent_owners" }

// DBOwnerResolver resolves owners from the shared table.
type DBOwnerResolver struct {
	db             *gorm.DB
	self           *MeshInterconnect
	selfIDOverride string
	// ownerTTL: the liveness window of the owner record (renewed by Agent
	// heartbeats; expiry is treated as a disconnect).
	ownerTTL time.Duration
}

// NewDBOwnerResolver creates the resolver. ownerTTL <= 0 falls back to the
// default 3 minutes (tolerating 6 missed 30s Agent heartbeats).
func NewDBOwnerResolver(db *gorm.DB, ownerTTL time.Duration) *DBOwnerResolver {
	return newDBOwnerResolver(db, ownerTTL, "")
}

// NewDBOwnerResolverForSelf explicitly sets this instance's ID (for scenarios
// like heartbeat self-healing that need an instance identity but have no
// interconnect mesh; the production main path goes through Start/SetMesh).
func NewDBOwnerResolverForSelf(db *gorm.DB, ownerTTL time.Duration, selfID string) *DBOwnerResolver {
	return newDBOwnerResolver(db, ownerTTL, selfID)
}

func newDBOwnerResolver(db *gorm.DB, ownerTTL time.Duration, selfID string) *DBOwnerResolver {
	if ownerTTL <= 0 {
		ownerTTL = 3 * time.Minute
	}
	return &DBOwnerResolver{db: db, ownerTTL: ownerTTL, selfIDOverride: selfID}
}

// EnsureTable creates the table (idempotent).
func (r *DBOwnerResolver) EnsureTable(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&AgentOwnerRecord{})
}

// ClaimOwner claims that this instance holds the Agent (called on
// register/reconnect).
// Idempotent: overwrite. epoch takes the owner instance's current term.
func (r *DBOwnerResolver) ClaimOwner(ctx context.Context, agentID, gameID, env, instanceID string, epoch uint64, labels map[string]string) error {
	now := time.Now().UTC()
	var existing AgentOwnerRecord
	err := r.db.WithContext(ctx).Where("agent_id = ?", agentID).First(&existing).Error
	switch {
	case err == nil:
		if existing.GameID != gameID || existing.Env != env {
			return fmt.Errorf("agent %q is already bound to game_id=%q env=%q", agentID, existing.GameID, existing.Env)
		}
		updates := map[string]interface{}{
			"instance_id":  instanceID,
			"owner_epoch":  epoch,
			"gameId":       gameID,
			"env":          env,
			"labels":       encodeOwnerLabels(labels),
			"last_seen_at": now,
		}
		return r.db.WithContext(ctx).Model(&AgentOwnerRecord{}).
			Where("agent_id = ?", agentID).Updates(updates).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		rec := AgentOwnerRecord{
			AgentID:    agentID,
			InstanceID: instanceID,
			OwnerEpoch: epoch,
			GameID:     gameID,
			Env:        env,
			Labels:     encodeOwnerLabels(labels),
			LastSeenAt: now,
		}
		return r.db.WithContext(ctx).Create(&rec).Error
	default:
		return err
	}
}

// Touch renews the lease (called on the Agent heartbeat path).
func (r *DBOwnerResolver) Touch(ctx context.Context, agentID string) error {
	return r.db.WithContext(ctx).Model(&AgentOwnerRecord{}).
		Where("agent_id = ? AND instance_id = ?", agentID, r.selfInstanceID()).
		Update("last_seen_at", time.Now().UTC()).Error
}

// Release releases ownership (called when the Agent disconnects).
func (r *DBOwnerResolver) Release(ctx context.Context, agentID string) error {
	return r.db.WithContext(ctx).
		Where("agent_id = ? AND instance_id = ?", agentID, r.selfInstanceID()).
		Delete(&AgentOwnerRecord{}).Error
}

func (r *DBOwnerResolver) selfInstanceID() string {
	if r.selfIDOverride != "" {
		return r.selfIDOverride
	}
	if r.self != nil {
		return r.self.SelfInfo().InstanceID
	}
	return ""
}

// SetMesh injects the mesh (used to validate the instance lease is still
// alive when resolving owners).
func (r *DBOwnerResolver) SetMesh(m *MeshInterconnect) { r.self = m }

// ResolveOwner returns the live instance holding that Agent; nil if none.
// Liveness = the owner record has not expired AND the instance's member
// lease has not expired (cross-checked via the last-known peers cache, to
// avoid forwarding to a dead instance).
func (r *DBOwnerResolver) ResolveOwner(ctx context.Context, agentID string) (*PeerInfo, error) {
	var rec AgentOwnerRecord
	err := r.db.WithContext(ctx).
		Where("agent_id = ? AND last_seen_at > ?", agentID, time.Now().UTC().Add(-r.ownerTTL)).
		First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Cross-check the instance lease (last-known peers; return self directly).
	selfID := r.selfInstanceID()
	if rec.InstanceID == selfID {
		return &PeerInfo{InstanceID: rec.InstanceID, Epoch: rec.OwnerEpoch}, nil
	}
	if r.self != nil {
		for _, p := range r.self.Peers() {
			if p.InstanceID == rec.InstanceID && p.Epoch >= rec.OwnerEpoch {
				return &p, nil
			}
		}
		// last-known 里没有：owner 实例已死或未发现 → 视为无 owner
		//（Agent 重连到存活实例后会重新 Claim）。
		return nil, nil
	}
	return &PeerInfo{InstanceID: rec.InstanceID, Epoch: rec.OwnerEpoch}, nil
}

// CountAgentsByOwner 聚合每个实例持有的活跃 agent 数（ops 集群视图）。
func (r *DBOwnerResolver) CountAgentsByOwner(ctx context.Context) (map[string]int64, error) {
	type row struct {
		InstanceID string
		N          int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Model(&AgentOwnerRecord{}).
		Select("instance_id, COUNT(*) as n").
		Where("last_seen_at > ?", time.Now().UTC().Add(-r.ownerTTL)).
		Group("instance_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, rw := range rows {
		out[rw.InstanceID] = rw.N
	}
	return out, nil
}

// ListAliveOwners 返回 TTL 内的全部归属记录（集群模式下 agent 列表的
// 全集来源——本地 registry 只含连到本实例的 agent，跨实例视图必须以
// 共享归属表为准，避免入口分流到不同实例时列表随机变化）。
func (r *DBOwnerResolver) ListAliveOwners(ctx context.Context) ([]AgentOwnerRecord, error) {
	var rows []AgentOwnerRecord
	err := r.db.WithContext(ctx).
		Where("last_seen_at > ?", time.Now().UTC().Add(-r.ownerTTL)).
		Order("agent_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].LabelsMap = decodeOwnerLabels(rows[i].Labels)
	}
	return rows, nil
}

func encodeOwnerLabels(labels map[string]string) string {
	if labels == nil {
		return "{}"
	}
	encoded, err := json.Marshal(labels)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func decodeOwnerLabels(encoded string) map[string]string {
	labels := map[string]string{}
	if encoded == "" {
		return labels
	}
	_ = json.Unmarshal([]byte(encoded), &labels)
	return labels
}

// FindOwner 返回归属记录（不受 TTL 过滤——心跳自愈需要回读真实 scope，
// 即使记录已临期）。不存在时返回 nil, nil。
func (r *DBOwnerResolver) FindOwner(ctx context.Context, agentID string) (*AgentOwnerRecord, error) {
	var rec AgentOwnerRecord
	err := r.db.WithContext(ctx).
		Where("agent_id = ?", agentID).
		First(&rec).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	rec.LabelsMap = decodeOwnerLabels(rec.Labels)
	return &rec, nil
}

// SelfOwnerScope 返回 agent 的归属 scope，仅当本实例是 Claim 持有者时
// ok=true（心跳自愈只允许本实例重建会话）。
func (r *DBOwnerResolver) SelfOwnerScope(ctx context.Context, agentID string) (gameID, env string, ok bool) {
	var rec AgentOwnerRecord
	err := r.db.WithContext(ctx).
		Where("agent_id = ?", agentID).
		First(&rec).Error
	if err != nil || rec.InstanceID != r.selfInstanceID() {
		return "", "", false
	}
	return rec.GameID, rec.Env, true
}
