package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFastRedisV9(t *testing.T, mr *miniredis.Miniredis) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestRedisCtorDefaultsAndNilV9(t *testing.T) {
	_, rdb := newTestRedis(t)
	assert.Equal(t, 15*time.Second, NewRedisMembership(rdb, 0).leaseTTL)
	assert.Equal(t, 3*time.Minute, NewRedisOwnerResolver(rdb, 0).ownerTTL)

	ctx := context.Background()
	var nilMembership *RedisMembership
	_, err := nilMembership.Register(ctx, PeerInfo{InstanceID: "a"})
	assert.Error(t, err)
	_, err = NewRedisMembership(nil, time.Second).Register(ctx, PeerInfo{InstanceID: "a"})
	assert.Error(t, err)

	var nilOwner *RedisOwnerResolver
	err = nilOwner.ClaimOwner(ctx, "ag", "g", "e", "i", 1)
	assert.Error(t, err)
	assert.Error(t, NewRedisOwnerResolver(nil, time.Minute).ClaimOwner(ctx, "ag", "g", "e", "i", 1))
}

func TestRedisMembership_ErrorPathsV9(t *testing.T) {
	mr, _ := newTestRedis(t)
	rdb := newFastRedisV9(t, mr)
	ctx := context.Background()
	m := NewRedisMembership(rdb, time.Minute)

	require.NoError(t, rdb.Set(ctx, memberKey("bad-inst"), "not-a-hash", 0).Err())
	_, err := m.Register(ctx, PeerInfo{InstanceID: "bad-inst", AdvertiseAddr: "a:1"})
	assert.Error(t, err, "HSet 到 string key 应报 WRONGTYPE")

	require.NoError(t, rdb.SAdd(ctx, membersDirKey, "bad-inst2").Err())
	require.NoError(t, rdb.Set(ctx, memberKey("bad-inst2"), "not-a-hash", 0).Err())
	_, err = m.ListAlive(ctx)
	assert.Error(t, err, "HGetAll 到 string key 应报 WRONGTYPE")

	mr.Close()
	_, err = m.Register(ctx, PeerInfo{InstanceID: "x", AdvertiseAddr: "x:1"})
	assert.Error(t, err)
	assert.Error(t, m.Renew(ctx, "x"))
	_, err = m.ListAlive(ctx)
	assert.Error(t, err)
	assert.Error(t, m.Resign(ctx, "x"))
}

func TestRedisOwner_ErrorPathsV9(t *testing.T) {
	mr, _ := newTestRedis(t)
	rdb := newFastRedisV9(t, mr)
	ctx := context.Background()
	r := NewRedisOwnerResolver(rdb, time.Minute)
	r.SetSelfID("me")

	require.NoError(t, rdb.Set(ctx, ownerKey("bad-ag"), "not-a-hash", 0).Err())
	assert.Error(t, r.ClaimOwner(ctx, "bad-ag", "g", "e", "me", 1), "HSet 到 string key 应报 WRONGTYPE")

	mr.Close()
	assert.Error(t, r.ClaimOwner(ctx, "ag", "g", "e", "me", 1))
	assert.Error(t, r.Release(ctx, "ag"))
	_, err := r.FindOwner(ctx, "ag")
	assert.Error(t, err)
	_, err = r.ResolveOwner(ctx, "ag")
	assert.Error(t, err)
	_, err = r.ListAliveOwners(ctx)
	assert.Error(t, err)
	_, err = r.CountAgentsByOwner(ctx)
	assert.Error(t, err)
	_, _, ok := r.SelfOwnerScope(ctx, "ag")
	assert.False(t, ok)
}

func TestRedisOwner_WrongTypeAndStaleDirV9(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()
	r := NewRedisOwnerResolver(rdb, time.Minute)

	require.NoError(t, r.ClaimOwner(ctx, "ag-1", "g", "e", "i1", 1))
	require.NoError(t, rdb.SAdd(ctx, ownerDirKey, "ghost-ag").Err())

	records, err := r.ListAliveOwners(ctx)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "ag-1", records[0].AgentID)
	members, err := rdb.SMembers(ctx, ownerDirKey).Result()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"ag-1"}, members, "目录残 ID 应被惰性清理")

	require.NoError(t, rdb.SAdd(ctx, ownerDirKey, "bad-ag").Err())
	require.NoError(t, rdb.Set(ctx, ownerKey("bad-ag"), "not-a-hash", 0).Err())
	_, err = r.ListAliveOwners(ctx)
	assert.Error(t, err, "readOwner 遇 WRONGTYPE 应报错")
	_, err = r.CountAgentsByOwner(ctx)
	assert.Error(t, err)
}

func TestRedisOwner_ResolveOwnerPeersV9(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()

	mesh := NewMeshInterconnect(PeerInfo{InstanceID: "self-i", AdvertiseAddr: "s:1", Epoch: 1}, nil, nil)
	mesh.peersMu.Lock()
	mesh.peersCache = []PeerInfo{{InstanceID: "peer-a", AdvertiseAddr: "p:1", Epoch: 5}}
	mesh.peersMu.Unlock()

	rMesh := NewRedisOwnerResolver(rdb, time.Minute)
	rMesh.SetMesh(mesh)

	require.NoError(t, rMesh.ClaimOwner(ctx, "ag-self", "g", "e", "self-i", 1))
	owner, err := rMesh.ResolveOwner(ctx, "ag-self")
	require.NoError(t, err)
	require.NotNil(t, owner)
	assert.Equal(t, "self-i", owner.InstanceID)

	require.NoError(t, rMesh.ClaimOwner(ctx, "ag-peer", "g", "e", "peer-a", 5))
	owner, err = rMesh.ResolveOwner(ctx, "ag-peer")
	require.NoError(t, err)
	require.NotNil(t, owner)
	assert.Equal(t, "peer-a", owner.InstanceID)
	assert.EqualValues(t, 5, owner.Epoch)
	assert.Equal(t, "p:1", owner.AdvertiseAddr)

	require.NoError(t, rMesh.ClaimOwner(ctx, "ag-dead", "g", "e", "peer-z", 1))
	owner, err = rMesh.ResolveOwner(ctx, "ag-dead")
	require.NoError(t, err)
	assert.Nil(t, owner, "owner 实例不在 last-known peers 应视为无 owner")

	require.NoError(t, rMesh.ClaimOwner(ctx, "ag-stale", "g", "e", "peer-a", 99))
	owner, err = rMesh.ResolveOwner(ctx, "ag-stale")
	require.NoError(t, err)
	assert.Nil(t, owner, "peer epoch 低于 owner epoch 应视为僵尸 owner")

	rNoMesh := NewRedisOwnerResolver(rdb, time.Minute)
	rNoMesh.SetSelfID("me")
	require.NoError(t, rNoMesh.ClaimOwner(ctx, "ag-other", "g", "e", "other-inst", 3))
	owner, err = rNoMesh.ResolveOwner(ctx, "ag-other")
	require.NoError(t, err)
	require.NotNil(t, owner)
	assert.Equal(t, "other-inst", owner.InstanceID)

	g, e, ok := rNoMesh.SelfOwnerScope(ctx, "ag-other")
	assert.False(t, ok)
	assert.Empty(t, g)
	assert.Empty(t, e)
}

func TestRedisOwner_SelfInstanceIDV9(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()

	r := NewRedisOwnerResolver(rdb, time.Minute)
	assert.NoError(t, r.Touch(ctx, "any"), "无 mesh/selfID 时 selfInstanceID 为空串，Touch 静默无效果")

	mesh := NewMeshInterconnect(PeerInfo{InstanceID: "mesh-i", Epoch: 2}, nil, nil)
	rWithMesh := NewRedisOwnerResolver(rdb, time.Minute)
	rWithMesh.SetMesh(mesh)
	require.NoError(t, rWithMesh.ClaimOwner(ctx, "ag-m", "g", "e", "mesh-i", 2))
	assert.NoError(t, rWithMesh.Touch(ctx, "ag-m"))
	assert.NoError(t, rWithMesh.Release(ctx, "ag-m"))
	rec, err := rWithMesh.FindOwner(ctx, "ag-m")
	require.NoError(t, err)
	assert.Nil(t, rec)
}
