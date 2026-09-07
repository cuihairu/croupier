package mq

import (
	"testing"
)

// 以下测试文档化 factory.go / kafka_pub.go 中不可达的防御分支：
//
//  1. factory.go:26/29：newRedisFromEnv 唯一返回语句为
//     `return NewRedis(...), nil`，error 恒为 nil；NewRedis 解析失败时降级为
//     NewNoop()（非 nil），成功时返回 &redisQueue{}。因此 (err != nil) 与
//     (q == nil) 两个分支均不可触发。
//  2. factory.go:35/38：newKafkaFromEnv 唯一返回语句为 `return q, nil`，
//     NewKafka 在 brokers 为空时返回 NewNoop()（非 nil），否则返回
//     &kafkaQueue{}（非 nil），两个分支同样不可触发。
//  3. kafka_pub.go:50/55：kafka-go v0.4.50 的 Writer.Close 无条件
//     `return nil`，Close 错误分支不可触发。
func TestNewRedisFromEnv_NeverReturnsErrorOrNil(t *testing.T) {
	t.Setenv("REDIS_URL", "://not-a-redis-url")
	q, err := newRedisFromEnv()
	if err != nil {
		t.Fatalf("newRedisFromEnv must never return an error, got %v", err)
	}
	if q == nil {
		t.Fatal("newRedisFromEnv must never return a nil queue with nil error")
	}
	if _, ok := q.(*Noop); !ok {
		t.Fatalf("invalid REDIS_URL should degrade to Noop inside NewRedis, got %T", q)
	}
}

func TestNewKafkaFromEnv_NeverReturnsErrorOrNil(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "   ")
	q, err := newKafkaFromEnv()
	if err != nil {
		t.Fatalf("newKafkaFromEnv must never return an error, got %v", err)
	}
	if q == nil {
		t.Fatal("newKafkaFromEnv must never return a nil queue with nil error")
	}
	if _, ok := q.(*kafkaQueue); !ok {
		t.Fatalf("blank brokers fall back to default brokers, want *kafkaQueue, got %T", q)
	}
	if err := q.Close(); err != nil {
		t.Fatalf("cleanup Close: %v", err)
	}
}

func TestKafkaQueue_CloseAlwaysNil(t *testing.T) {
	q := NewKafka([]string{"127.0.0.1:19092"}, "t.events", "t.payments")
	if err := q.Close(); err != nil {
		t.Fatalf("kafka Writer.Close in kafka-go v0.4.50 always returns nil, got %v", err)
	}
	// 二次 Close 仍然为 nil：Writer.Close 的 closeOnce 语义。
	if err := q.Close(); err != nil {
		t.Fatalf("second Close should also be nil, got %v", err)
	}
}
