package worker

import (
	"context"
	"testing"

	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 验收（DATA-055）：共享同一 stream 的两个消费组各自持有独立检查点——
// 较旧进度不再覆盖较新进度（旧无分组共享键的复现探针），旧键永不写入，
// 重启后各自地板互不影响。
func TestGroupScopedCheckpoint_IsolationAndRestart(t *testing.T) {
	wA, mr := testWorker(t)
	ctx := context.Background()
	wB := testWorkerOn(t, mr)
	wB.group = "analytics-worker-b" // 第二个消费组，共享同一 stream 与前缀

	id1 := seedEvent(t, wA, "u1")
	id2 := seedEvent(t, wA, "u2")
	wA.ensureGroups(ctx)
	wB.ensureGroups(ctx)
	readNew(t, wA, wA.streamEvents)
	readNew(t, wB, wB.streamEvents)

	// 两个组消费进度不同：A 停在较旧的消息上，B 已推进到较新的。
	wA.advanceCheckpoint(ctx, wA.streamEvents, id1)
	wB.advanceCheckpoint(ctx, wB.streamEvents, id2)

	// 各组读回自己的进度，不再互相覆盖（旧行为：共享键，双方都是 id2）。
	assert.Equal(t, id1, wA.checkpointFor(ctx, wA.streamEvents), "group A keeps its own progress")
	assert.Equal(t, id2, wB.checkpointFor(ctx, wB.streamEvents), "group B keeps its own progress")
	assert.Equal(t, id1, wA.lastIDs[wA.streamEvents])
	assert.Equal(t, id2, wB.lastIDs[wB.streamEvents])

	// 旧无分组键永不写入。
	assert.False(t, mr.Exists("analytics:checkpoint:"+wA.streamEvents), "legacy key must never be written")
	// 新键形态含组名。
	assert.True(t, mr.Exists("analytics:checkpoint:"+wA.group+":"+wA.streamEvents))
	assert.True(t, mr.Exists("analytics:checkpoint:"+wB.group+":"+wB.streamEvents))

	// 重启：各自实例只采纳自己的地板；且不回拨已有组的游标。
	wA2 := testWorkerOn(t, mr)
	wA2.ensureGroups(ctx)
	wB2 := testWorkerOn(t, mr)
	wB2.group = "analytics-worker-b"
	wB2.ensureGroups(ctx)
	assert.Equal(t, id1, wA2.lastIDs[wA2.streamEvents], "restart A adopts its own floor")
	assert.Equal(t, id2, wB2.lastIDs[wB2.streamEvents], "restart B adopts its own floor")

	// A2 的地板挡住向 id1 之后的旧值回写不会发生：推进到更旧 id1 不落库。
	wB2.advanceCheckpoint(ctx, wB2.streamEvents, id1)
	assert.Equal(t, id2, wB2.checkpointFor(ctx, wB2.streamEvents), "floor prevents regression")
}

// 验收（DATA-055 迁移规则）：旧无分组键作为 adoptCheckpoint 的单调地板被
// 只读采纳；组创建只读新键（不为新组继承他组进度）；旧键永不回写。
func TestCheckpointLegacyKeyAdoptedAsMigrationFloor(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	legacy := "analytics:checkpoint:" + w.streamEvents
	require.NoError(t, w.rdb.Set(ctx, legacy, "5-1", 0).Err())

	seedEvent(t, w, "u1")
	// 组不存在 → ensureGroups 只按声明起点（"0"）建组，不继承旧键进度：
	// 种子消息 id（真实毫秒时间戳）远大于旧键值 "5-1"，若组被建在 5-1，
	// ">" 不会投递它。
	w.ensureGroups(ctx)
	delivered, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: w.consumer, Streams: []string{w.streamEvents, ">"},
	}).Result()
	require.NoError(t, err)
	require.Len(t, delivered, 1)
	require.Len(t, delivered[0].Messages, 1, "a fresh group starts at the declared position, not the legacy key")

	// 旧键被采纳为单调地板：推进更旧 id 不落库、不回写旧键。
	w.advanceCheckpoint(ctx, w.streamEvents, "4-1")
	assert.False(t, mr.Exists(w.checkpointKey(w.streamEvents)), "older id below the legacy floor is not persisted")
	assert.Equal(t, "5-1", w.rdb.Get(ctx, legacy).Val(), "legacy key is read-only")

	// 越过地板后写新键；旧键原样保留。
	w.advanceCheckpoint(ctx, w.streamEvents, "6-1")
	assert.Equal(t, "6-1", w.checkpointFor(ctx, w.streamEvents))
	assert.Equal(t, "5-1", w.rdb.Get(ctx, legacy).Val())
}
