package common

import (
	"github.com/spf13/viper"

	"github.com/cuihairu/croupier/core/logx"
)

// 日志装配已上收 core/logx（agent-core K1 上收，行为零变化）：本文件只保留
// 类型别名与转发，既有配置结构（LogConfig 字段即 logx.LogConfig）与调用点
// 不动；新代码请直接使用 core/logx。

// LogConfig 日志配置（core/logx 别名，序列化标签随源类型）。
type LogConfig = logx.LogConfig

// SetupLoggerWithFile 转发 core/logx 装配（std log + slog 默认 logger）。
func SetupLoggerWithFile(level, format, filePath string, maxSizeMB, maxBackups, maxAgeDays int, compress bool) {
	logx.SetupLoggerWithFile(level, format, filePath, maxSizeMB, maxBackups, maxAgeDays, compress)
}

// GetLogCounters 转发 core/logx 级别计数。
func GetLogCounters() map[string]int64 { return logx.GetLogCounters() }

// MergeLogSection flattens a nested "log" section into top-level log.* keys.
func MergeLogSection(v *viper.Viper) {
	if sub := v.Sub("log"); sub != nil {
		for _, k := range []string{"level", "format", "file", "max_size", "max_backups", "max_age", "compress", "output"} {
			if sub.IsSet(k) {
				v.Set("log."+k, sub.Get(k))
			}
		}
	}
}
