package journal

// journal.go 补测（续 journal_extra_test.go 清单之后）：Append 的 json
// 编码失败臂（After 子片段为非法 JSON——Validate 只查存在性不查合法
// 性，Marshal 才失败）、Replay 的前序行读取错误臂（外部覆写范围内第一
// 行→Read(1,1) 解码失败，与既有外部篡改注入同法）、Replay 的行校验失败
// 臂（ErrInvalidReplayRow：绕过 Append/recover 双重校验直构日志注入）、
// Replay 事务边界刷组失败臂（无元数据 End 组 + 换事务 → 边界 flush 命中
// sink 错误）。
//
// 诚实跳过：OpenWithLimit 的 seek end（80-82）、recover 的 seek start /
// partial-tail truncate / read（91-92、100-101、106-107）、Append 的短写
// 子臂与 sync（173-174、179-181）、Read 的 scanner.Err（261-262）——常规
// 文件上偏移恒非负、写/同步无 I/O 故障注入缝；flush 空组守卫（323-324）
// 与收尾 last<to 切界守卫（371-372）为防御性死码——flush 仅在组非空时
// 被调、Read 已保证范围完整或报错；事务边界刷组错误臂（350-352）不可达
// ——End 事件入组即刷（361-364）、无元数据非 End 事件在 343 被拒，组抵达
// 边界时若前事件无元数据则已被刷出、若有元数据且未闭合则 347 先报
// ErrIncompleteTx，flush 在边界处恒不可达。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendEncodeFailure(t *testing.T) {
	j, err := Open(filepath.Join(t.TempDir(), "enc.jsonl"))
	require.NoError(t, err)
	defer j.Close()

	// After 子片段为非法 JSON：Validate 通过、json.Marshal 失败
	e := event("", 0, false)
	e.After = map[string]json.RawMessage{"id": json.RawMessage(`{bad`)}
	_, _, err = j.Append(e)
	require.ErrorContains(t, err, "journal: encode")
}

func TestReplayReadPreviousError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prev.jsonl")
	j, err := Open(path)
	require.NoError(t, err)
	defer j.Close()
	_, _, err = j.Append(event("tx-a", 1, true))
	require.NoError(t, err)
	_, _, err = j.Append(event("tx-b", 1, true))
	require.NoError(t, err)

	// 外部以等长垃圾覆写第一行（保持第二行边界完整）→ Replay 的
	// Read(from-1) 解码失败透传
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	row1Len := int64(bytes.IndexByte(raw, '\n') + 1)
	ext, err := os.OpenFile(path, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = ext.WriteAt(bytes.Repeat([]byte("X"), int(row1Len-1)), 0)
	require.NoError(t, err)
	require.NoError(t, ext.Close())

	_, err = j.Replay(context.Background(), 2, 2, &recordingSink{})
	require.ErrorContains(t, err, "decode sequence 1")
}

func TestReplayInvalidRowGuard(t *testing.T) {
	// 绕过 Append/recover 双重校验直构日志：Read 解码成功、Replay 行校验
	// 失败（ErrInvalidReplayRow）
	path := filepath.Join(t.TempDir(), "badrow.jsonl")
	bad := event("tx-a", 1, true)
	bad.SourceID = "" // 任何公有入口写入都会在此被拒
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	require.NoError(t, err)
	_, err = f.Write(mustMarshalRow(t, record{Sequence: 1, ID: "e1", Event: bad}))
	require.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err)

	j := &Journal{path: path, file: f, byID: map[string]record{}, next: 1, maxBytes: 0}
	defer j.Close()
	_, err = j.Replay(context.Background(), 1, 1, &recordingSink{})
	require.ErrorIs(t, err, ErrInvalidReplayRow)
}

func TestReplayBoundaryFlushError(t *testing.T) {
	j, err := Open(filepath.Join(t.TempDir(), "bflush.jsonl"))
	require.NoError(t, err)
	defer j.Close()
	// 无元数据独立组（End）后换事务 → 边界刷组命中 sink 错误
	_, _, err = j.Append(event("", 0, true))
	require.NoError(t, err)
	_, _, err = j.Append(event("tx-b", 1, false))
	require.NoError(t, err)

	_, err = j.Replay(context.Background(), 1, 2,
		&recordingSink{err: errors.New("apply failed")})
	require.ErrorContains(t, err, "apply failed")
}
