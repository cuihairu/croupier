package execlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleRec(status Status) *Record {
	return &Record{
		TaskID:        "task-1",
		Operator:      "gm:alice",
		AgentType:     "sidecar",
		AgentInstance: "agent-demo-1",
		Action:        "kick_player",
		Params:        json.RawMessage(`{"playerId":42}`),
		Status:        status,
		DurationMs:    12,
		TsUnixMs:      1700000000000,
		GameID:        "demo",
		Env:           "prod",
	}
}

func TestRecordJSONKeysLowerCamel(t *testing.T) {
	data, err := json.Marshal(sampleRec(StatusSuccess))
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	for _, key := range []string{"taskId", "operator", "agentType", "agentInstance", "action", "status", "durationMs", "tsUnixMs", "gameId", "env"} {
		assert.Contains(t, m, key)
	}
	assert.NotContains(t, m, "task_id")
}

func TestScopeRequired(t *testing.T) {
	r := sampleRec(StatusSuccess)
	assert.True(t, r.ScopeRequired())
	r.Env = ""
	assert.False(t, r.ScopeRequired())
}

func newTestLogger(t *testing.T) (*Logger, string) {
	t.Helper()
	dir := t.TempDir()
	w, err := OpenFileWriter(dir, "exec.log", WriterOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	return New(w), dir
}

func TestFileWriterAppendAndReadBack(t *testing.T) {
	l, dir := newTestLogger(t)
	require.NoError(t, l.Record(sampleRec(StatusSuccess)))
	require.NoError(t, l.Record(sampleRec(StatusFailure)))

	recs, err := ReadRecords(filepath.Join(dir, "exec.log"))
	require.NoError(t, err)
	require.Len(t, recs, 2)
	assert.Equal(t, StatusSuccess, recs[0].Status)
	assert.Equal(t, StatusFailure, recs[1].Status)
	assert.Equal(t, int64(1700000000000), recs[0].TsUnixMs)
}

func TestReadRecordsSkipsBrokenLines(t *testing.T) {
	dir := t.TempDir()
	good, err := json.Marshal(sampleRec(StatusSuccess))
	require.NoError(t, err)
	content := string(good) + "\n{broken json\n\n" + string(good) + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "exec.log"), []byte(content), 0o644))

	recs, err := ReadRecords(filepath.Join(dir, "exec.log"))
	require.NoError(t, err)
	require.Len(t, recs, 2)
}

func TestLoggerRejectsMissingScope(t *testing.T) {
	l, _ := newTestLogger(t)
	rec := sampleRec(StatusSuccess)
	rec.GameID = ""
	assert.Error(t, l.Record(rec))
	rec = sampleRec(StatusSuccess)
	rec.Env = ""
	assert.Error(t, l.Record(rec))
}

func TestLoggerClosedWriterErrors(t *testing.T) {
	l, _ := newTestLogger(t)
	require.NoError(t, l.w.Close())
	assert.Error(t, l.Record(sampleRec(StatusSuccess)))
	var nilL *Logger
	assert.Error(t, nilL.Record(sampleRec(StatusSuccess)))
}

func TestLoggerUploadsAndCountsFailures(t *testing.T) {
	dir := t.TempDir()
	w, err := OpenFileWriter(dir, "exec.log", WriterOptions{})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	var calls, fails atomic.Int64
	l := New(w,
		WithUploader(UploaderFunc(func(ctx context.Context, rec *Record) error {
			calls.Add(1)
			if rec.Status == StatusFailure {
				return errors.New("server down")
			}
			return nil
		})),
		WithUploadErrorHook(func(error) { fails.Add(1) }),
	)

	require.NoError(t, l.Record(sampleRec(StatusSuccess)))
	require.NoError(t, l.Record(sampleRec(StatusFailure)))
	assert.Equal(t, int64(2), calls.Load())
	assert.Equal(t, int64(1), fails.Load())
	assert.Equal(t, int64(1), l.UploadFailures())
}

// 上报失败不阻断执行：本地真值已落盘即成功返回。
func TestLoggerUploadFailureDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	w, err := OpenFileWriter(dir, "exec.log", WriterOptions{})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()
	l := New(w, WithUploader(UploaderFunc(func(ctx context.Context, rec *Record) error {
		return errors.New("no route")
	})), WithUploadTimeout(50*time.Millisecond))
	assert.NoError(t, l.Record(sampleRec(StatusSuccess)))
}

func TestGateAdmitPersistsBeforeReturning(t *testing.T) {
	l, dir := newTestLogger(t)
	gate := l.Gate()

	rec := sampleRec(StatusSuccess)
	rec.Status = "" // 闸覆盖状态位
	require.NoError(t, gate.Admit(rec))

	// 顺序契约：Admit 返回时记录必须已持久化可读（K3 审计先落盘后放行用例）。
	recs, err := ReadRecords(filepath.Join(dir, "exec.log"))
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, StatusAdmitted, recs[0].Status)
	assert.Equal(t, "kick_player", recs[0].Action)
	assert.Empty(t, recs[0].Summary)
}

func TestGateDeniesWhenNotPersisted(t *testing.T) {
	l, _ := newTestLogger(t)
	gate := l.Gate()
	require.NoError(t, l.w.Close()) // 模拟盘故障

	err := gate.Admit(sampleRec(StatusSuccess))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execlog:")
}

func TestGateNilSafe(t *testing.T) {
	var gate *Gate
	assert.Error(t, gate.Admit(sampleRec(StatusSuccess)))
}

func TestOpenFileWriterValidation(t *testing.T) {
	_, err := OpenFileWriter("", "exec.log", WriterOptions{})
	assert.Error(t, err)
	_, err = OpenFileWriter(t.TempDir(), "", WriterOptions{})
	assert.Error(t, err)

	// 目录自动创建 + 默认档不 panic
	w, err := OpenFileWriter(filepath.Join(t.TempDir(), "audit"), "exec.log", WriterOptions{})
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, w.Close()) // 幂等
}

func TestExportJSONLAndCSVWithRange(t *testing.T) {
	dir, dest := t.TempDir(), t.TempDir()
	w, err := OpenFileWriter(dir, "exec.log", WriterOptions{})
	require.NoError(t, err)

	early := sampleRec(StatusSuccess)
	early.TsUnixMs = 1000
	mid := sampleRec(StatusSuccess)
	mid.TsUnixMs = 5000
	late := sampleRec(StatusFailure)
	late.TsUnixMs = 9000
	for _, r := range []*Record{early, mid, late} {
		require.NoError(t, w.Append(r))
	}
	require.NoError(t, w.Close())

	// 伪造一个轮转文件（lumberjack 命名形态）验证 glob 并集
	rotated := filepath.Join(dir, "exec-2026-10-09T00-00-00.000.log")
	old := sampleRec(StatusSuccess)
	old.TsUnixMs = 500
	require.NoError(t, os.WriteFile(rotated, []byte(mustJSON(old)+"\n"), 0o644))

	// 时间窗为闭区间：[1000, 9000] 命中 1000/5000/9000 三条，排除轮转文件的 500。
	n, err := Export(ExportOptions{SourceDir: dir, Filename: "exec.log", DestDir: dest, SinceUnixMs: 1000, UntilUnixMs: 9000})
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	files, _ := filepath.Glob(filepath.Join(dest, "execlog-export-*.jsonl"))
	require.Len(t, files, 1)
	recs, err := ReadRecords(files[0])
	require.NoError(t, err)
	require.Len(t, recs, 3)

	n, err = Export(ExportOptions{SourceDir: dir, Filename: "exec.log", DestDir: dest, Format: ExportCSV})
	require.NoError(t, err)
	assert.Equal(t, 4, n, "no range = full export incl. rotated file")
	csvs, _ := filepath.Glob(filepath.Join(dest, "execlog-export-*.csv"))
	require.Len(t, csvs, 1)
	data, err := os.ReadFile(csvs[0])
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 5, "header + 4 records")
	assert.True(t, strings.HasPrefix(lines[0], "taskId,operator"))
}

func TestExportValidation(t *testing.T) {
	_, err := Export(ExportOptions{DestDir: t.TempDir()})
	assert.Error(t, err)
	_, err = Export(ExportOptions{SourceDir: t.TempDir(), Filename: "x.log", DestDir: t.TempDir(), Format: "xlsx"})
	assert.Error(t, err)
}

func mustJSON(r *Record) string {
	b, err := json.Marshal(r)
	if err != nil {
		panic(fmt.Sprintf("marshal: %v", err))
	}
	return string(b)
}
