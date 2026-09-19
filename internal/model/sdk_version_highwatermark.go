package model

import "time"

// SDKVersionHighwatermark records the highest SDK version ever observed per
// (game_id, env, sdk_language) — the sliding version floor evaluated at
// function registration time. 注册链见到更高版本的 SDK 后抬升高水位；
// 后续注册低于高水位 2 个 minor 或 1 个 major 的 SDK 会被拒绝函数注册
// （internal/platform/registry/sdkversion）。
//
// 硬删模型（无 DeletedAt）：高水位是单调观测值不是业务实体，软删残留行
// 会占住物理唯一索引导致同 key 重建失败（0024 同族教训）。
type SDKVersionHighwatermark struct {
	ID          uint64 `gorm:"primaryKey"`
	GameID      string `gorm:"size:64;uniqueIndex:idx_sdk_hwm_game_env_lang"`
	Env         string `gorm:"size:64;uniqueIndex:idx_sdk_hwm_game_env_lang"`
	SdkLanguage string `gorm:"size:32;uniqueIndex:idx_sdk_hwm_game_env_lang"`
	// SdkVersion 是见过的最高可解析版本（semver 数字前缀）。不可解析的
	// 自报版本（"unknown"）不参与抬升。
	SdkVersion string `gorm:"size:32"`
	UpdatedAt  time.Time
}

// TableName overrides the default gorm table name.
func (SDKVersionHighwatermark) TableName() string { return "sdk_version_highwatermarks" }
