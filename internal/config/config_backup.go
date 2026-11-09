package config

import (
	"time"
)

// BackupConfig 数据面备份（OPS-008）域配置。
type BackupConfig struct {
	// Schedule 定期备份调度。enabled 缺省 false=不装配（仅显式 true 才
	// 启动后台循环，枚举注册启用的数据源触发备份）。
	Schedule BackupScheduleConfig `json:"schedule,omitempty" yaml:"schedule,omitempty"`
}

// BackupScheduleConfig 定期备份调度开关与节奏。
type BackupScheduleConfig struct {
	// Enabled 缺省 false：不启动定期备份后台循环（必须显式 true）。
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Interval 调度周期（如 "6h"）；空/非法/非正 → 缺省 6h。
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`
}

// DefaultBackupScheduleInterval 是定期备份调度的缺省周期。
const DefaultBackupScheduleInterval = 6 * time.Hour

// EffectiveInterval 返回调度周期，空/非法/非正用缺省 6h。
func (c BackupScheduleConfig) EffectiveInterval() time.Duration {
	if c.Interval == "" {
		return DefaultBackupScheduleInterval
	}
	d, err := time.ParseDuration(c.Interval)
	if err != nil || d <= 0 {
		return DefaultBackupScheduleInterval
	}
	return d
}
