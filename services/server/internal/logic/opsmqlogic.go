package logic

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/redis/go-redis/v9"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsMQLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsMQLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMQLogic {
	return &OpsMQLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMQLogic) OpsMQ() (*types.OpsMQResponse, error) {
	typ := strings.TrimSpace(os.Getenv("ANALYTICS_MQ_TYPE"))
	if typ == "" {
		typ = "noop"
	}
	resp := &types.OpsMQResponse{
		Type: typ,
	}
	switch typ {
	case "redis":
		info, lengths, groups := l.redisInfo()
		resp.Redis = info
		if len(lengths) > 0 {
			resp.Lengths = lengths
		}
		if len(groups) > 0 {
			resp.Groups = groups
		}
	case "kafka":
		resp.Kafka = l.kafkaInfo()
	default:
	}
	return resp, nil
}

func (l *OpsMQLogic) redisInfo() (*types.OpsMQRedis, map[string]int64, []types.OpsMQGroup) {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	events := os.Getenv("ANALYTICS_REDIS_STREAM_EVENTS")
	if events == "" {
		events = "analytics:events"
	}
	payments := os.Getenv("ANALYTICS_REDIS_STREAM_PAYMENTS")
	if payments == "" {
		payments = "analytics:payments"
	}
	info := &types.OpsMQRedis{
		URL: url,
		Streams: map[string]string{
			"events":   events,
			"payments": payments,
		},
	}
	if url == "" {
		return info, map[string]int64{}, []types.OpsMQGroup{}
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return info, map[string]int64{}, []types.OpsMQGroup{}
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(l.ctx, 800*time.Millisecond)
	defer cancel()
	lengths := map[string]int64{}
	groups := []types.OpsMQGroup{}
	collect := func(stream string) {
		if stream == "" {
			return
		}
		if n, err := client.XLen(ctx, stream).Result(); err == nil {
			lengths[stream] = n
		}
		if gs, err := client.XInfoGroups(ctx, stream).Result(); err == nil {
			for _, g := range gs {
				groups = append(groups, types.OpsMQGroup{
					Stream:      stream,
					Name:        g.Name,
					Consumers:   g.Consumers,
					Pending:     g.Pending,
					EntriesRead: g.EntriesRead,
					Lag:         g.Lag,
				})
			}
		}
	}
	collect(events)
	collect(payments)
	_ = client.Close()
	return info, lengths, groups
}

func (l *OpsMQLogic) kafkaInfo() *types.OpsMQKafka {
	brokers := strings.TrimSpace(os.Getenv("KAFKA_BROKERS"))
	events := firstNonEmpty(
		os.Getenv("ANALYTICS_KAFKA_TOPIC_EVENTS"),
		os.Getenv("KAFKA_TOPIC_EVENTS"),
		"analytics.events",
	)
	payments := firstNonEmpty(
		os.Getenv("ANALYTICS_KAFKA_TOPIC_PAYMENTS"),
		os.Getenv("KAFKA_TOPIC_PAYMENTS"),
		"analytics.payments",
	)
	return &types.OpsMQKafka{
		Brokers: brokers,
		Topics: map[string]string{
			"events":   events,
			"payments": payments,
		},
	}
}
