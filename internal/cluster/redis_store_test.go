package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestRedisMembership_Lifecycle(t *testing.T) {
	mr, rdb := newTestRedis(t)
	ctx := context.Background()
	m := NewRedisMembership(rdb, 15*time.Second)

	// Register：epoch 从 1 开始。
	e1, err := m.Register(ctx, PeerInfo{InstanceID: "a", AdvertiseAddr: "a:1", StartedAt: time.Now().UTC()})
	if err != nil || e1 != 1 {
		t.Fatalf("register: epoch=%d err=%v", e1, err)
	}
	_, err = m.Register(ctx, PeerInfo{InstanceID: "b", AdvertiseAddr: "b:1", StartedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("register b: %v", err)
	}
	alive, err := m.ListAlive(ctx)
	if err != nil || len(alive) != 2 {
		t.Fatalf("list alive = %v (err %v), want 2", alive, err)
	}

	// Renew 续租；TTL 过期后成员消失、目录惰性清理。
	if err := m.Renew(ctx, "a"); err != nil {
		t.Fatalf("renew: %v", err)
	}
	mr.FastForward(16 * time.Second)
	alive, err = m.ListAlive(ctx)
	if err != nil || len(alive) != 0 {
		t.Fatalf("after ttl: alive=%v err=%v, want 0", alive, err)
	}
	// 租约丢失后 Renew 必须报错（调用方触发重新注册）。
	if err := m.Renew(ctx, "a"); err == nil {
		t.Fatal("renew after expiry should fail")
	}

	// 重启复用 ID：epoch 单调递增（fencing）。
	_, err = m.Register(ctx, PeerInfo{InstanceID: "a", AdvertiseAddr: "a:2", StartedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("re-register: %v", err)
	}
	e3, err := m.Register(ctx, PeerInfo{InstanceID: "a", AdvertiseAddr: "a:3", StartedAt: time.Now().UTC()})
	if err != nil || e3 <= e1 {
		t.Fatalf("epoch must monotonically increase: %d -> %d", e1, e3)
	}

	// Resign：立即消失。
	if err := m.Resign(ctx, "a"); err != nil {
		t.Fatalf("resign: %v", err)
	}
	alive, _ = m.ListAlive(ctx)
	if len(alive) != 0 {
		t.Fatalf("after resign alive=%v, want 0", alive)
	}
}

func TestRedisOwnerStore_OwnerSemantics(t *testing.T) {
	mr, rdb := newTestRedis(t)
	ctx := context.Background()
	r := NewRedisOwnerResolver(rdb, 3*time.Minute)
	r.SetSelfID("inst-1")

	// Claim + Find + Resolve（self 直返）。
	if err := r.ClaimOwner(ctx, "ag-1", "g1", "prod", "inst-1", 5); err != nil {
		t.Fatalf("claim: %v", err)
	}
	rec, err := r.FindOwner(ctx, "ag-1")
	if err != nil || rec == nil || rec.InstanceID != "inst-1" || rec.GameID != "g1" || rec.OwnerEpoch != 5 {
		t.Fatalf("find owner: %+v err=%v", rec, err)
	}
	peer, err := r.ResolveOwner(ctx, "ag-1")
	if err != nil || peer == nil || peer.InstanceID != "inst-1" || peer.Epoch != 5 {
		t.Fatalf("resolve(self): %+v err=%v", peer, err)
	}

	// SelfOwnerScope：仅本实例持有时 ok。
	g, e, ok := r.SelfOwnerScope(ctx, "ag-1")
	if !ok || g != "g1" || e != "prod" {
		t.Fatalf("self scope: (%q,%q,%v)", g, e, ok)
	}

	// Touch：ownerTTL 过期前续命；FastForward 验证。
	mr.FastForward(2 * time.Minute)
	if err := r.Touch(ctx, "ag-1"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	mr.FastForward(2 * time.Minute) // 共 4m > 3m TTL，但 touch 在 2m 处续过
	if _, found, _ := r.readOwner(ctx, "ag-1"); !found {
		t.Fatal("owner expired despite touch")
	}

	// 被接管：inst-2 claim 后，inst-1 的 Touch/Release 不得影响新归属。
	if err := r.ClaimOwner(ctx, "ag-1", "g1", "prod", "inst-2", 6); err != nil {
		t.Fatalf("takeover claim: %v", err)
	}
	_ = r.Touch(ctx, "ag-1") // 不应续命/改归属
	mr.FastForward(3*time.Minute + time.Second)
	if _, found, _ := r.readOwner(ctx, "ag-1"); found {
		t.Fatal("takeover record should expire via own TTL (old owner touch must not extend it)")
	}

	// ListAliveOwners + CountAgentsByOwner。
	if err := r.ClaimOwner(ctx, "ag-1", "g1", "prod", "inst-1", 5); err != nil {
		t.Fatalf("claim1: %v", err)
	}
	if err := r.ClaimOwner(ctx, "ag-2", "g1", "prod", "inst-2", 5); err != nil {
		t.Fatalf("claim2: %v", err)
	}
	records, err := r.ListAliveOwners(ctx)
	if err != nil || len(records) != 2 {
		t.Fatalf("list alive owners: %v err=%v", records, err)
	}
	counts, err := r.CountAgentsByOwner(ctx)
	if err != nil || counts["inst-1"] != 1 || counts["inst-2"] != 1 {
		t.Fatalf("counts: %v err=%v", counts, err)
	}

	// Release：仅本实例持有的记录被删——inst-1 释放归属 inst-2 的
	// ag-2 必须无效果（防误删他人归属）。
	if err := r.Release(ctx, "ag-2"); err != nil {
		t.Fatalf("release: %v", err)
	}
	records, _ = r.ListAliveOwners(ctx)
	if len(records) != 2 {
		t.Fatalf("release of foreign-owned record must be no-op, got %v", records)
	}
	if err := r.Release(ctx, "ag-1"); err != nil {
		t.Fatalf("release own: %v", err)
	}
	records, _ = r.ListAliveOwners(ctx)
	if len(records) != 1 || records[0].AgentID != "ag-2" {
		t.Fatalf("after releasing own record: %v", records)
	}

	// 无记录：ResolveOwner/FindOwner 返回 nil, nil。
	if p, err := r.ResolveOwner(ctx, "nope"); err != nil || p != nil {
		t.Fatalf("resolve missing: %v %v", p, err)
	}
	if rec, err := r.FindOwner(ctx, "nope"); err != nil || rec != nil {
		t.Fatalf("find missing: %v %v", rec, err)
	}
}
