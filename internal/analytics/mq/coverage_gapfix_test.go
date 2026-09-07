package mq

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	kafka "github.com/segmentio/kafka-go"
)

// fakeKafkaWriter 记录写入消息并可注入 Close/WriteMessages 故障。
type fakeKafkaWriter struct {
	closeErr error
	writeErr error
	msgs     []kafka.Message
}

func (f *fakeKafkaWriter) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.msgs = append(f.msgs, msgs...)
	return nil
}

func (f *fakeKafkaWriter) Close() error { return f.closeErr }

func overrideRedisFromEnv(t *testing.T, fn func() (Queue, error)) {
	t.Helper()
	prev := newRedisFromEnvFn
	newRedisFromEnvFn = fn
	t.Cleanup(func() { newRedisFromEnvFn = prev })
}

func overrideKafkaFromEnv(t *testing.T, fn func() (Queue, error)) {
	t.Helper()
	prev := newKafkaFromEnvFn
	newKafkaFromEnvFn = fn
	t.Cleanup(func() { newKafkaFromEnvFn = prev })
}

// newRedisFromEnv 唯一返回形态为 (非 nil Queue, nil error)，redis 分支的
// 错误包装仅在缝隙注入 (nil, err) 时可触达。
func TestNewFromEnv_RedisConstructionErrorWrapped(t *testing.T) {
	withMQType(t, "redis")
	overrideRedisFromEnv(t, func() (Queue, error) {
		return nil, errors.New("redis dial boom")
	})

	q, err := NewFromEnv()
	if err == nil {
		t.Fatal("injected redis construction failure must surface")
	}
	if q != nil {
		t.Fatal("error path must return nil queue")
	}
	if !strings.Contains(err.Error(), "analytics redis mq: redis dial boom") {
		t.Fatalf("error must wrap injected cause, got: %v", err)
	}
}

// redis 分支的 nil publisher 防御仅在缝隙注入 (nil, nil) 时可触达。
func TestNewFromEnv_RedisNilPublisherRejected(t *testing.T) {
	withMQType(t, "redis")
	overrideRedisFromEnv(t, func() (Queue, error) { return nil, nil })

	q, err := NewFromEnv()
	if err == nil {
		t.Fatal("nil redis publisher must be rejected")
	}
	if q != nil {
		t.Fatal("error path must return nil queue")
	}
	if err.Error() != "analytics redis mq: nil publisher" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// newKafkaFromEnv 唯一返回形态为 (非 nil Queue, nil error)，kafka 分支的
// 错误包装仅在缝隙注入 (nil, err) 时可触达。
func TestNewFromEnv_KafkaConstructionErrorWrapped(t *testing.T) {
	withMQType(t, "kafka")
	overrideKafkaFromEnv(t, func() (Queue, error) {
		return nil, errors.New("kafka writer boom")
	})

	q, err := NewFromEnv()
	if err == nil {
		t.Fatal("injected kafka construction failure must surface")
	}
	if q != nil {
		t.Fatal("error path must return nil queue")
	}
	if !strings.Contains(err.Error(), "analytics kafka mq: kafka writer boom") {
		t.Fatalf("error must wrap injected cause, got: %v", err)
	}
}

// kafka 分支的 nil publisher 防御仅在缝隙注入 (nil, nil) 时可触达。
func TestNewFromEnv_KafkaNilPublisherRejected(t *testing.T) {
	withMQType(t, "kafka")
	overrideKafkaFromEnv(t, func() (Queue, error) { return nil, nil })

	q, err := NewFromEnv()
	if err == nil {
		t.Fatal("nil kafka publisher must be rejected")
	}
	if q != nil {
		t.Fatal("error path must return nil queue")
	}
	if err.Error() != "analytics kafka mq: nil publisher" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// kafka-go v0.4.50 的 Writer.Close 恒返回 nil，Close 的错误传播分支仅在
// 缝隙注入故障 writer 时可触达：events writer 失败时错误必须向上传播。
func TestKafkaQueue_ClosePropagatesEventsWriterError(t *testing.T) {
	boom := errors.New("events writer close boom")
	q := &kafkaQueue{
		wEvents:   &fakeKafkaWriter{closeErr: boom},
		wPayments: &fakeKafkaWriter{},
	}
	if err := q.Close(); !errors.Is(err, boom) {
		t.Fatalf("Close must propagate events writer error, got: %v", err)
	}
}

// 两个 writer 均失败时，payments 的错误后写覆盖 events 的错误（现行语义）。
func TestKafkaQueue_ClosePaymentsErrorWins(t *testing.T) {
	eventsErr := errors.New("events close boom")
	payErr := errors.New("payments close boom")
	q := &kafkaQueue{
		wEvents:   &fakeKafkaWriter{closeErr: eventsErr},
		wPayments: &fakeKafkaWriter{closeErr: payErr},
	}
	if err := q.Close(); !errors.Is(err, payErr) {
		t.Fatalf("Close must return the payments writer error (last write wins), got: %v", err)
	}
}

// PublishEvent/PublishPayment 走 write：payload 必须序列化为 JSON 消息体，
// 且 write 错误原样传播。
func TestKafkaQueue_WriteMarshalsPayloadAndPropagatesError(t *testing.T) {
	events := &fakeKafkaWriter{}
	payments := &fakeKafkaWriter{}
	q := &kafkaQueue{wEvents: events, wPayments: payments}

	evt := map[string]any{"event": "login", "uid": 42}
	if err := q.PublishEvent(evt); err != nil {
		t.Fatalf("PublishEvent with healthy writer must succeed, got %v", err)
	}
	if len(events.msgs) != 1 {
		t.Fatalf("expected exactly one event message, got %d", len(events.msgs))
	}
	var decoded map[string]any
	if err := json.Unmarshal(events.msgs[0].Value, &decoded); err != nil {
		t.Fatalf("event payload must be valid JSON: %v", err)
	}
	if decoded["event"] != "login" || decoded["uid"] != float64(42) {
		t.Fatalf("payload mismatch: %v", decoded)
	}

	pay := map[string]any{"order": "o-1"}
	if err := q.PublishPayment(pay); err != nil {
		t.Fatalf("PublishPayment with healthy writer must succeed, got %v", err)
	}
	if len(payments.msgs) != 1 {
		t.Fatalf("expected exactly one payment message, got %d", len(payments.msgs))
	}
	if strings.TrimSpace(string(payments.msgs[0].Value)) != `{"order":"o-1"}` {
		t.Fatalf("payment payload mismatch: %s", payments.msgs[0].Value)
	}

	writeBoom := errors.New("write boom")
	qFailing := &kafkaQueue{wEvents: &fakeKafkaWriter{writeErr: writeBoom}}
	if err := qFailing.PublishEvent(evt); !errors.Is(err, writeBoom) {
		t.Fatalf("PublishEvent must propagate write error, got: %v", err)
	}
}

// write 对 nil writer 的容忍分支：半构造的 kafkaQueue 不 panic 且不产生副作用。
func TestKafkaQueue_WriteNilWriterNoOp(t *testing.T) {
	q := &kafkaQueue{}
	if err := q.PublishEvent(map[string]any{"k": "v"}); err != nil {
		t.Fatalf("nil writer write must be a no-op, got %v", err)
	}
	if err := q.PublishPayment(map[string]any{"k": "v"}); err != nil {
		t.Fatalf("nil writer write must be a no-op, got %v", err)
	}
	if err := q.Close(); err != nil {
		t.Fatalf("Close with nil writers must return nil, got %v", err)
	}
}

// 真实 writer 经缝隙接口仍满足 kafkaQueue 依赖（编译期契约）。
var _ kafkaWriter = (*kafka.Writer)(nil)

// PendingEvents/PendingPayments 恒 (0, nil)。
func TestKafkaQueue_PendingZero(t *testing.T) {
	q := &kafkaQueue{}
	n, err := q.PendingEvents()
	if n != 0 || err != nil {
		t.Fatalf("PendingEvents must be (0, nil), got (%d, %v)", n, err)
	}
	n, err = q.PendingPayments()
	if n != 0 || err != nil {
		t.Fatalf("PendingPayments must be (0, nil), got (%d, %v)", n, err)
	}
}
