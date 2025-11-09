package mq

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

type kafkaQueue struct {
	wEvents   *kafka.Writer
	wPayments *kafka.Writer
}

func NewKafka(brokers []string, topicEvents, topicPayments string) Queue {
	if len(brokers) == 0 {
		return NewNoop()
	}
	if topicEvents == "" {
		topicEvents = "analytics.events"
	}
	if topicPayments == "" {
		topicPayments = "analytics.payments"
	}
	// Writers are safe for concurrent use
	we := &kafka.Writer{Addr: kafka.TCP(brokers...), Topic: topicEvents, RequiredAcks: kafka.RequireOne, Balancer: &kafka.LeastBytes{}, BatchTimeout: 50 * time.Millisecond}
	wp := &kafka.Writer{Addr: kafka.TCP(brokers...), Topic: topicPayments, RequiredAcks: kafka.RequireOne, Balancer: &kafka.LeastBytes{}, BatchTimeout: 50 * time.Millisecond}
	return &kafkaQueue{wEvents: we, wPayments: wp}
}

func newKafkaFromEnv() (Queue, error) {
	bs := strings.TrimSpace(os.Getenv("KAFKA_BROKERS"))
	if bs == "" {
		bs = "localhost:9092"
	}
	events := strings.TrimSpace(os.Getenv("ANALYTICS_KAFKA_TOPIC_EVENTS"))
	pays := strings.TrimSpace(os.Getenv("ANALYTICS_KAFKA_TOPIC_PAYMENTS"))
	q := NewKafka(strings.Split(bs, ","), events, pays)
	log.Printf("[analytics-mq] kafka publisher enabled: brokers=%s events=%s payments=%s", bs, events, pays)
	return q, nil
}

func (q *kafkaQueue) Close() error {
	var err error
	if q.wEvents != nil {
		if e := q.wEvents.Close(); e != nil {
			err = e
		}
	}
	if q.wPayments != nil {
		if e := q.wPayments.Close(); e != nil {
			err = e
		}
	}
	return err
}

func (q *kafkaQueue) write(w *kafka.Writer, m map[string]any) error {
	if w == nil {
		return nil
	}
	b, _ := json.Marshal(m)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return w.WriteMessages(ctx, kafka.Message{Value: b})
}

func (q *kafkaQueue) PublishEvent(evt map[string]any) error   { return q.write(q.wEvents, evt) }
func (q *kafkaQueue) PublishPayment(pay map[string]any) error { return q.write(q.wPayments, pay) }
