package worker

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DATA-017：稳定事件标识。生产方未携带 event_id 时，worker 用载荷指纹派生
// 确定性 UUIDv5（此前是随机 UUID——重投/重提交在 ClickHouse 自然键上无法收敛）。

// rowEventID 提取刚 append 行的 event_id 列（insertEventsSQL 第 11 列，下标 10）。
func rowEventID(t *testing.T, w *Worker, rowIdx int) string {
	t.Helper()
	batch, ok := w.eventBatch.(*mockBatch)
	require.True(t, ok, "eventBatch type %T, want *mockBatch", w.eventBatch)
	require.Greater(t, len(batch.rows), rowIdx)
	row, ok := batch.rows[rowIdx].([]any)
	require.True(t, ok, "row type %T, want []any", batch.rows[rowIdx])
	id, ok := row[10].(string)
	require.True(t, ok, "event_id column type %T, want string", row[10])
	return id
}

func sampleEventPayload() map[string]any {
	return map[string]any{
		"game_id": "g", "env": "prod", "ts": "2026-11-26T03:02:55Z",
		"event": "login", "user_id": "u1", "session_id": "s1",
		"props": map[string]any{"b": 2, "a": "x"},
	}
}

// TestDeriveEventID_DeterministicAcrossRestarts：同一载荷（重新解析/重新提交/
// 进程重启后重放）必须得到同一 id——纯函数、无内存态。
func TestDeriveEventID_DeterministicAcrossRestarts(t *testing.T) {
	first := deriveEventID(sampleEventPayload())
	assert.True(t, isUUID(first))
	for i := 0; i < 3; i++ {
		again := deriveEventID(sampleEventPayload())
		assert.Equal(t, first, again, "identical payload must derive identical id")
	}

	// 内容任一字段不同 -> 不同 id（不同事件不误合并）。
	other := sampleEventPayload()
	other["user_id"] = "u2"
	assert.NotEqual(t, first, deriveEventID(other))

	other = sampleEventPayload()
	other["props"] = map[string]any{"b": 2, "a": "y"}
	assert.NotEqual(t, first, deriveEventID(other))

	// props 键序不同但内容相同 -> 同一 id（canonical JSON 排序键）。
	reordered := sampleEventPayload()
	reordered["props"] = map[string]any{"a": "x", "b": 2}
	assert.Equal(t, first, deriveEventID(reordered))
}

// TestDeriveEventID_FoldsClientID：非 UUID 的客户端 event_id 参与指纹且稳定
// （旧实现对该形态生成随机 id，去重失效）。
func TestDeriveEventID_FoldsClientID(t *testing.T) {
	m := sampleEventPayload()
	m["event_id"] = "producer-abc-1"
	a := deriveEventID(m)
	assert.True(t, isUUID(a))
	assert.Equal(t, a, deriveEventID(m))

	m["event_id"] = "producer-abc-2"
	assert.NotEqual(t, a, deriveEventID(m))
}

// TestInsertEvent_KeepsClientUUID：客户端给合法 UUID 时原样落列。
func TestInsertEvent_KeepsClientUUID(t *testing.T) {
	w, _ := testWorker(t)
	clientID := "11111111-2222-3333-4444-555555555555"
	m := sampleEventPayload()
	m["event_id"] = clientID
	require.NoError(t, w.insertEvent(t.Context(), m, "1-1"))
	assert.Equal(t, clientID, rowEventID(t, w, 0))
}

// TestInsertEvent_MissingIDDeterministicAcrossRedelivery：无 event_id 的载荷
// 经两个独立 worker 实例（模拟重启）各插入一次，行 event_id 一致——
// Send 后 ACK 前崩溃重投场景的收敛依据。
func TestInsertEvent_MissingIDDeterministicAcrossRedelivery(t *testing.T) {
	payload := sampleEventPayload()

	w1, _ := testWorker(t)
	require.NoError(t, w1.insertEvent(t.Context(), payload, "1-1"))
	id1 := rowEventID(t, w1, 0)

	w2, _ := testWorker(t)
	require.NoError(t, w2.insertEvent(t.Context(), payload, "1-1"))
	id2 := rowEventID(t, w2, 0)

	assert.Equal(t, id1, id2, "redelivered identical payload must map to one natural key")
	assert.True(t, isUUID(id1))
}

// isUUID 判定合法 UUID（worker 落列约束）。
func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
