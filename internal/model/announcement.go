package model

import (
	"time"

	"gorm.io/gorm"
)

// Announcement 是后台公告：面向全体或指定角色的运营通知。
// 内容为 Markdown；popup=true 时对未确认用户在登录后弹窗展示，
// 直至显式 dismiss（announcement_reads 记录确认状态）。
type Announcement struct {
	gorm.Model
	Title     string `gorm:"size:255;not null"`
	ContentMd string `gorm:"type:text;not null"`
	// Audience: all | role（role 需与 RoleName 匹配）
	Audience string `gorm:"size:32;not null;default:all"`
	Role     string `gorm:"size:128"`
	Popup    bool   `gorm:"default:false"`
	Active   bool   `gorm:"default:true;index"`
	// 发布窗口（可选；零值=不限制）
	StartAt *time.Time
	EndAt   *time.Time
	// CreatedBy 为发布管理员用户名（审计展示用）。
	CreatedBy string `gorm:"size:128"`
}

func (Announcement) TableName() string { return "announcements" }

// AnnouncementRead 记录用户对公告的确认（dismiss 后不再弹窗）。
type AnnouncementRead struct {
	gorm.Model
	AnnouncementID uint      `gorm:"not null;uniqueIndex:uidx_announcement_read,priority:1"`
	Username       string    `gorm:"size:128;not null;uniqueIndex:uidx_announcement_read,priority:2"`
	ReadAt         time.Time `gorm:"not null"`
}

func (AnnouncementRead) TableName() string { return "announcement_reads" }
