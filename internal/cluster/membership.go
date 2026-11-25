package cluster

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// InstanceRecord is the member table in the shared directory
// (croupier_meta.instances).
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

// DBMembership implements the member table on gorm (shared meta database).
type DBMembership struct {
	db       *gorm.DB
	leaseTTL time.Duration
}

// NewDBMembership creates the member-table operations. leaseTTL <= 0 falls
// back to the default 15s.
func NewDBMembership(db *gorm.DB, leaseTTL time.Duration) *DBMembership {
	if leaseTTL <= 0 {
		leaseTTL = 15 * time.Second
	}
	return &DBMembership{db: db, leaseTTL: leaseTTL}
}

// EnsureTable creates the table (idempotent; the migration baseline or this
// fallback covers it).
func (m *DBMembership) EnsureTable(ctx context.Context) error {
	return m.db.WithContext(ctx).AutoMigrate(&InstanceRecord{})
}

// Register self-registers: a new instance gets epoch=1; a restart reusing the
// same instanceId gets epoch+1.
func (m *DBMembership) Register(ctx context.Context, info PeerInfo) (uint64, error) {
	now := time.Now().UTC()
	expire := now.Add(m.leaseTTL)

	var existing InstanceRecord
	err := m.db.WithContext(ctx).
		Where("instance_id = ?", info.InstanceID).
		First(&existing).Error
	switch {
	case err == nil:
		// Restart reusing the ID: epoch increments (fencing token), address and
		// lease are reset.
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

// Renew renews the lease (a successful renewal after lease expiry = network
// partition recovery, with epoch unchanged).
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

// ListAlive returns members whose leases have not expired.
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

// Resign exits gracefully: deletes this instance's own record.
func (m *DBMembership) Resign(ctx context.Context, instanceID string) error {
	return m.db.WithContext(ctx).
		Where("instance_id = ?", instanceID).
		Delete(&InstanceRecord{}).Error
}
