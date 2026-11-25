package mq

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	kafka "github.com/segmentio/kafka-go"
)

// fakeKafkaWriter records written messages and can inject Close/WriteMessages failures.
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

// newRedisFromEnv's only possible return shape is (non-nil Queue, nil error); the
// error wrapping in the redis branch is reachable only via a (nil, err) seam injection.
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

// The nil publisher guard in the redis branch is reachable only via a (nil, nil) seam injection.
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

// newKafkaFromEnv's only possible return shape is (non-nil Queue, nil error); the
// error wrapping in the kafka branch is reachable only via a (nil, err) seam injection.
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

// The nil publisher guard in the kafka branch is reachable only via a (nil, nil) seam injection.
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

// kafka-go v0.4.50's Writer.Close always returns nil, so the error propagation
// branch of Close is reachable only by injecting a broken writer through the seam:
// when the events writer fails, the error must propagate upward.
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

// When both writers fail, the payments error, written last, overwrites the events
// error (current semantics).
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

// PublishEvent/PublishPayment go through write: the payload must be serialized into
// a JSON message body, and write errors must propagate as-is.
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

// write's nil-writer tolerance branch: a half-constructed kafkaQueue must not
// panic and must have no side effects.
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

// The real writer still satisfies kafkaQueue's dependency through the seam
// interface (compile-time contract).
var _ kafkaWriter = (*kafka.Writer)(nil)

// PendingEvents/PendingPayments always return (0, nil).
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
