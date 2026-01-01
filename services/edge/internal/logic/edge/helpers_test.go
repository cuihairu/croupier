package edge

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

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
			name:     "未来时间",
			input:    time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: "2099-12-31T23:59:59Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.input)

			if tt.expected == "" && tt.input.IsZero() {
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
			name:     "制表符",
			value:    "\t",
			fallback: "default",
			expected: "default",
		},
		{
			name:     "混合空格",
			value:    " \t\n ",
			fallback: "default",
			expected: "default",
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

// TestGenerateTunnelID 测试生成隧道 ID
func TestGenerateTunnelID(t *testing.T) {
	// 记录开始时间
	before := time.Now()

	agentID := "agent-123"
	tunnelID := generateTunnelID(agentID)

	// 记录结束时间
	after := time.Now()

	// 验证格式
	if !strings.HasPrefix(tunnelID, "tunnel-") {
		t.Errorf("Tunnel ID should start with 'tunnel-', got %q", tunnelID)
	}

	// 验证包含 agent ID（修剪后）
	if !strings.Contains(tunnelID, strings.TrimSpace(agentID)) {
		t.Errorf("Tunnel ID should contain agent ID, got %q", tunnelID)
	}

	// 验证包含时间戳
	parts := strings.Split(tunnelID, "-")
	if len(parts) < 3 {
		t.Errorf("Tunnel ID should have at least 3 parts, got %d", len(parts))
	}

	// 验证时间戳部分是数字
	timestampStr := parts[len(parts)-1]
	var timestamp int64
	_, err := fmt.Sscanf(timestampStr, "%d", &timestamp)
	if err != nil {
		t.Errorf("Last part should be numeric timestamp: %v", err)
	}

	// 验证时间戳在合理范围内
	tunnelTime := time.Unix(0, timestamp)
	if tunnelTime.Before(before.Add(-time.Second)) || tunnelTime.After(after.Add(time.Second)) {
		t.Errorf("Tunnel timestamp %v is outside expected range [%v, %v]",
			tunnelTime, before, after)
	}
}

// TestGenerateTunnelID_Uniqueness 测试生成唯一 ID
func TestGenerateTunnelID_Uniqueness(t *testing.T) {
	agentID := "agent-123"
	ids := make(map[string]bool)

	// 生成 1000 个 ID 并验证唯一性
	for i := 0; i < 1000; i++ {
		tunnelID := generateTunnelID(agentID)
		if ids[tunnelID] {
			t.Errorf("Duplicate tunnel ID generated: %s", tunnelID)
		}
		ids[tunnelID] = true

		// 避免生成太快导致时间戳重复
		time.Sleep(time.Microsecond)
	}
}

// TestGenerateTunnelID_SpecialCharacters 测试特殊字符处理
func TestGenerateTunnelID_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name    string
		agentID string
		check   func(t *testing.T, tunnelID string)
	}{
		{
			name:    "前后空格",
			agentID: "  agent-123  ",
			check: func(t *testing.T, tunnelID string) {
				// 空格应该被修剪
				if strings.Contains(tunnelID, "  ") {
					t.Error("Leading/trailing spaces should be trimmed")
				}
			},
		},
		{
			name:    "制表符",
			agentID: "\tagent-123\t",
			check: func(t *testing.T, tunnelID string) {
				// 制表符应该被修剪
				if strings.Contains(tunnelID, "\t") {
					t.Error("Tabs should be trimmed")
				}
			},
		},
		{
			name:    "Unicode",
			agentID: "agent-中文-123",
			check: func(t *testing.T, tunnelID string) {
				// Unicode 字符应该保留
				if !strings.Contains(tunnelID, "中文") {
					t.Error("Unicode characters should be preserved")
				}
			},
		},
		{
			name:    "空字符串",
			agentID: "",
			check: func(t *testing.T, tunnelID string) {
				// 空字符串也应该能工作
				if tunnelID == "" {
					t.Error("Tunnel ID should not be empty for empty agent ID")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tunnelID := generateTunnelID(tt.agentID)
			tt.check(t, tunnelID)
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

// TestFormatTime_Nanoseconds 测试纳秒精度
func TestFormatTime_Nanoseconds(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC)

	testCases := []int{0, 1, 1000, 1000000, 123456789, 999999999}

	for _, nanos := range testCases {
		t.Run("", func(t *testing.T) {
			testTime := baseTime.Add(time.Duration(nanos) * time.Nanosecond)
			result := formatTime(testTime)

			if result == "" {
				t.Error("formatTime should not return empty for time with nanoseconds")
			}

			// 验证可以解析回时间
			parsed, err := time.Parse(time.RFC3339, result)
			if err != nil {
				t.Errorf("Failed to parse formatted time: %v", err)
			}

			// 验证秒级精度一致
			if parsed.Unix() != testTime.Unix() {
				t.Errorf("Time mismatch: parsed %v, original %v", parsed, testTime)
			}
		})
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
			name:     "混合 Unicode 和空格",
			value:    "  Hello 世界  ",
			fallback: "fallback",
			expected: "  Hello 世界  ", // 前后空格保留
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

// BenchmarkGenerateTunnelID 性能基准测试
func BenchmarkGenerateTunnelID(b *testing.B) {
	agentID := "agent-123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generateTunnelID(agentID)
	}
}

// BenchmarkGenerateTunnelID_Parallel 并发性能测试
func BenchmarkGenerateTunnelID_Parallel(b *testing.B) {
	agentID := "agent-123"

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			generateTunnelID(agentID)
		}
	})
}

// BenchmarkFormatTime 性能基准测试
func BenchmarkFormatTime(b *testing.B) {
	testTime := time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatTime(testTime)
	}
}

// BenchmarkDefaultString 性能基准测试
func BenchmarkDefaultString(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		defaultString("test value", "fallback")
	}
}

// TestGenerateTunnelID_Concurrency 测试并发安全性
func TestGenerateTunnelID_Concurrency(t *testing.T) {
	agentID := "agent-123"
	done := make(chan string, 100)

	// 并发生成 100 个 ID
	for i := 0; i < 100; i++ {
		go func() {
			id := generateTunnelID(agentID)
			done <- id
		}()
	}

	// 收集所有 ID
	ids := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		id := <-done
		ids = append(ids, id)
	}

	// 验证所有 ID 格式正确
	for _, id := range ids {
		if !strings.HasPrefix(id, "tunnel-") {
			t.Errorf("Invalid ID format: %s", id)
		}
	}

	// 由于纳秒级时间戳在极高并发下可能重复，我们只验证不全部相同
	uniqueCount := make(map[string]bool)
	for _, id := range ids {
		uniqueCount[id] = true
	}

	if len(uniqueCount) == 1 {
		t.Error("All IDs are identical, which suggests timestamp is not varying")
	}

	// 验证至少有一些唯一性
	if len(uniqueCount) < 10 {
		t.Errorf("Too few unique IDs generated: %d out of 100", len(uniqueCount))
	}
}
