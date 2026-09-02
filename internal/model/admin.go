package model

import (
	"time"

	"gorm.io/gorm"
)

// Admin represents administrator accounts.
type Admin struct {
	gorm.Model
	Username     string `gorm:"uniqueIndex;size:64;not null"`
	Nickname     string `gorm:"size:128;index"`
	Email        string `gorm:"size:256;index"`
	Phone        string `gorm:"size:32;index"`
	Avatar       string `gorm:"size:512"`
	PasswordHash string `gorm:"size:255"`
	Status       int    `gorm:"default:1;index"` // 1:active 0:disabled
	OTPSecret    string `gorm:"size:64"`
	// OTPEnabled 标记该本地账号是否已确认启用 TOTP 二次验证；
	// 仅 local provider 登录时校验，LDAP/OIDC 登录跳过（MFA 属 IdP 职责）。
	OTPEnabled bool `gorm:"not null;default:false"`
	// FailedAttempts 记录连续密码失败次数（仅本地 provider 计数），
	// 成功登录清零；达到阈值后由 LockedUntil 承载锁定截止时间。
	FailedAttempts int        `gorm:"not null;default:0"`
	LockedUntil    *time.Time `gorm:"index"`
	// TokenVersion 单调递增，签发 JWT 时写入 claims；改密码/禁用/登出
	// 时 +1，中间件比对不一致即拒绝，实现 token 即时撤销。
	TokenVersion int        `gorm:"not null;default:0"`
	LastLoginAt  *time.Time `gorm:"index"`
	LastGameID   string     `gorm:"size:64;index"` // 上次选择的游戏 ID（业务标识）
	LastEnv      string     `gorm:"size:64"`       // 上次选择的环境
	CreatedBy    uint       `gorm:"index"`
	UpdatedBy    uint
}

// TableName implements gorm's tabler interface.
func (Admin) TableName() string {
	return "admins"
}
