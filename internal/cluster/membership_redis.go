package cluster

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisMembership 是 Membership 的 Redis 实现：成员租约靠 key TTL，
// 目录集合（SET）只做发现索引——过期成员的 ID 会残留在目录里，
// ListAlive 逐个探测时过滤（顺带惰性清理）。
//
// 键布局（m: = membership）：
//
//	croupier:cluster:m:member:{instanceID}  HASH{advertiseAddr, startedAt, epoch}  TTL=leaseTTL
//	croupier:cluster:m:members              SET 目录（SADD/SREM）
//	croupier:cluster:m:epoch:{instanceID}   INCR 计数器（无 TTL——fencing
//	                                       epoch 必须跨重启单调递增）
type RedisMembership struct {
	rdb      redis.UniversalClient
	leaseTTL time.Duration
}

// NewRedisMembership 创建 Redis 成员表。leaseTTL <= 0 用默认 15s。
func NewRedisMembership(rdb redis.UniversalClient, leaseTTL time.Duration) *RedisMembership {
	if leaseTTL <= 0 {
		leaseTTL = 15 * time.Second
	}
	return &RedisMembership{rdb: rdb, leaseTTL: leaseTTL}
}

const (
	memberKeyPrefix = "croupier:cluster:m:member:"
	membersDirKey   = "croupier:cluster:m:members"
	epochKeyPrefix  = "croupier:cluster:m:epoch:"
)

func memberKey(instanceID string) string { return memberKeyPrefix + instanceID }
func epochKey(instanceID string) string  { return epochKeyPrefix + instanceID }

// Register 自注册并返回分配的 epoch。
func (m *RedisMembership) Register(ctx context.Context, info PeerInfo) (uint64, error) {
	if m == nil || m.rdb == nil {
		return 0, fmt.Errorf("redis membership not configured")
	}
	epoch, err := m.rdb.Incr(ctx, epochKey(info.InstanceID)).Result()
	if err != nil {
		return 0, fmt.Errorf("cluster: incr epoch: %w", err)
	}
	fields := map[string]interface{}{
		"advertiseAddr": info.AdvertiseAddr,
		"startedAt":     strconv.FormatInt(info.StartedAt.Unix(), 10),
		"epoch":         strconv.FormatUint(uint64(epoch), 10),
	}
	pipe := m.rdb.Pipeline()
	pipe.HSet(ctx, memberKey(info.InstanceID), fields)
	pipe.Expire(ctx, memberKey(info.InstanceID), m.leaseTTL)
	pipe.SAdd(ctx, membersDirKey, info.InstanceID)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("cluster: register member: %w", err)
	}
	return uint64(epoch), nil
}

// Renew 续租：成员 key 存在则续 TTL（key 已过期 = 租约丢失，报错让
// 调用方感知并触发重新注册拿新 epoch）。
func (m *RedisMembership) Renew(ctx context.Context, instanceID string) error {
	ok, err := m.rdb.Expire(ctx, memberKey(instanceID), m.leaseTTL).Result()
	if err != nil {
		return fmt.Errorf("cluster: renew member: %w", err)
	}
	if !ok {
		return fmt.Errorf("cluster: membership lease lost for %s", instanceID)
	}
	return nil
}

// ListAlive 返回租约未过期的全部成员。
func (m *RedisMembership) ListAlive(ctx context.Context) ([]PeerInfo, error) {
	ids, err := m.rdb.SMembers(ctx, membersDirKey).Result()
	if err != nil {
		return nil, fmt.Errorf("cluster: list member dir: %w", err)
	}
	alive := make([]PeerInfo, 0, len(ids))
	var stale []string
	for _, id := range ids {
		fields, err := m.rdb.HGetAll(ctx, memberKey(id)).Result()
		if err != nil {
			return nil, fmt.Errorf("cluster: read member %s: %w", id, err)
		}
		if len(fields) == 0 {
			stale = append(stale, id)
			continue
		}
		epoch, _ := strconv.ParseUint(fields["epoch"], 10, 64)
		startedUnix, _ := strconv.ParseInt(fields["startedAt"], 10, 64)
		alive = append(alive, PeerInfo{
			InstanceID:    id,
			AdvertiseAddr: fields["advertiseAddr"],
			StartedAt:     time.Unix(startedUnix, 0).UTC(),
			Epoch:         epoch,
		})
	}
	// 惰性清理目录残 ID（best-effort）。
	if len(stale) > 0 {
		_ = m.rdb.SRem(ctx, membersDirKey, stale).Err()
	}
	return alive, nil
}

// Resign 优雅退出：删成员 key 并从目录移除。
func (m *RedisMembership) Resign(ctx context.Context, instanceID string) error {
	pipe := m.rdb.Pipeline()
	pipe.Del(ctx, memberKey(instanceID))
	pipe.SRem(ctx, membersDirKey, instanceID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("cluster: resign member: %w", err)
	}
	return nil
}

var _ Membership = (*RedisMembership)(nil)
