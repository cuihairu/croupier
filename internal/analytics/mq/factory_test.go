package mq

import (
	"os"
	"strings"
	"testing"
)

// withMQType sets (or unsets with "") ANALYTICS_MQ_TYPE for the test and
// restores the previous value afterwards.
func withMQType(t *testing.T, value string) {
	t.Helper()
	prev, had := os.LookupEnv("ANALYTICS_MQ_TYPE")
	if value == "" {
		os.Unsetenv("ANALYTICS_MQ_TYPE")
	} else {
		os.Setenv("ANALYTICS_MQ_TYPE", value)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv("ANALYTICS_MQ_TYPE", prev)
		} else {
			os.Unsetenv("ANALYTICS_MQ_TYPE")
		}
	})
}

// TestNewFromEnv_DefaultRedis asserts the fail-fast default: with no
// ANALYTICS_MQ_TYPE the factory attempts redis. Against an unreachable
// address it must return an error instead of silently degrading to noop.
func TestNewFromEnv_DefaultRedis(t *testing.T) {
	withMQType(t, "")
	os.Setenv("REDIS_URL", "redis://127.0.0.1:1/0") // nothing listens here
	t.Cleanup(func() { os.Unsetenv("REDIS_URL") })

	q, err := NewFromEnv()
	if err == nil {
		// A reachable local Redis would make construction succeed; then a
		// non-nil redis publisher is the correct outcome.
		if q == nil {
			t.Fatal("NewFromEnv() returned nil queue and nil error")
		}
		if _, ok := q.(*Noop); ok {
			t.Fatal("default must not silently degrade to noop")
		}
		return
	}
	if q != nil {
		t.Fatal("error path must return nil queue")
	}
	if !strings.Contains(err.Error(), "redis") {
		t.Errorf("error should mention redis, got: %v", err)
	}
}

// TestNewFromEnv_ExplicitNoop keeps the local-debug escape hatch.
func TestNewFromEnv_ExplicitNoop(t *testing.T) {
	withMQType(t, "noop")

	q, err := NewFromEnv()
	if err != nil {
		t.Fatalf("noop must never fail: %v", err)
	}
	if _, ok := q.(*Noop); !ok {
		t.Fatal("explicit noop should return Noop queue")
	}
	if err := q.PublishEvent(map[string]any{"k": "v"}); err != nil {
		t.Errorf("noop PublishEvent should be nil, got %v", err)
	}
}

// TestNewFromEnv_UnsupportedTypeFails asserts fail-fast on typos instead
// of the old silent noop fallback.
func TestNewFromEnv_UnsupportedTypeFails(t *testing.T) {
	withMQType(t, "unsupported")

	q, err := NewFromEnv()
	if err == nil {
		t.Fatal("unsupported type must return an error")
	}
	if q != nil {
		t.Fatal("error path must return nil queue")
	}
	if !strings.Contains(err.Error(), "unsupported") || !strings.Contains(err.Error(), "redis|kafka|noop") {
		t.Errorf("error should list supported types, got: %v", err)
	}
}

// TestNewFromEnv_CaseSensitive rejects mixed-case values (which previously
// fell back to noop silently).
func TestNewFromEnv_CaseSensitive(t *testing.T) {
	for _, value := range []string{"REDIS", "Kafka", "Noop", "Redis"} {
		withMQType(t, value)
		q, err := NewFromEnv()
		if err == nil {
			t.Fatalf("type %q must be rejected (case-sensitive)", value)
		}
		if q != nil {
			t.Fatalf("type %q error path must return nil queue", value)
		}
	}
}

// TestNewFromEnv_RedisExplicit mirrors the default path for the explicit
// value, using an unreachable address to force the error branch.
func TestNewFromEnv_RedisExplicit(t *testing.T) {
	withMQType(t, "redis")
	os.Setenv("REDIS_URL", "redis://127.0.0.1:1/0")
	t.Cleanup(func() { os.Unsetenv("REDIS_URL") })

	q, err := NewFromEnv()
	if err == nil {
		if q == nil {
			t.Fatal("nil queue with nil error")
		}
		return // local redis reachable
	}
	if q != nil || !strings.Contains(err.Error(), "redis") {
		t.Errorf("unexpected result: q=%v err=%v", q, err)
	}
}

// TestNewFromEnv_KafkaUnreachableFails asserts kafka also fails fast on
// unreachable brokers.
func TestNewFromEnv_KafkaUnreachableFails(t *testing.T) {
	withMQType(t, "kafka")
	// kafka-go dials lazily, so construction may succeed; when it does, the
	// queue must not be nil and must not be a silent noop.
	q, err := NewFromEnv()
	if err != nil {
		if q != nil {
			t.Fatal("error path must return nil queue")
		}
		if !strings.Contains(err.Error(), "kafka") {
			t.Errorf("error should mention kafka, got: %v", err)
		}
		return
	}
	if q == nil {
		t.Fatal("nil queue with nil error")
	}
	if _, ok := q.(*Noop); ok {
		t.Fatal("kafka must not silently degrade to noop")
	}
}
