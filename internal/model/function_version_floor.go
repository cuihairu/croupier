package model

import "time"

// FunctionVersionFloor 是平台侧按 (game_id, env, function_id) 配置的最低
// 可注册 SDK 版本（函数级版本门槛）。provider 自报 sdk_version 低于配置
// 值时，该函数不随本次注册物化（只产生 function_version_below_minimum
// 注册警告），交叉提供保护与高水位门槛一致。
//
// 优先级：函数级配置 > registry.sdkVersionMinimums（语言级 yaml）>
// 滑动高水位（自动观测）。与注册物化解耦存独立表：函数行随重注册
// upsert，平台设置不能随描述符回写被冲掉。
//
// 硬删模型（无 DeletedAt）：清空门槛即物理删除行，软删残留会占住物理
// 唯一索引导致同 key 重建失败（0024 同族教训）。
type FunctionVersionFloor struct {
	ID         uint64 `gorm:"primaryKey" json:"id"`
	GameID     string `gorm:"size:64;uniqueIndex:idx_fn_floor_game_env_fn" json:"gameId"`
	Env        string `gorm:"size:64;uniqueIndex:idx_fn_floor_game_env_fn" json:"env"`
	FunctionID string `gorm:"size:128;uniqueIndex:idx_fn_floor_game_env_fn" json:"functionId"`
	// MinVersion 是最低可注册版本（semver 数字前缀）。设置时服务端校验
	// 可解析（sdkversion.Parseable），不可解析值拒绝入库。
	MinVersion string `gorm:"size:32" json:"minVersion"`
	// UpdatedBy 是最近一次设置/清空的管理员账号（审计用）。
	UpdatedBy string    `gorm:"size:64" json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName overrides the default gorm table name.
func (FunctionVersionFloor) TableName() string { return "function_version_floors" }
