package model

import (
	"gorm.io/gorm"
)

// Player 玩家结构体 (对应 player.api)
type Player struct {
	gorm.Model
	Username string `gorm:"size:64;not null;index:idx_player_username_game,priority:1"`
	Nickname string `gorm:"size:128;index"`
	Email    string `gorm:"size:256;index"`
	Phone    string `gorm:"size:32;index"`
	GameID   string `gorm:"size:64;index:idx_player_game_status,priority:1;index:idx_player_username_game,priority:2;not null"`
	Status   int    `gorm:"default:1;index:idx_player_game_status,priority:2"` // 1:active 0:banned 2:suspended
	Balance  int64  `gorm:"default:0;index"`                                    // 游戏货币
	Level    int    `gorm:"default:1;index"`
	VIP      int    `gorm:"default:0;index"`
	Password string `gorm:"size:255"` // 密码哈希
}

// TableName 实现 GORM 的表名接口
func (Player) TableName() string {
	return "players"
}

// IsActive 检查玩家是否为活跃状态
func (p *Player) IsActive() bool {
	return p.Status == 1
}

// IsBanned 检查玩家是否被封禁
func (p *Player) IsBanned() bool {
	return p.Status == 0
}

// IsSuspended 检查玩家是否被暂停
func (p *Player) IsSuspended() bool {
	return p.Status == 2
}

// Ban 封禁玩家
func (p *Player) Ban() {
	p.Status = 0
}

// Suspend 暂停玩家
func (p *Player) Suspend() {
	p.Status = 2
}

// Activate 激活玩家
func (p *Player) Activate() {
	p.Status = 1
}
