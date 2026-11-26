package worker

import (
	"encoding/json"
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

// DATA-018：金额精确解析——json.Number 逐位 ParseUint（无 float64 往返），
// 小数/负数/零/超范围拒绝进死信，遗留 float64 必须是正整数且在值域内。
func TestAmountCentsUint64_Exact(t *testing.T) {
	cases := []struct {
		name    string
		in      map[string]any
		want    uint64
		wantErr bool
	}{
		{name: "json number exact", in: map[string]any{"amount_cents": json.Number("123")}, want: 123},
		{name: "big integer beyond float64", in: map[string]any{"amount_cents": json.Number("9007199254740993")}, want: 9007199254740993},
		{name: "fractional json number", in: map[string]any{"amount_cents": json.Number("10.5")}, wantErr: true},
		{name: "negative json number", in: map[string]any{"amount_cents": json.Number("-5")}, wantErr: true},
		{name: "zero json number", in: map[string]any{"amount_cents": json.Number("0")}, wantErr: true},
		{name: "overflow json number", in: map[string]any{"amount_cents": json.Number("99999999999999999999")}, wantErr: true},
		{name: "legacy integral float", in: map[string]any{"amount_cents": float64(7)}, want: 7},
		{name: "legacy fractional float", in: map[string]any{"amount_cents": float64(10.5)}, wantErr: true},
		{name: "legacy negative float", in: map[string]any{"amount_cents": float64(-5)}, wantErr: true},
		{name: "float beyond uint64", in: map[string]any{"amount_cents": float64(2e63)}, wantErr: true},
		{name: "int", in: map[string]any{"amount_cents": 3}, want: 3},
		{name: "string rejected", in: map[string]any{"amount_cents": "100"}, wantErr: true},
		{name: "missing rejected", in: map[string]any{}, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := amountCentsUint64(c.in)
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

// TestInsertPayment_ExactAmountRoundTrip：2^53+1 金额经 UseNumber 解码与
// 精确解析后逐位落列（旧实现经 float64 已失真为 9007199254740992）。
func TestInsertPayment_ExactAmountRoundTrip(t *testing.T) {
	w, _ := testWorker(t)
	payload := map[string]any{
		"ts": "2026-11-26T03:02:55Z", "game_id": "g", "env": "prod",
		"user_id": "u1", "order_id": "o1", "currency": "USD", "status": "success",
		"amount_cents": json.Number("9007199254740993"),
	}
	require.NoError(t, w.insertPayment(t.Context(), payload, "1-1"))
	batch, ok := w.paymentBatch.(*mockBatch)
	require.True(t, ok, "paymentBatch type %T", w.paymentBatch)
	require.Len(t, batch.rows, 1)
	row, ok := batch.rows[0].([]any)
	require.True(t, ok, "row type %T", batch.rows[0])
	// insertPaymentsSQL 第 6 列（下标 5）为 amount_cents。
	assert.Equal(t, uint64(9007199254740993), row[5], "amount must round-trip digit-for-digit")
}
