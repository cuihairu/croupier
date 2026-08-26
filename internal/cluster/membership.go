package cluster

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// InstanceRecord 是共享目录里的成员表（croupier_meta.instances）。
type InstanceRecord struct {
	ID            uint      `gorm:"primaryKey"`
	InstanceID    string    `gorm:"size:64;uniqueIndex"`
	AdvertiseAddr string    `gorm:"size:255;not null"`
	Epoch         uint64    `gorm:"not null;default:1"`
	StartedAt     time.Time `gorm:"not null"`
	LeaseExpireAt time.Time `gorm:"index;not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (InstanceRecord) TableName() string { return "cluster_instances" }

// DBMembership 基于 gorm（共享 meta 库）的成员表实现。
type DBMembership struct {
	db       *gorm.DB
	leaseTTL time.Duration
}

// NewDBMembership 创建成员表操作。leaseTTL <= 0 时用默认 15s。
func NewDBMembership(db *gorm.DB, leaseTTL time.Duration) *DBMembership {
	if leaseTTL <= 0 {
		leaseTTL = 15 * time.Second
	}
	return &DBMembership{db: db, leaseTTL: leaseTTL}
}

// EnsureTable 建表（幂等；由迁移基线或此处兜底）。
func (m *DBMembership) EnsureTable(ctx context.Context) error {
	return m.db.WithContext(ctx).AutoMigrate(&InstanceRecord{})
}

// Register 自注册：新实例 epoch=1；同 instanceId 重启则 epoch+1。
func (m *DBMembership) Register(ctx context.Context, info PeerInfo) (uint64, error) {
	now := time.Now().UTC()
	expire := now.Add(m.leaseTTL)

	var existing InstanceRecord
	err := m.db.WithContext(ctx).
		Where("instance_id = ?", info.InstanceID).
		First(&existing).Error
	switch {
	case err == nil:
		// 重启复用 ID：epoch 递增（fencing token），重置地址与租约。
		epoch := existing.Epoch + 1
		updates := map[string]interface{}{
			"advertise_addr":  info.AdvertiseAddr,
			"epoch":           epoch,
			"started_at":      now,
			"lease_expire_at": expire,
		}
		if err := m.db.WithContext(ctx).Model(&InstanceRecord{}).
			Where("instance_id = ?", info.InstanceID).
			Updates(updates).Error; err != nil {
			return 0, err
		}
		return epoch, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		rec := InstanceRecord{
			InstanceID:    info.InstanceID,
			AdvertiseAddr: info.AdvertiseAddr,
			Epoch:         1,
			StartedAt:     now,
			LeaseExpireAt: expire,
		}
		if err := m.db.WithContext(ctx).Create(&rec).Error; err != nil {
			return 0, err
		}
		return 1, nil
	default:
		return 0, err
	}
}

// Renew 续租（租约过期后续租成功 = 网络分区恢复，epoch 不变）。
func (m *DBMembership) Renew(ctx context.Context, instanceID string) error {
	res := m.db.WithContext(ctx).Model(&InstanceRecord{}).
		Where("instance_id = ?", instanceID).
		Update("lease_expire_at", time.Now().UTC().Add(m.leaseTTL))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("membership: instance record missing")
	}
	return nil
}

// ListAlive 返回租约未过期的成员。
func (m *DBMembership) ListAlive(ctx context.Context) ([]PeerInfo, error) {
	var recs []InstanceRecord
	err := m.db.WithContext(ctx).
		Where("lease_expire_at > ?", time.Now().UTC()).
		Order("instance_id ASC").
		Find(&recs).Error
	if err != nil {
		return nil, err
	}
	out := make([]PeerInfo, 0, len(recs))
	for _, r := range recs {
		out = append(out, PeerInfo{
			InstanceID:    r.InstanceID,
			AdvertiseAddr: r.AdvertiseAddr,
			Epoch:         r.Epoch,
			StartedAt:     r.StartedAt,
		})
	}
	return out, nil
}

// Resign 优雅退出：删除自身记录。
func (m *DBMembership) Resign(ctx context.Context, instanceID string) error {
	return m.db.WithContext(ctx).
		Where("instance_id = ?", instanceID).
		Delete(&InstanceRecord{}).Error
}
