package messagesgorm

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Broadcast messages target all users or users having any of the target roles.
type BroadcastMessageRecord struct {
	gorm.Model
	Title    string `gorm:"size:200"`
	Content  string `gorm:"type:text"`
	Type     string `gorm:"size:32"` // info|warning|task
	Audience string `gorm:"size:16"` // all|roles

	// Associations
	BroadcastRoleRecords []BroadcastRoleRecord `gorm:"foreignKey:BroadcastID"`
	BroadcastAckRecords  []BroadcastAckRecord  `gorm:"foreignKey:BroadcastID"`
}

// Target roles (normalized by name)
type BroadcastRoleRecord struct {
	gorm.Model
	BroadcastID uint   `gorm:"index;not null"`
	RoleName    string `gorm:"index;size:64;not null"`
}

// Read acknowledgements per user
type BroadcastAckRecord struct {
	gorm.Model
	BroadcastID uint      `gorm:"index;not null"`
	UserID      uint      `gorm:"index;not null"`
	ReadAt      time.Time `gorm:"not null"`
}

func AutoMigrateBroadcast(db *gorm.DB) error {
	return db.AutoMigrate(&BroadcastMessageRecord{}, &BroadcastRoleRecord{}, &BroadcastAckRecord{})
}

type BroadItem struct {
	ID        uint
	Title     string
	Content   string
	Type      string
	CreatedAt time.Time
	Read      bool
}

type BroadcastRepo struct{ db *gorm.DB }

func NewBroadcastRepo(db *gorm.DB) *BroadcastRepo { return &BroadcastRepo{db: db} }

func (r *BroadcastRepo) Create(ctx context.Context, msg *BroadcastMessageRecord, roleNames []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		if msg.Audience == "roles" {
			for _, rn := range roleNames {
				br := &BroadcastRoleRecord{BroadcastID: msg.ID, RoleName: rn}
				if err := tx.Create(br).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// List returns applicable broadcasts for the user's roles; if unreadOnly is true, only unread ones.
func (r *BroadcastRepo) List(ctx context.Context, userID uint, roleNames []string, unreadOnly bool, limit, offset int) ([]BroadItem, int64, error) {
	if len(roleNames) == 0 {
		roleNames = []string{""}
	}

	// Use GORM's Raw method with proper SQL to avoid alias issues
	baseSQL := `SELECT bm.id, bm.title, bm.content, bm.type, bm.created_at,
                CASE WHEN ba.id IS NULL THEN 0 ELSE 1 END AS read
                FROM broadcast_message_records AS bm
                LEFT JOIN broadcast_ack_records ba ON ba.broadcast_id = bm.id AND ba.user_id = ?
                LEFT JOIN broadcast_role_records br ON br.broadcast_id = bm.id
                WHERE (bm.audience = 'all' OR br.role_name IN ?)
                AND bm.deleted_at IS NULL`

	var sqlParams []interface{}
	sqlParams = append(sqlParams, userID, roleNames)

	if unreadOnly {
		baseSQL += " AND ba.id IS NULL"
	}

	// Count total
	countSQL := `SELECT COUNT(DISTINCT bm.id)
                FROM broadcast_message_records AS bm
                LEFT JOIN broadcast_ack_records ba ON ba.broadcast_id = bm.id AND ba.user_id = ?
                LEFT JOIN broadcast_role_records br ON br.broadcast_id = bm.id
                WHERE (bm.audience = 'all' OR br.role_name IN ?)
                AND bm.deleted_at IS NULL`

	countParams := []interface{}{userID, roleNames}
	if unreadOnly {
		countSQL += " AND ba.id IS NULL"
	}

	var total int64
	if err := r.db.WithContext(ctx).Raw(countSQL, countParams...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get results
	finalSQL := baseSQL + " GROUP BY bm.id, bm.title, bm.content, bm.type, bm.created_at, ba.id ORDER BY bm.created_at DESC LIMIT ? OFFSET ?"
	sqlParams = append(sqlParams, limit, offset)

	var rows []BroadItem
	if err := r.db.WithContext(ctx).Raw(finalSQL, sqlParams...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

func (r *BroadcastRepo) UnreadCount(ctx context.Context, userID uint, roleNames []string) (int64, error) {
	if len(roleNames) == 0 {
		roleNames = []string{""}
	}
	q := r.db.WithContext(ctx).Table("broadcast_message_records AS bm").
		Joins("LEFT JOIN broadcast_ack_records ba ON ba.broadcast_id = bm.id AND ba.user_id = ?", userID).
		Joins("LEFT JOIN broadcast_role_records br ON br.broadcast_id = bm.id").
		Where("(bm.audience = 'all' OR br.role_name IN ?) AND ba.id IS NULL", roleNames)
	var c int64
	if err := q.Distinct("bm.id").Count(&c).Error; err != nil {
		return 0, err
	}
	return c, nil
}

func (r *BroadcastRepo) MarkRead(ctx context.Context, userID uint, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	// upsert Acks
	now := time.Now()
	rows := make([]BroadcastAckRecord, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, BroadcastAckRecord{BroadcastID: id, UserID: userID, ReadAt: now})
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}
