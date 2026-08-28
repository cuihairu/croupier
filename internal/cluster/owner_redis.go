package cluster

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisOwnerResolver 是 OwnerStore 的 Redis 实现：归属租约靠 key TTL。
//
// 键布局（o: = owner）：
//
//	croupier:cluster:o:owner:{agentID}  HASH{instanceID, ownerEpoch, gameID, env, lastSeenAt}  TTL=ownerTTL
//	croupier:cluster:o:agents           SET 目录（ListAliveOwners 的发现索引）
type RedisOwnerResolver struct {
	rdb      redis.UniversalClient
	self     *MeshInterconnect
	selfID   string
	ownerTTL time.Duration
}

// NewRedisOwnerResolver 创建 Redis 归属表。ownerTTL <= 0 用默认 3 分钟。
func NewRedisOwnerResolver(rdb redis.UniversalClient, ownerTTL time.Duration) *RedisOwnerResolver {
	if ownerTTL <= 0 {
		ownerTTL = 3 * time.Minute
	}
	return &RedisOwnerResolver{rdb: rdb, ownerTTL: ownerTTL}
}

const (
	ownerKeyPrefix = "croupier:cluster:o:owner:"
	ownerDirKey    = "croupier:cluster:o:agents"
)

func ownerKey(agentID string) string { return ownerKeyPrefix + agentID }

// touchScript 原子续期：仅当记录仍归属本实例时续 TTL（被接管则不动）。
var touchScript = redis.NewScript(`
local v = redis.call('HGET', KEYS[1], 'instanceID')
if v == ARGV[1] then
  return redis.call('EXPIRE', KEYS[1], ARGV[2])
end
return 0
`)

// releaseScript 原子释放：仅当记录仍归属本实例时删除记录与目录项
// （读后删的两步版本在「校验后、删除前被新实例 Claim」时会误删新归属）。
var releaseScript = redis.NewScript(`
local v = redis.call('HGET', KEYS[1], 'instanceID')
if v == ARGV[1] then
  redis.call('DEL', KEYS[1])
  redis.call('SREM', KEYS[2], ARGV[2])
  return 1
end
return 0
`)

// ClaimOwner 声明归属（覆盖写：Agent 重连到新实例时接管）。
func (r *RedisOwnerResolver) ClaimOwner(ctx context.Context, agentID, gameID, env, instanceID string, epoch uint64) error {
	if r == nil || r.rdb == nil {
		return fmt.Errorf("redis owner store not configured")
	}
	fields := map[string]interface{}{
		"instanceID": instanceID,
		"ownerEpoch": strconv.FormatUint(epoch, 10),
		"gameID":     gameID,
		"env":        env,
		"lastSeenAt": strconv.FormatInt(time.Now().UTC().Unix(), 10),
	}
	pipe := r.rdb.Pipeline()
	pipe.HSet(ctx, ownerKey(agentID), fields)
	pipe.Expire(ctx, ownerKey(agentID), r.ownerTTL)
	pipe.SAdd(ctx, ownerDirKey, agentID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cluster: claim owner: %w", err)
	}
	return nil
}

// Touch 续期（Agent 心跳路径；记录被接管时静默不动）。
func (r *RedisOwnerResolver) Touch(ctx context.Context, agentID string) error {
	return touchScript.Run(ctx, r.rdb,
		[]string{ownerKey(agentID)},
		r.selfInstanceID(), strconv.FormatInt(int64(r.ownerTTL.Seconds()), 10),
	).Err()
}

// Release 释放归属（仅本实例持有的记录；原子脚本防误删新归属）。
func (r *RedisOwnerResolver) Release(ctx context.Context, agentID string) error {
	if err := releaseScript.Run(ctx, r.rdb,
		[]string{ownerKey(agentID), ownerDirKey},
		r.selfInstanceID(), agentID,
	).Err(); err != nil {
		return fmt.Errorf("cluster: release owner: %w", err)
	}
	return nil
}

func (r *RedisOwnerResolver) readOwner(ctx context.Context, agentID string) (*AgentOwnerRecord, bool, error) {
	fields, err := r.rdb.HGetAll(ctx, ownerKey(agentID)).Result()
	if err != nil {
		return nil, false, fmt.Errorf("cluster: read owner %s: %w", agentID, err)
	}
	if len(fields) == 0 {
		return nil, false, nil
	}
	epoch, _ := strconv.ParseUint(fields["ownerEpoch"], 10, 64)
	lastSeenUnix, _ := strconv.ParseInt(fields["lastSeenAt"], 10, 64)
	return &AgentOwnerRecord{
		AgentID:    agentID,
		InstanceID: fields["instanceID"],
		OwnerEpoch: epoch,
		GameID:     fields["gameID"],
		Env:        fields["env"],
		LastSeenAt: time.Unix(lastSeenUnix, 0).UTC(),
	}, true, nil
}

// ResolveOwner 返回持有该 Agent 的存活实例（peers 交叉验证，语义对齐
// DBOwnerResolver：owner 记录在 TTL 内且实例成员租约存活）。
func (r *RedisOwnerResolver) ResolveOwner(ctx context.Context, agentID string) (*PeerInfo, error) {
	rec, found, err := r.readOwner(ctx, agentID)
	if err != nil || !found {
		return nil, err
	}
	selfID := r.selfInstanceID()
	if rec.InstanceID == selfID {
		return &PeerInfo{InstanceID: rec.InstanceID, Epoch: rec.OwnerEpoch}, nil
	}
	if r.self != nil {
		for _, p := range r.self.Peers() {
			if p.InstanceID == rec.InstanceID && p.Epoch >= rec.OwnerEpoch {
				return &p, nil
			}
		}
		return nil, nil
	}
	return &PeerInfo{InstanceID: rec.InstanceID, Epoch: rec.OwnerEpoch}, nil
}

// FindOwner 回读归属记录（key 过期即视为不存在）。
func (r *RedisOwnerResolver) FindOwner(ctx context.Context, agentID string) (*AgentOwnerRecord, error) {
	rec, found, err := r.readOwner(ctx, agentID)
	if err != nil || !found {
		return nil, err
	}
	return rec, nil
}

// ListAliveOwners 返回 TTL 内全部归属记录（TTL 由 key 生存期保证，
// 目录残 ID 惰性清理）。
func (r *RedisOwnerResolver) ListAliveOwners(ctx context.Context) ([]AgentOwnerRecord, error) {
	ids, err := r.rdb.SMembers(ctx, ownerDirKey).Result()
	if err != nil {
		return nil, fmt.Errorf("cluster: list owner dir: %w", err)
	}
	records := make([]AgentOwnerRecord, 0, len(ids))
	var stale []string
	for _, id := range ids {
		rec, found, err := r.readOwner(ctx, id)
		if err != nil {
			return nil, err
		}
		if !found {
			stale = append(stale, id)
			continue
		}
		records = append(records, *rec)
	}
	if len(stale) > 0 {
		_ = r.rdb.SRem(ctx, ownerDirKey, stale).Err()
	}
	sort.Slice(records, func(i, j int) bool { return records[i].AgentID < records[j].AgentID })
	return records, nil
}

// CountAgentsByOwner 聚合每实例持有的活跃 agent 数。
func (r *RedisOwnerResolver) CountAgentsByOwner(ctx context.Context) (map[string]int64, error) {
	records, err := r.ListAliveOwners(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(records))
	for _, rec := range records {
		out[rec.InstanceID]++
	}
	return out, nil
}

// SelfOwnerScope 返回 agent 的 scope，仅当本实例是当前 Claim 持有者。
func (r *RedisOwnerResolver) SelfOwnerScope(ctx context.Context, agentID string) (gameID, env string, ok bool) {
	rec, found, err := r.readOwner(ctx, agentID)
	if err != nil || !found || rec.InstanceID != r.selfInstanceID() {
		return "", "", false
	}
	return rec.GameID, rec.Env, true
}

// SetMesh 注入 mesh（解析 owner 时交叉验证实例租约）。
func (r *RedisOwnerResolver) SetMesh(m *MeshInterconnect) { r.self = m }

// SetSelfID 显式指定本实例 ID（无 mesh 场景；生产主链路走 SetMesh）。
func (r *RedisOwnerResolver) SetSelfID(id string) { r.selfID = id }

func (r *RedisOwnerResolver) selfInstanceID() string {
	if r.selfID != "" {
		return r.selfID
	}
	if r.self != nil {
		return r.self.SelfInfo().InstanceID
	}
	return ""
}

var _ OwnerStore = (*RedisOwnerResolver)(nil)
