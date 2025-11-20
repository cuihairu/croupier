package logic

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsRealtimeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsRealtimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsRealtimeLogic {
	return &AnalyticsRealtimeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsRealtimeLogic) AnalyticsRealtime(req *types.AnalyticsRealtimeQuery) (*types.AnalyticsRealtimeResponse, error) {
	resp := &types.AnalyticsRealtimeResponse{
		Rev5mYuan:    "0.00",
		RevTodayYuan: "0.00",
	}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	wGame, wEnv, filterArgs := buildAnalyticsFilters(strings.TrimSpace(req.GameId), strings.TrimSpace(req.Env))
	_ = ch.QueryRow(l.ctx, "SELECT online FROM analytics.minute_online WHERE 1=1"+wGame+wEnv+" ORDER BY m DESC LIMIT 1", filterArgs...).Scan(&resp.Online)
	_ = ch.QueryRow(l.ctx, "SELECT sum(online) FROM analytics.minute_online WHERE m>now()-interval 1 minute"+wGame+wEnv, filterArgs...).Scan(&resp.Active1m)
	_ = ch.QueryRow(l.ctx, "SELECT sum(online) FROM analytics.minute_online WHERE m>now()-interval 5 minute"+wGame+wEnv, filterArgs...).Scan(&resp.Active5m)
	_ = ch.QueryRow(l.ctx, "SELECT sum(online) FROM analytics.minute_online WHERE m>now()-interval 15 minute"+wGame+wEnv, filterArgs...).Scan(&resp.Active15m)
	var succ, total uint64
	_ = ch.QueryRow(l.ctx, "SELECT sumIf(amount_cents, status='success') FROM analytics.payments WHERE time>now()-interval 5 minute"+wGame+wEnv, filterArgs...).Scan(&resp.Rev5m)
	_ = ch.QueryRow(l.ctx, "SELECT countIf(status='success'), count() FROM analytics.payments WHERE time>now()-interval 5 minute"+wGame+wEnv, filterArgs...).Scan(&succ, &total)
	_ = ch.QueryRow(l.ctx, "SELECT sumIf(amount_cents, status='success') FROM analytics.payments WHERE time>=toStartOfDay(now())"+wGame+wEnv, filterArgs...).Scan(&resp.RevToday)
	l.applyRedisRealtimeEstimates(req.GameId, req.Env, resp)
	resp.Rev5mYuan = fmt.Sprintf("%.2f", float64(resp.Rev5m)/100.0)
	resp.RevTodayYuan = fmt.Sprintf("%.2f", float64(resp.RevToday)/100.0)
	if total > 0 {
		resp.PaySuccRate = float64(succ) * 100.0 / float64(total)
	}
	_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events WHERE event IN ('register','first_active')"+wGame+wEnv, filterArgs...).Scan(&resp.RegisteredTotal)
	_ = ch.QueryRow(l.ctx, "SELECT maxMerge(peak_online) FROM analytics.daily_online_peak WHERE d=today()"+wGame+wEnv, filterArgs...).Scan(&resp.OnlinePeakToday)
	_ = ch.QueryRow(l.ctx, "SELECT maxMerge(peak_online) FROM analytics.daily_online_peak WHERE 1=1"+wGame+wEnv, filterArgs...).Scan(&resp.OnlinePeakAllTime)
	today := time.Now().Format("2006-01-02")
	resp.DauToday = l.redisPFCount(fmt.Sprintf("hll:dau:%s:%s:%s", req.GameId, req.Env, today))
	resp.NewToday = l.redisPFCount(fmt.Sprintf("hll:new:%s:%s:%s", req.GameId, req.Env, today))
	if resp.DauToday == 0 {
		_ = ch.QueryRow(l.ctx, "SELECT sum(dau) FROM analytics.daily_users WHERE d=toDate(?)"+wGame+wEnv, append([]any{today}, filterArgs...)...).Scan(&resp.DauToday)
	}
	if resp.NewToday == 0 {
		_ = ch.QueryRow(l.ctx, "SELECT sum(new_users) FROM analytics.daily_users WHERE d=toDate(?)"+wGame+wEnv, append([]any{today}, filterArgs...)...).Scan(&resp.NewToday)
	}
	return resp, nil
}

func (l *AnalyticsRealtimeLogic) applyRedisRealtimeEstimates(game, env string, resp *types.AnalyticsRealtimeResponse) {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		return
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return
	}
	client := redis.NewClient(opt)
	defer client.Close()
	ctx, cancel := context.WithTimeout(l.ctx, 600*time.Millisecond)
	defer cancel()
	now := time.Now()
	curKey := fmt.Sprintf("hll:online:%s:%s:%s", game, env, now.Truncate(time.Minute).Format("200601021504"))
	if n, err := client.PFCount(ctx, curKey).Result(); err == nil && n >= 0 {
		resp.Online = uint64(n)
		resp.Active1m = uint64(n)
	}
	resp.Active5m = l.redisMergePF(ctx, client, game, env, now, 5)
	resp.Active15m = l.redisMergePF(ctx, client, game, env, now, 15)
}

func (l *AnalyticsRealtimeLogic) redisMergePF(ctx context.Context, client *redis.Client, game, env string, now time.Time, minutes int) uint64 {
	if minutes <= 0 {
		return 0
	}
	keys := make([]string, 0, minutes)
	for i := 0; i < minutes; i++ {
		t := now.Add(time.Duration(-i) * time.Minute)
		keys = append(keys, fmt.Sprintf("hll:online:%s:%s:%s", game, env, t.Truncate(time.Minute).Format("200601021504")))
	}
	tmp := fmt.Sprintf("tmp:hll:online:%d:%d", minutes, now.UnixNano())
	if err := client.PFMerge(ctx, tmp, keys...).Err(); err != nil {
		return 0
	}
	n, err := client.PFCount(ctx, tmp).Result()
	_ = client.Expire(ctx, tmp, 2*time.Second).Err()
	if err != nil || n < 0 {
		return 0
	}
	return uint64(n)
}

func (l *AnalyticsRealtimeLogic) redisPFCount(key string) uint64 {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" || key == "" {
		return 0
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return 0
	}
	client := redis.NewClient(opt)
	defer client.Close()
	ctx, cancel := context.WithTimeout(l.ctx, 500*time.Millisecond)
	defer cancel()
	n, err := client.PFCount(ctx, key).Result()
	if err != nil || n < 0 {
		return 0
	}
	return uint64(n)
}
