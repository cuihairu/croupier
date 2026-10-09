package common

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestMergeLogSection(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*viper.Viper)
		expected map[string]interface{}
	}{
		{
			name: "no log section",
			setup: func(v *viper.Viper) {
				v.Set("server.port", 8080)
			},
			expected: map[string]interface{}{},
		},
		{
			name: "with log section",
			setup: func(v *viper.Viper) {
				v.Set("log.level", "debug")
				v.Set("log.format", "json")
			},
			expected: map[string]interface{}{
				"log.level":  "debug",
				"log.format": "json",
			},
		},
		{
			name: "partial log section",
			setup: func(v *viper.Viper) {
				v.Set("log.level", "warn")
			},
			expected: map[string]interface{}{
				"log.level": "warn",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			tt.setup(v)

			MergeLogSection(v)

			for key, expectedValue := range tt.expected {
				actualValue := v.Get(key)
				assert.Equal(t, expectedValue, actualValue, "key: %s", key)
			}
		})
	}
}

// 日志装配本体测试随上收迁至 core/logx；此处只验证别名与转发仍可用。
func TestLogxShim(t *testing.T) {
	assert.NotNil(t, LogConfig{})
	assert.NotNil(t, GetLogCounters())
	SetupLoggerWithFile("info", "json", "", 0, 0, 0, false)
}
