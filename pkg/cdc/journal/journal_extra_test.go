package journal

// journal / capture 补测：Open 参数守卫与目录/文件打开失败臂、recover
// 的乱序行/坏事件/重复 ID（冲突与同容）三类损坏、Append 的关闭态/
// 故障锁定/容量上限/写失败（只读句柄替换注入 EBADF）、Close 幂等与
// Sync 失败、Read 的关闭态/范围归一/越界空结果/seek 失败与外部篡改
// （坏行解码、乱序行、截断致范围不完整）、Replay 的 nil sink/关闭态/
// 空日志/from-to 归一/完整事务刷组失败/ctx 取消/组中换事务未闭合/
// 组内序号断裂/组首序号非 1，以及 Capture/CaptureEvent 的参数守卫、
// selection 校验、Poll 失败、ctx 取消、消费失败与确认失败臂。
//
// 诚实跳过：常规文件上 Seek/Truncate/Read 的错误臂与 partial-tail
// Truncate 失败臂（无注入缝）；ErrShortWrite 子臂（*os.File 短写不可
// 注入）；Append 与 recover 的 json 编码错误臂（全字段可序列化）；
// Read 的 scanner.Err 臂；Replay 的行校验臂（Append 与 recover 双重
// 校验后日志内行恒过 Validate）、flush 空组守卫、事务边界 flush 臂
// （End 事件追加后立即刷组，边界到达时组必为未闭合态→错误臂）与
// 收尾 last<to 切界守卫（Read 已保证范围完整或报错，经公有入口不可
// 达）。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/filter"
	"github.com/stretchr/testify/require"
)

func mustMarshalRow(t *testing.T, row record) []byte {
	t.Helper()
	data, err := json.Marshal(row)
	require.NoError(t, err)
	return append(data, '\n')
}

func TestOpenGuardsAndDirectoryFailures(t *testing.T) {
	_, err := Open("")
	require.ErrorContains(t, err, "path is required")

	_, err = OpenWithLimit(filepath.Join(t.TempDir(), "j.jsonl"), -1)
	require.ErrorContains(t, err, "negative")

	// 父路径被文件占位 → 目录创建失败
	occupied := filepath.Join(t.TempDir(), "occupied")
	require.NoError(t, os.WriteFile(occupied, []byte("x"), 0o600))
	_, err = Open(filepath.Join(occupied, "sub", "j.jsonl"))
	require.ErrorContains(t, err, "create directory")

	// 路径是目录 → 打开失败
	dir := filepath.Join(t.TempDir(), "as-dir")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	_, err = Open(dir)
	require.ErrorContains(t, err, "journal: open")
}

func TestRecoverRejectsInvalidAndDuplicateRecords(t *testing.T) {
	valid := event("", 0, false)
	valid.ID = "e1"
	row1 := record{Sequence: 1, ID: "e1", Event: valid}

	// 乱序行（sequence 非 next+1）
	bad := valid
	bad.ID = "e9"
	path := filepath.Join(t.TempDir(), "seq.jsonl")
	require.NoError(t, os.WriteFile(path, mustMarshalRow(t, record{Sequence: 5, ID: "e9", Event: bad}), 0o600))
	_, err := Open(path)
	require.ErrorContains(t, err, "invalid record")

	// 空 ID
	path2 := filepath.Join(t.TempDir(), "empty-id.jsonl")
	noID := valid
	noID.ID = ""
	require.NoError(t, os.WriteFile(path2, mustMarshalRow(t, record{Sequence: 1, ID: "", Event: noID}), 0o600))
	_, err = Open(path2)
	require.ErrorContains(t, err, "invalid record")

	// 行 ID 与事件 ID 不一致
	path3 := filepath.Join(t.TempDir(), "mismatch.jsonl")
	mis := valid
	mis.ID = "other"
	require.NoError(t, os.WriteFile(path3, mustMarshalRow(t, record{Sequence: 1, ID: "e1", Event: mis}), 0o600))
	_, err = Open(path3)
	require.ErrorContains(t, err, "invalid record")

	// 事件契约校验失败
	path4 := filepath.Join(t.TempDir(), "bad-event.jsonl")
	invalid := valid
	invalid.SourceID = ""
	require.NoError(t, os.WriteFile(path4, mustMarshalRow(t, record{Sequence: 1, ID: "e1", Event: invalid}), 0o600))
	_, err = Open(path4)
	require.ErrorContains(t, err, "invalid event")

	// 同 ID 不同内容（冲突）
	path5 := filepath.Join(t.TempDir(), "conflict.jsonl")
	diff := valid
	diff.Table = "other"
	var buf bytes.Buffer
	buf.Write(mustMarshalRow(t, row1))
	buf.Write(mustMarshalRow(t, record{Sequence: 2, ID: "e1", Event: diff}))
	require.NoError(t, os.WriteFile(path5, buf.Bytes(), 0o600))
	_, err = Open(path5)
	require.ErrorContains(t, err, "conflicting duplicate")

	// 同 ID 同内容（重复）
	path6 := filepath.Join(t.TempDir(), "dupe.jsonl")
	var buf2 bytes.Buffer
	buf2.Write(mustMarshalRow(t, row1))
	buf2.Write(mustMarshalRow(t, record{Sequence: 2, ID: "e1", Event: valid}))
	require.NoError(t, os.WriteFile(path6, buf2.Bytes(), 0o600))
	_, err = Open(path6)
	require.ErrorContains(t, err, "duplicate ID")
}

func TestAppendClosedFaultCapacityAndWriteFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "j.jsonl")
	j, err := Open(path)
	require.NoError(t, err)

	// 关闭态
	require.NoError(t, j.Close())
	_, _, err = j.Append(event("", 0, false))
	require.ErrorIs(t, err, os.ErrClosed)

	// 故障锁定：先前写失败后拒绝追加
	j2, err := Open(filepath.Join(t.TempDir(), "fault.jsonl"))
	require.NoError(t, err)
	defer j2.Close()
	j2.fault = errors.New("disk died")
	_, _, err = j2.Append(event("", 0, false))
	require.ErrorContains(t, err, "unavailable after prior write failure")
	j2.fault = nil

	// 容量上限
	j3, err := OpenWithLimit(filepath.Join(t.TempDir(), "cap.jsonl"), 10)
	require.NoError(t, err)
	defer j3.Close()
	_, _, err = j3.Append(event("", 0, false))
	require.ErrorIs(t, err, ErrCapacityExceeded)

	// 写失败：句柄换成只读打开 → Seek 成功、Write EBADF → 故障锁定
	j4, err := Open(filepath.Join(t.TempDir(), "ro.jsonl"))
	require.NoError(t, err)
	defer j4.Close()
	ro, err := os.Open(path)
	require.NoError(t, err)
	j4.file = ro
	_, _, err = j4.Append(event("", 0, false))
	require.ErrorContains(t, err, "journal: append")
	require.Error(t, j4.fault)
	_, _, err = j4.Append(event("", 0, false))
	require.ErrorContains(t, err, "unavailable after prior write failure")
	require.NoError(t, ro.Close())

	// 追加前置 Seek 失败：句柄已关闭 → seek end 报错并锁定
	j5, err := Open(filepath.Join(t.TempDir(), "seek.jsonl"))
	require.NoError(t, err)
	defer j5.Close()
	closedHandle, err := os.Open(path)
	require.NoError(t, err)
	require.NoError(t, closedHandle.Close())
	j5.file = closedHandle
	_, _, err = j5.Append(event("", 0, false))
	require.ErrorContains(t, err, "seek end before append")
}

func TestCloseIdempotentAndSyncFailure(t *testing.T) {
	dir := t.TempDir()
	j, err := Open(filepath.Join(dir, "j.jsonl"))
	require.NoError(t, err)
	require.NoError(t, j.Close())
	require.NoError(t, j.Close()) // 幂等

	// Sync 失败：句柄已关闭 → Close 返回错误但仍标记关闭
	j2, err := Open(filepath.Join(dir, "j2.jsonl"))
	require.NoError(t, err)
	closed, err := os.Open(filepath.Join(dir, "j.jsonl"))
	require.NoError(t, err)
	require.NoError(t, closed.Close())
	j2.file = closed
	require.Error(t, j2.Close())
	require.NoError(t, j2.Close()) // 已标记关闭，幂等兜底
}

func TestReadGuardsAndRangeNormalization(t *testing.T) {
	dir := t.TempDir()
	j, err := Open(filepath.Join(dir, "j.jsonl"))
	require.NoError(t, err)
	defer j.Close()
	for i := 0; i < 2; i++ {
		e := event("", 0, false)
		e.TsUnixMs = int64(i + 1)
		_, _, err = j.Append(e)
		require.NoError(t, err)
	}

	// 范围归一：from=0 → 1，to=0 → next
	rows, err := j.Read(0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// from 越过末端 → 空结果
	rows, err = j.Read(3, 0)
	require.NoError(t, err)
	require.Empty(t, rows)

	// 逆序范围 → 拒绝
	_, err = j.Read(3, 1)
	require.ErrorContains(t, err, "invalid sequence range")

	// 关闭态
	require.NoError(t, j.Close())
	_, err = j.Read(1, 0)
	require.ErrorIs(t, err, os.ErrClosed)

	// seek 失败：句柄关闭但未标记关闭态（空日志在范围检查早退，触不到
	// Seek，须先有一条事件）
	j2, err := Open(filepath.Join(dir, "j2.jsonl"))
	require.NoError(t, err)
	defer j2.Close()
	_, _, err = j2.Append(event("", 0, false))
	require.NoError(t, err)
	closed, err := os.Open(filepath.Join(dir, "j.jsonl"))
	require.NoError(t, err)
	require.NoError(t, closed.Close())
	j2.file = closed
	_, err = j2.Read(1, 0)
	require.Error(t, err)
}

func TestReadDetectsExternalCorruption(t *testing.T) {
	// 两事件 + 外部覆写范围内第二行为坏行 → 解码失败（追加的坏行总落
	// 在 to 归一之外，读不到）
	path := filepath.Join(t.TempDir(), "junk.jsonl")
	j, err := Open(path)
	require.NoError(t, err)
	defer j.Close()
	for i := 0; i < 2; i++ {
		e := event("", 0, false)
		e.TsUnixMs = int64(i + 1)
		_, _, err = j.Append(e)
		require.NoError(t, err)
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	line2 := int64(bytes.IndexByte(raw, '\n') + 1)
	ext, err := os.OpenFile(path, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = ext.WriteAt([]byte("junk\n"), line2)
	require.NoError(t, err)
	require.NoError(t, ext.Close())
	_, err = j.Read(1, 0)
	require.ErrorContains(t, err, "decode sequence")

	// 外部覆写范围内第二行为乱序行（sequence 漂移）
	path2 := filepath.Join(t.TempDir(), "drift.jsonl")
	j2, err := Open(path2)
	require.NoError(t, err)
	defer j2.Close()
	for i := 0; i < 2; i++ {
		e := event("", 0, false)
		e.TsUnixMs = int64(i + 1)
		_, _, err = j2.Append(e)
		require.NoError(t, err)
	}
	drift := event("", 0, false)
	drift.ID = "drift"
	drift.TsUnixMs = 9
	rowBytes := mustMarshalRow(t, record{Sequence: 9, ID: "drift", Event: drift})
	rawDrift, err := os.ReadFile(path2)
	require.NoError(t, err)
	offset2 := int64(bytes.IndexByte(rawDrift, '\n') + 1)
	ext, err = os.OpenFile(path2, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = ext.WriteAt(rowBytes, offset2)
	require.NoError(t, err)
	require.NoError(t, ext.Close())
	_, err = j2.Read(1, 0)
	require.ErrorContains(t, err, "unexpected sequence")

	// 外部截断 → 记账序列仍在但行数缺失 → 范围不完整（两事件内容须
	// 有差异，裸 event() 全同会被 Append 幂等去重）
	path3 := filepath.Join(t.TempDir(), "cut.jsonl")
	j3, err := Open(path3)
	require.NoError(t, err)
	defer j3.Close()
	for i := 0; i < 2; i++ {
		e := event("", 0, false)
		e.TsUnixMs = int64(i + 1)
		_, _, err = j3.Append(e)
		require.NoError(t, err)
	}
	rawCut, err := os.ReadFile(path3)
	require.NoError(t, err)
	cut := bytes.IndexByte(rawCut, '\n') + 1
	require.NoError(t, os.Truncate(path3, int64(cut)))
	_, err = j3.Read(1, 0)
	require.ErrorContains(t, err, "is incomplete")
}

func TestReplayGuardsAndRangeNormalization(t *testing.T) {
	j, err := Open(filepath.Join(t.TempDir(), "j.jsonl"))
	require.NoError(t, err)
	defer j.Close()
	_, _, err = j.Append(event("tx-a", 1, true))
	require.NoError(t, err)

	// nil sink
	_, err = j.Replay(context.Background(), 1, 1, nil)
	require.ErrorContains(t, err, "sink is required")

	// 空日志 → 无事可放
	empty, err := Open(filepath.Join(t.TempDir(), "empty.jsonl"))
	require.NoError(t, err)
	defer empty.Close()
	last, err := empty.Replay(context.Background(), 0, 0, &recordingSink{})
	require.NoError(t, err)
	require.Zero(t, last)

	// from=0 / to=0 归一到实际范围，完整事务放行
	sink := &recordingSink{}
	last, err = j.Replay(context.Background(), 0, 0, sink)
	require.NoError(t, err)
	require.Equal(t, uint64(1), last)
	require.Equal(t, [][]string{{"tx-a"}}, sink.groups)

	// 关闭态 → Read 失败透传
	require.NoError(t, j.Close())
	_, err = j.Replay(context.Background(), 1, 1, &recordingSink{})
	require.ErrorIs(t, err, os.ErrClosed)

	// ctx 已取消 → 循环首步退出
	j2, err := Open(filepath.Join(t.TempDir(), "j2.jsonl"))
	require.NoError(t, err)
	defer j2.Close()
	_, _, err = j2.Append(event("tx-a", 1, true))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	last, err = j2.Replay(ctx, 1, 1, &recordingSink{})
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, last)
}

func TestReplayRejectsBrokenTransactionGroups(t *testing.T) {
	ctx := context.Background()

	// 完整事务 + sink 失败 → 刷组失败，last 不前进
	j, err := Open(filepath.Join(t.TempDir(), "sink-fail.jsonl"))
	require.NoError(t, err)
	defer j.Close()
	_, _, err = j.Append(event("tx-a", 1, true))
	require.NoError(t, err)
	last, err := j.Replay(ctx, 1, 1, &recordingSink{err: errors.New("apply failed")})
	require.ErrorContains(t, err, "apply failed")
	require.Zero(t, last)

	// 组中换事务：前一事务未闭合
	j2, err := Open(filepath.Join(t.TempDir(), "switch.jsonl"))
	require.NoError(t, err)
	defer j2.Close()
	for _, e := range []cdc.ChangeEvent{event("tx-a", 1, false), event("tx-b", 1, false)} {
		_, _, err = j2.Append(e)
		require.NoError(t, err)
	}
	_, err = j2.Replay(ctx, 1, 2, &recordingSink{})
	require.ErrorIs(t, err, ErrIncompleteTx)

	// 组内序号断裂：同事务 seq 1 → 3
	j3, err := Open(filepath.Join(t.TempDir(), "gap.jsonl"))
	require.NoError(t, err)
	defer j3.Close()
	for _, e := range []cdc.ChangeEvent{event("tx-a", 1, false), event("tx-a", 3, false)} {
		_, _, err = j3.Append(e)
		require.NoError(t, err)
	}
	_, err = j3.Replay(ctx, 1, 2, &recordingSink{})
	require.ErrorIs(t, err, ErrIncompleteTx)

	// 组首序号非 1
	j4, err := Open(filepath.Join(t.TempDir(), "offset.jsonl"))
	require.NoError(t, err)
	defer j4.Close()
	_, _, err = j4.Append(event("tx-a", 2, true))
	require.NoError(t, err)
	_, err = j4.Replay(ctx, 1, 1, &recordingSink{})
	require.ErrorIs(t, err, ErrIncompleteTx)

	// 前序事件无事务元数据：sameTransaction 空事务 ID 臂（无元数据但
	// 带 End 的事件放行 → 独立成组，不视为切界）
	j5, err := Open(filepath.Join(t.TempDir(), "metaless.jsonl"))
	require.NoError(t, err)
	defer j5.Close()
	_, _, err = j5.Append(event("tx-a", 1, true))
	require.NoError(t, err)
	_, _, err = j5.Append(event("", 0, true))
	require.NoError(t, err)
	sinkMetaless := &recordingSink{}
	last, err = j5.Replay(ctx, 2, 2, sinkMetaless)
	require.NoError(t, err)
	require.Equal(t, uint64(2), last)
}

func TestCaptureGuardsAndErrorArms(t *testing.T) {
	ctx := context.Background()
	j, err := Open(filepath.Join(t.TempDir(), "j.jsonl"))
	require.NoError(t, err)
	defer j.Close()
	selection := filter.Filter{Include: []filter.Selector{{}}}

	// 参数守卫
	_, err = Capture(ctx, nil, j, selection, cdc.PollRequest{})
	require.ErrorContains(t, err, "source and journal are required")
	_, err = Capture(ctx, &fakeSource{}, nil, selection, cdc.PollRequest{})
	require.ErrorContains(t, err, "source and journal are required")

	// selection 校验失败
	badSelection := filter.Filter{Include: []filter.Selector{{
		Operations: []cdc.Operation{cdc.Operation("weird")},
	}}}
	_, err = Capture(ctx, &fakeSource{}, j, badSelection, cdc.PollRequest{})
	require.ErrorContains(t, err, "invalid operation")

	// Poll 失败
	_, err = Capture(ctx, &errSource{err: errors.New("source down")}, j, selection, cdc.PollRequest{})
	require.ErrorContains(t, err, "poll source")

	// ctx 取消（事件通道永不关闭；预先取消的 ctx 让 select 立即走 Done）
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Capture(cancelledCtx, &stuckSource{}, j, selection, cdc.PollRequest{})
	require.ErrorIs(t, err, context.Canceled)

	// CaptureEvent：source 缺失 / selection 校验 / 位点 kind 闭集外
	_, err = CaptureEvent(ctx, nil, j, selection, event("", 0, false), nil)
	require.ErrorContains(t, err, "source is required")
	_, err = CaptureEvent(ctx, &fakeSource{}, j, badSelection, event("", 0, false), nil)
	require.ErrorContains(t, err, "invalid operation")
	bad := event("", 0, false)
	bad.Position = cdc.Position{Kind: "weird"}
	_, err = CaptureEvent(ctx, &fakeSource{}, nil, selection, bad, nil)
	require.ErrorContains(t, err, "invalid position kind")

	// 消费失败
	_, err = CaptureEvent(ctx, &fakeSource{}, nil, selection, event("", 0, false),
		func(cdc.ChangeEvent) error { return errors.New("consumer down") })
	require.ErrorContains(t, err, "consume source event")

	// 确认失败
	_, err = CaptureEvent(ctx, &ackErrSource{}, nil, selection, event("", 0, false), nil)
	require.ErrorContains(t, err, "acknowledge source position")
}

// errSource 的 Poll 恒失败。
type errSource struct {
	cdc.SourceConnector
	err error
}

func (s *errSource) Poll(context.Context, cdc.PollRequest) (<-chan cdc.ChangeEvent, error) {
	return nil, s.err
}

// stuckSource 的事件通道永不关闭（ctx 取消臂专用）。
type stuckSource struct {
	cdc.SourceConnector
}

func (s *stuckSource) Poll(context.Context, cdc.PollRequest) (<-chan cdc.ChangeEvent, error) {
	out := make(chan cdc.ChangeEvent)
	return out, nil
}

// ackErrSource 的确认恒失败。
type ackErrSource struct {
	cdc.SourceConnector
}

func (s *ackErrSource) Poll(context.Context, cdc.PollRequest) (<-chan cdc.ChangeEvent, error) {
	out := make(chan cdc.ChangeEvent, 1)
	out <- event("", 0, false)
	close(out)
	return out, nil
}

func (s *ackErrSource) AckCheckpoint(context.Context, cdc.Position) error {
	return errors.New("ack refused")
}
