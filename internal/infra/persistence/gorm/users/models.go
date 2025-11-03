package usersgorm

import (
    "gorm.io/gorm"
    "time"
)

// GORM models (IDs as uint via gorm.Model)

type UserAccount struct {
    gorm.Model
    Username     string `gorm:"uniqueIndex;size:64;not null"`
    DisplayName  string `gorm:"size:128"`
    Email        string `gorm:"size:256"`
    Phone        string `gorm:"size:32"`
    PasswordHash string `gorm:"size:255"` // bcrypt hash
    Active       bool   `gorm:"default:true"`
    OTPSecret    string `gorm:"size:64"`
}

// TableName returns the table name for UserAccount model
func (UserAccount) TableName() string {
    return "user_records"
}

type RoleRecord struct {
    gorm.Model
    Name        string `gorm:"uniqueIndex;size:64;not null"`
    Description string `gorm:"size:256"`
}

// TableName returns the table name for RoleRecord model
func (RoleRecord) TableName() string {
    return "role_records"
}

type UserRoleRecord struct {
    gorm.Model
    UserID uint `gorm:"index;not null"`
    RoleID uint `gorm:"index;not null"`
}

// TableName returns the table name for UserRoleRecord model
func (UserRoleRecord) TableName() string {
    return "user_role_records"
}

type RolePermRecord struct {
    gorm.Model
    RoleID uint   `gorm:"index;not null"`
    Perm   string `gorm:"index;size:128;not null"`
}

// TableName returns the table name for RolePermRecord model
func (RolePermRecord) TableName() string {
    return "role_perm_records"
}

func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(&UserAccount{}, &RoleRecord{}, &UserRoleRecord{}, &RolePermRecord{})
}

// Helpers to stamp time manually if needed
func now() time.Time { return time.Now().UTC() }
