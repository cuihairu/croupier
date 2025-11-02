package messagesgorm

import (
    "context"
    "time"
    "gorm.io/gorm"
)

type Repo struct { db *gorm.DB }

func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

func (r *Repo) Create(ctx context.Context, m *MessageRecord) error { return r.db.WithContext(ctx).Create(m).Error }

// List returns messages for a user; if unreadOnly is true, only unread.
func (r *Repo) List(ctx context.Context, userID uint, unreadOnly bool, limit, offset int) ([]*MessageRecord, int64, error) {
    q := r.db.WithContext(ctx).Model(&MessageRecord{}).Where("to_user_id = ?", userID)
    if unreadOnly { q = q.Where("read_at IS NULL") }
    var total int64
    if err := q.Count(&total).Error; err != nil { return nil, 0, err }
    var out []*MessageRecord
    if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&out).Error; err != nil { return nil, 0, err }
    return out, total, nil
}

func (r *Repo) UnreadCount(ctx context.Context, userID uint) (int64, error) {
    var c int64
    err := r.db.WithContext(ctx).Model(&MessageRecord{}).Where("to_user_id = ? AND read_at IS NULL", userID).Count(&c).Error
    return c, err
}

func (r *Repo) MarkRead(ctx context.Context, userID uint, ids []uint) error {
    if len(ids) == 0 { return nil }
    now := time.Now()
    return r.db.WithContext(ctx).Model(&MessageRecord{}).Where("to_user_id = ? AND id IN ? AND read_at IS NULL", userID, ids).Update("read_at", now).Error
}

// Re-export broadcast functionality with a child repo for convenience
func (r *Repo) Broadcast() *BroadcastRepo { return NewBroadcastRepo(r.db) }
