package cluster

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisMembership is the Redis implementation of Membership: member leases
// rely on key TTL, and the directory set (SET) is only a discovery index —
// expired members' IDs linger in the directory and ListAlive filters them out
// while probing one by one (with lazy cleanup along the way).
//
// Key layout (m: = membership):
//
//	croupier:cluster:m:member:{instanceID}  HASH{advertiseAddr, startedAt, epoch}  TTL=leaseTTL
//	croupier:cluster:m:members              SET directory (SADD/SREM)
//	croupier:cluster:m:epoch:{instanceID}   INCR counter (no TTL — the fencing
//	                                       epoch must increase monotonically
//	                                       across restarts)
type RedisMembership struct {
	rdb      redis.UniversalClient
	leaseTTL time.Duration
}

// NewRedisMembership creates the Redis member table. leaseTTL <= 0 falls back
// to the default 15s.
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

// Register self-registers and returns the assigned epoch.
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

// Renew renews the lease: the TTL is extended only if the member key still
// exists (an expired key = lost lease; returning an error lets the caller
// notice and re-register to get a new epoch).
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

// ListAlive returns all members whose leases have not expired.
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
	// Lazily clean up stale directory IDs (best-effort).
	if len(stale) > 0 {
		_ = m.rdb.SRem(ctx, membersDirKey, stale).Err()
	}
	return alive, nil
}

// Resign exits gracefully: deletes the member key and removes it from the
// directory.
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
