package mq

import (
	"os"
	"testing"
)

// TestNewFromEnv_Default 测试默认环境变量（返回 noop）
func TestNewFromEnv_Default(t *testing.T) {
	// 清除环境变量
	os.Unsetenv("ANALYTICS_MQ_TYPE")

	q := NewFromEnv()
	if q == nil {
		t.Fatal("NewFromEnv() should return non-nil Queue")
	}

	// 默认应该是 Noop
	if _, ok := q.(*Noop); !ok {
		t.Error("Default should return Noop queue")
	}
}

// TestNewFromEnv_Noop 测试显式指定 noop 类型
func TestNewFromEnv_Noop(t *testing.T) {
	os.Setenv("ANALYTICS_MQ_TYPE", "noop")
	defer os.Unsetenv("ANALYTICS_MQ_TYPE")

	q := NewFromEnv()
	if q == nil {
		t.Fatal("NewFromEnv() should return non-nil Queue")
	}

	if _, ok := q.(*Noop); !ok {
		t.Error("Type 'noop' should return Noop queue")
	}
}

// TestNewFromEnv_UnsupportedType 测试不支持的类型（回退到 noop）
func TestNewFromEnv_UnsupportedType(t *testing.T) {
	os.Setenv("ANALYTICS_MQ_TYPE", "unsupported")
	defer os.Unsetenv("ANALYTICS_MQ_TYPE")

	q := NewFromEnv()
	if q == nil {
		t.Fatal("NewFromEnv() should return non-nil Queue")
	}

	// 不支持的类型应该回退到 Noop
	if _, ok := q.(*Noop); !ok {
		t.Error("Unsupported type should fallback to Noop")
	}
}

// TestNewFromEnv_EmptyType 测试空字符串类型
func TestNewFromEnv_EmptyType(t *testing.T) {
	os.Setenv("ANALYTICS_MQ_TYPE", "")
	defer os.Unsetenv("ANALYTICS_MQ_TYPE")

	q := NewFromEnv()
	if q == nil {
		t.Fatal("NewFromEnv() should return non-nil Queue")
	}

	if _, ok := q.(*Noop); !ok {
		t.Error("Empty type should return Noop queue")
	}
}

// TestNewFromEnv_Redis_NoBuildTag 测试 Redis 类型
func TestNewFromEnv_Redis_NoBuildTag(t *testing.T) {
	os.Setenv("ANALYTICS_MQ_TYPE", "redis")
	defer os.Unsetenv("ANALYTICS_MQ_TYPE")

	q := NewFromEnv()
	if q == nil {
		t.Fatal("NewFromEnv() should return non-nil Queue")
	}

	// Redis 实现存在（不在 Noop 分支）
	// 验证 Queue 接口方法可以调用
	err := q.PublishEvent(map[string]any{"test": "data"})
	// 可能成功或失败，取决于 Redis 连接，但不应该 panic
	_ = err
}

// TestNewFromEnv_Kafka_NoBuildTag 测试 Kafka 类型
func TestNewFromEnv_Kafka_NoBuildTag(t *testing.T) {
	os.Setenv("ANALYTICS_MQ_TYPE", "kafka")
	defer os.Unsetenv("ANALYTICS_MQ_TYPE")

	q := NewFromEnv()
	if q == nil {
		t.Fatal("NewFromEnv() should return non-nil Queue")
	}

	// Kafka 可能回退到 Noop（因为没有配置）
	// 验证 Queue 接口方法可以调用
	err := q.PublishEvent(map[string]any{"test": "data"})
	// 应该不 panic
	_ = err
}

// TestNewFromEnv_CaseInsensitive 测试类型是否大小写敏感
func TestNewFromEnv_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name       string
		envType    string
		expectNoop bool
	}{
		{"大写 REDIS 不匹配", "REDIS", true}, // 大写不匹配小写比较，回退到 Noop
		{"大写 KAFKA 不匹配", "KAFKA", true}, // 大写不匹配小写比较，回退到 Noop
		{"大写 NOOP 不匹配", "NOOP", true},   // 大写不匹配小写比较，回退到 Noop
		{"小写 noop 匹配", "noop", true},    // 匹配，返回 Noop
		{"混合 Redis 不匹配", "Redis", true}, // 不匹配小写比较
		{"混合 Kafka 不匹配", "Kafka", true}, // 不匹配小写比较
		{"混合 Noop 不匹配", "Noop", true},   // 不匹配小写比较
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("ANALYTICS_MQ_TYPE", tt.envType)
			defer os.Unsetenv("ANALYTICS_MQ_TYPE")

			q := NewFromEnv()
			if q == nil {
				t.Fatal("NewFromEnv() should return non-nil Queue")
			}

			if tt.expectNoop {
				if _, ok := q.(*Noop); !ok {
					t.Error("Should return Noop for non-matching type case")
				}
			}
			// 验证 Queue 接口可以调用
			err := q.PublishEvent(map[string]any{"test": "data"})
			_ = err // 不应该 panic
		})
	}
}
