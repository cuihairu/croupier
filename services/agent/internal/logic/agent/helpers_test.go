package agent

import (
	"testing"
	"time"
)

// TestDefaultString 测试默认字符串函数
func TestDefaultString(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback string
		expected string
	}{
		{
			name:     "非空值",
			value:    "hello",
			fallback: "default",
			expected: "hello",
		},
		{
			name:     "空字符串",
			value:    "",
			fallback: "default",
			expected: "default",
		},
		{
			name:     "纯空格",
			value:    "   ",
			fallback: "default",
			expected: "default",
		},
		{
			name:     "前后空格",
			value:    "  hello  ",
			fallback: "default",
			expected: "  hello  ", // 注意：函数只 TrimSpace 检查是否为空，不修剪
		},
		{
			name:     "制表符",
			value:    "\t",
			fallback: "default",
			expected: "default",
		},
		{
			name:     "换行符",
			value:    "\n",
			fallback: "default",
			expected: "default",
		},
		{
			name:     "零值",
			value:    "0",
			fallback: "default",
			expected: "0",
		},
		{
			name:     "fallback 为空",
			value:    "",
			fallback: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defaultString(tt.value, tt.fallback)
			if result != tt.expected {
				t.Errorf("defaultString(%q, %q) = %q, want %q",
					tt.value, tt.fallback, result, tt.expected)
			}
		})
	}
}

// TestFormatTime 测试时间格式化函数
func TestFormatTime(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "零时间",
			input:    time.Time{},
			expected: "",
		},
		{
			name:     "有效时间",
			input:    time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
			expected: "2024-01-15T12:30:45Z",
		},
		{
			name:     "Unix 纪元",
			input:    time.Unix(0, 0).UTC(),
			expected: "1970-01-01T00:00:00Z",
		},
		{
			name:     "当前时间",
			input:    time.Now().UTC(),
			expected: "", // 我们不验证具体值，只验证格式
		},
		{
			name:     "未来时间",
			input:    time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: "2099-12-31T23:59:59Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.input)

			if tt.expected == "" && tt.input.IsZero() {
				// 零时间应该返回空字符串
				if result != "" {
					t.Errorf("formatTime(zero time) returned %q, want empty string", result)
				}
				return
			}

			if tt.expected != "" && result != tt.expected {
				t.Errorf("formatTime(%v) = %q, want %q",
					tt.input, result, tt.expected)
			}

			// 对于非空结果，验证 RFC3339 格式
			if result != "" {
				_, err := time.Parse(time.RFC3339, result)
				if err != nil {
					t.Errorf("formatTime returned invalid RFC3339 format: %q, error: %v", result, err)
				}
			}
		})
	}
}

// TestFormatTime_Timezone 测试时区处理
func TestFormatTime_Timezone(t *testing.T) {
	// 本地时间
	localTime := time.Date(2024, 1, 15, 12, 30, 45, 0, time.Local)
	result := formatTime(localTime)

	// 应该被转换为 UTC
	if result == "" {
		t.Error("formatTime should not return empty for valid time")
	}

	// 验证是 UTC 格式（以 Z 结尾）
	if len(result) > 0 && result[len(result)-1] != 'Z' {
		t.Errorf("formatTime should return UTC time ending with 'Z', got %q", result)
	}
}

// TestDefaultString_Unicode 测试 Unicode 字符串
func TestDefaultString_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback string
		expected string
	}{
		{
			name:     "中文字符",
			value:    "你好",
			fallback: "默认",
			expected: "你好",
		},
		{
			name:     "Emoji",
			value:    "🎉",
			fallback: "😊",
			expected: "🎉",
		},
		{
			name:     "混合 Unicode",
			value:    "Hello 世界 🌍",
			fallback: "fallback",
			expected: "Hello 世界 🌍",
		},
		{
			name:     "空 Unicode",
			value:    "",
			fallback: "默认值",
			expected: "默认值",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defaultString(tt.value, tt.fallback)
			if result != tt.expected {
				t.Errorf("defaultString(%q, %q) = %q, want %q",
					tt.value, tt.fallback, result, tt.expected)
			}
		})
	}
}

// TestFormatTime_EdgeCases 测试时间边界情况
func TestFormatTime_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input time.Time
		check func(t *testing.T, result string)
	}{
		{
			name: "大时间值（9999年）",
			input: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC),
			check: func(t *testing.T, result string) {
				if result == "" {
					t.Error("formatTime(large time) should not return empty")
				}
			},
		},
		{
			name: "最小时间",
			input: time.Unix(-62135596801, 0), // 0001-01-01 00:00:00 UTC
			check: func(t *testing.T, result string) {
				if result == "" {
					t.Error("formatTime(min time) should not return empty")
				}
			},
		},
		{
			name: "毫秒精度",
			input: time.Date(2024, 1, 15, 12, 30, 45, 123456789, time.UTC),
			check: func(t *testing.T, result string) {
				// RFC3339 格式应该包含纳秒或毫秒
				if result == "" {
					t.Error("formatTime with nanoseconds should not return empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.input)
			tt.check(t, result)
		})
	}
}

// BenchmarkDefaultString 性能基准测试
func BenchmarkDefaultString(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		defaultString("test value", "fallback")
	}
}

// BenchmarkDefaultString_Empty 性能基准测试（空值）
func BenchmarkDefaultString_Empty(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		defaultString("", "fallback")
	}
}

// BenchmarkFormatTime 性能基准测试
func BenchmarkFormatTime(b *testing.B) {
	testTime := time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatTime(testTime)
	}
}

// BenchmarkFormatTime_Zero 性能基准测试（零时间）
func BenchmarkFormatTime_Zero(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatTime(time.Time{})
	}
}

// TestDefaultString_LongStrings 测试长字符串
func TestDefaultString_LongStrings(t *testing.T) {
	// 创建一个长字符串
	longString := make([]byte, 10000)
	for i := range longString {
		longString[i] = 'a'
	}

	result := defaultString(string(longString), "fallback")
	if result != string(longString) {
		t.Error("defaultString should handle long strings correctly")
	}

	// 测试长空格字符串
	longSpaces := make([]byte, 10000)
	for i := range longSpaces {
		longSpaces[i] = ' '
	}

	result = defaultString(string(longSpaces), "fallback")
	if result != "fallback" {
		t.Error("defaultString should treat long whitespace as empty")
	}
}

// TestFormatTime_Nanoseconds 测试纳秒精度
func TestFormatTime_Nanoseconds(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC)

	testCases := []struct {
		nanoseconds int
	}{
		{0},
		{1},
		{1000},        // 1 微秒
		{1000000},     // 1 毫秒
		{123456789},   // 123.456789 毫秒
		{999999999},   // 最大纳秒
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			testTime := baseTime.Add(time.Duration(tc.nanoseconds) * time.Nanosecond)
			result := formatTime(testTime)

			if result == "" {
				t.Error("formatTime should not return empty for time with nanoseconds")
			}

			// 验证可以解析回时间
			parsed, err := time.Parse(time.RFC3339, result)
			if err != nil {
				t.Errorf("Failed to parse formatted time: %v", err)
			}

			// 验证秒级精度一致（纳秒可能在序列化时丢失）
			if parsed.Unix() != testTime.Unix() {
				t.Errorf("Time mismatch: parsed %v, original %v", parsed, testTime)
			}
		})
	}
}
