package usersgorm

import (
    "gorm.io/gorm"
    "time"
)

// GORM models (IDs as uint via gorm.Model)

type UserRecord struct {
    gorm.Model
    Username     string `gorm:"uniqueIndex;size:64;not null"`
    DisplayName  string `gorm:"size:128"`
    Email        string `gorm:"size:256"`
    Phone        string `gorm:"size:32"`
    PasswordHash string `gorm:"size:255"` // bcrypt hash
    Active       bool   `gorm:"default:true"`
}

type RoleRecord struct {
    gorm.Model
    Name        string `gorm:"uniqueIndex;size:64;not null"`
    Description string `gorm:"size:256"`
}

type UserRoleRecord struct {
    gorm.Model
    UserID uint `gorm:"index;not null"`
    RoleID uint `gorm:"index;not null"`
}

type RolePermRecord struct {
    gorm.Model
    RoleID uint   `gorm:"index;not null"`
    Perm   string `gorm:"index;size:128;not null"`
}

func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(&UserRecord{}, &RoleRecord{}, &UserRoleRecord{}, &RolePermRecord{})
}

// Helpers to stamp time manually if needed
func now() time.Time { return time.Now().UTC() }

