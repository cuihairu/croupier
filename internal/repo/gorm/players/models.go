//go:build legacy_repo
// +build legacy_repo

package players

import (
	"gorm.io/gorm"
	"time"
)

// PlayerRecord represents a game player
type PlayerRecord struct {
	gorm.Model
	Username string `gorm:"size:64;not null"`
	Nickname string `gorm:"size:128"`
	Email    string `gorm:"size:256"`
	Phone    string `gorm:"size:32"`
	GameID   string `gorm:"size:64;index;not null"`
	Status   int    `gorm:"default:1"` // 1:active 0:banned 2:suspended
	Balance  int64  `gorm:"default:0"` // 游戏货币
	Level    int    `gorm:"default:1"`
	VIP      int    `gorm:"default:0"`
	Password string `gorm:"size:255"` // 可选，某些游戏需要
}

// TableName returns the table name for PlayerRecord model
func (PlayerRecord) TableName() string {
	return "player_records"
}

// PlayerLoginRecord tracks player login activity
type PlayerLoginRecord struct {
	gorm.Model
	PlayerID  uint      `gorm:"index;not null"`
	LoginIP   string    `gorm:"size:45"` // IPv6 support
	LoginTime time.Time `gorm:"index"`
	UserAgent string    `gorm:"size:512"`
	DeviceID  string    `gorm:"size:128"`
}

// TableName returns the table name for PlayerLoginRecord model
func (PlayerLoginRecord) TableName() string {
	return "player_login_records"
}

// PlayerBalanceRecord tracks balance changes
type PlayerBalanceRecord struct {
	gorm.Model
	PlayerID     uint      `gorm:"index;not null"`
	OldBalance   int64     `gorm:"not null"`
	NewBalance   int64     `gorm:"not null"`
	ChangeAmount int64     `gorm:"not null"`
	Reason       string    `gorm:"size:256"`
	Operation    string    `gorm:"size:64"` // add, subtract, set
	Operator     string    `gorm:"size:64"` // system, admin, game
	OperatorID   uint      // 操作者ID
	CreatedAt    time.Time `gorm:"index"`
}

// TableName returns the table name for PlayerBalanceRecord model
func (PlayerBalanceRecord) TableName() string {
	return "player_balance_records"
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&PlayerRecord{},
		&PlayerLoginRecord{},
		&PlayerBalanceRecord{},
	)
}

// TableName returns table name for PlayerRecord migration
func (PlayerRecord) TableName() string {
	return "player_records_migration"
}

// TableName returns table name for PlayerLoginRecord migration
func (PlayerLoginRecord) TableName() string {
	return "player_login_records_migration"
}

// TableName returns table name for PlayerBalanceRecord migration
func (PlayerBalanceRecord) TableName() string {
	return "player_balance_records_migration"
}
