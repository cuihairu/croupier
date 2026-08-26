package cluster

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// AgentOwnerRecord 记录 agent 连接归属（谁持有该 Agent 的 TCP session）。
// 多实例模式下由持有连接的实例在 Agent 注册/重连时写入，断连时清除。
type AgentOwnerRecord struct {
	ID         uint      `gorm:"primaryKey"`
	AgentID    string    `gorm:"size:128;uniqueIndex;not null"`
	InstanceID string    `gorm:"size:64;index;not null"`
	OwnerEpoch uint64    `gorm:"not null;default:1"`
	GameID     string    `gorm:"size:64"`
	Env        string    `gorm:"size:64"`
	LastSeenAt time.Time `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (AgentOwnerRecord) TableName() string { return "cluster_agent_owners" }

// DBOwnerResolver 基于共享表的 owner 解析。
type DBOwnerResolver struct {
	db   *gorm.DB
	self *MeshInterconnect
	// ownerTTL：owner 记录的存活窗口（Agent 心跳续期；超时视为断连）。
	ownerTTL time.Duration
}

// NewDBOwnerResolver 创建解析器。ownerTTL <= 0 用默认 3 分钟
// （Agent 心跳 30s × 6 容忍）。
func NewDBOwnerResolver(db *gorm.DB, ownerTTL time.Duration) *DBOwnerResolver {
	if ownerTTL <= 0 {
		ownerTTL = 3 * time.Minute
	}
	return &DBOwnerResolver{db: db, ownerTTL: ownerTTL}
}

// EnsureTable 建表（幂等）。
func (r *DBOwnerResolver) EnsureTable(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&AgentOwnerRecord{})
}

// ClaimOwner 声明本实例持有该 Agent（注册/重连时调用）。
// 幂等：覆盖写。epoch 取 owner 实例当前任期。
func (r *DBOwnerResolver) ClaimOwner(ctx context.Context, agentID, gameID, env string, instanceID string, epoch uint64) error {
	now := time.Now().UTC()
	var existing AgentOwnerRecord
	err := r.db.WithContext(ctx).Where("agent_id = ?", agentID).First(&existing).Error
	switch {
	case err == nil:
		updates := map[string]interface{}{
			"instance_id":  instanceID,
			"owner_epoch":  epoch,
			"game_id":      gameID,
			"env":          env,
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
			LastSeenAt: now,
		}
		return r.db.WithContext(ctx).Create(&rec).Error
	default:
		return err
	}
}

// Touch 续期（Agent 心跳路径调用）。
func (r *DBOwnerResolver) Touch(ctx context.Context, agentID string) error {
	return r.db.WithContext(ctx).Model(&AgentOwnerRecord{}).
		Where("agent_id = ? AND instance_id = ?", agentID, r.selfInstanceID()).
		Update("last_seen_at", time.Now().UTC()).Error
}

// Release 释放归属（Agent 断连时调用）。
func (r *DBOwnerResolver) Release(ctx context.Context, agentID string) error {
	return r.db.WithContext(ctx).
		Where("agent_id = ? AND instance_id = ?", agentID, r.selfInstanceID()).
		Delete(&AgentOwnerRecord{}).Error
}

func (r *DBOwnerResolver) selfInstanceID() string {
	if r.self != nil {
		return r.self.SelfInfo().InstanceID
	}
	return ""
}

// SetMesh 注入 mesh（解析 owner 时校验实例租约仍存活）。
func (r *DBOwnerResolver) SetMesh(m *MeshInterconnect) { r.self = m }

// ResolveOwner 返回持有该 Agent 的存活实例；无则 nil。
// 存活判定 = owner 记录未过期 AND 实例成员租约未过期（经 last-known peers
// 缓存交叉验证，避免转发到死实例）。
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

	// 交叉验证实例租约（last-known peers；自身直接返回）。
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
