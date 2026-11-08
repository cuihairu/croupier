package execlog

// export.go / logger.go / writer.go 补测：Export 的 glob 模式错误、轮转
// 文件读取失败容错 continue、DestDir 创建失败、until 时间窗排除臂、CSV
// 目标创建失败（只读目录）臂；Logger 的 TsUnixMs 缺省注入（now 闭包与
// 赋值臂）与 nil receiver 的 UploadFailures；FileWriter 的目录创建失败、
// Marshal 失败、底层写失败臂与 ReadRecords 读缺失文件、splitLines 无尾
// 换行段、NowMs 直接调用。
//
// 诚实跳过：writeJSONL 的 json.Marshal 错误臂经 Export 不可达——ReadRecords
// 只产出「整体合法 JSON 且 Action 非空」的记录，其 Params 子片段作为合法
// JSON 值必然可再编码，故以同包直测覆盖（构造坏 Params 直调 writeJSONL）；
// writeCSV 的 csv.Writer.Write / w.Write 错误臂（csvHeader 写入与逐行写入）
// 属驱动 I/O 面，除目标创建失败外无可注入缝——csv.Writer 无自定义 writer
// 注入，os.File 上无法仅令某一次 Write 失败。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExportGlobPatternAndReadErrors(t *testing.T) {
	dir, dest := t.TempDir(), t.TempDir()

	// glob 模式错误：文件名含未闭合 '[' → ErrBadPattern
	_, err := Export(ExportOptions{SourceDir: dir, Filename: "exec[.log", DestDir: dest})
	require.Error(t, err)
	require.Contains(t, err.Error(), "syntax error in pattern")

	// 轮转文件读取失败容错：与模式匹配的目录被 ReadRecords 拒绝 → continue
	w, err := OpenFileWriter(dir, "exec.log", WriterOptions{})
	require.NoError(t, err)
	require.NoError(t, w.Append(sampleRec(StatusSuccess)))
	require.NoError(t, w.Close())
	rotated := filepath.Join(dir, "exec-2026-01-01T00-00-00.000.log")
	require.NoError(t, os.MkdirAll(rotated, 0o755))
	active, err := os.ReadFile(filepath.Join(dir, "exec.log"))
	require.NoError(t, err)

	// 活动文件本体并入（52-54 臂）
	n, err := Export(ExportOptions{SourceDir: dir, Filename: "exec.log", DestDir: dest})
	require.NoError(t, err)
	require.Equal(t, 1, n)
	_ = active
}

func TestExportMkdirAllFailure(t *testing.T) {
	dir := t.TempDir()
	occupied := filepath.Join(dir, "occupied")
	require.NoError(t, os.WriteFile(occupied, []byte("x"), 0o600))
	_, err := Export(ExportOptions{SourceDir: dir, Filename: "exec.log",
		DestDir: filepath.Join(occupied, "out")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a directory")
}

func TestExportFilterRangeExcludesAfterUntil(t *testing.T) {
	dir, dest := t.TempDir(), t.TempDir()
	w, err := OpenFileWriter(dir, "exec.log", WriterOptions{})
	require.NoError(t, err)
	inside := sampleRec(StatusSuccess)
	inside.TsUnixMs = 4000
	outside := sampleRec(StatusFailure)
	outside.TsUnixMs = 9000
	for _, r := range []*Record{inside, outside} {
		require.NoError(t, w.Append(r))
	}
	require.NoError(t, w.Close())

	// until=5000 排除 9000（until 臂）；since=0 不排除
	n, err := Export(ExportOptions{SourceDir: dir, Filename: "exec.log", DestDir: dest, UntilUnixMs: 5000})
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestExportCSVCreateFailure(t *testing.T) {
	dir := t.TempDir()
	w, err := OpenFileWriter(dir, "exec.log", WriterOptions{})
	require.NoError(t, err)
	require.NoError(t, w.Append(sampleRec(StatusSuccess)))
	require.NoError(t, w.Close())

	ro := filepath.Join(t.TempDir(), "ro")
	require.NoError(t, os.MkdirAll(ro, 0o755))
	require.NoError(t, os.Chmod(ro, 0o555))
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })

	// MkdirAll 对已存在目录成功；os.Create 于只读目录 → EACCES
	n, err := Export(ExportOptions{SourceDir: dir, Filename: "exec.log", DestDir: ro, Format: ExportCSV})
	require.Error(t, err)
	require.Equal(t, 1, n)
}

func TestExportJSONLSkipsUnmarshalableRecord(t *testing.T) {
	dest := t.TempDir()
	bad := sampleRec(StatusSuccess)
	bad.Params = json.RawMessage("{bad") // 不可再编码
	err := writeJSONL(filepath.Join(dest, "out.jsonl"), []*Record{bad})
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(dest, "out.jsonl"))
	require.NoError(t, err)
	require.Empty(t, data, "unmarshalable record must be skipped")
}

func TestLoggerRecordsDefaultTimestamp(t *testing.T) {
	l, dir := newTestLogger(t)
	rec := sampleRec(StatusSuccess)
	rec.TsUnixMs = 0 // 触发 now 闭包与赋值臂
	require.NoError(t, l.Record(rec))
	require.Greater(t, rec.TsUnixMs, int64(0), "default timestamp must be injected")

	recs, err := ReadRecords(filepath.Join(dir, "exec.log"))
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.Equal(t, rec.TsUnixMs, recs[0].TsUnixMs)
}

func TestUploadFailuresNilSafe(t *testing.T) {
	var nilL *Logger
	require.Zero(t, nilL.UploadFailures())
}

func TestFileWriterErrorArms(t *testing.T) {
	// 目录创建失败（父路径被文件占位）
	occupied := filepath.Join(t.TempDir(), "occupied")
	require.NoError(t, os.WriteFile(occupied, []byte("x"), 0o600))
	_, err := OpenFileWriter(filepath.Join(occupied, "sub"), "exec.log", WriterOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "create dir")

	// Marshal 失败：Params 不可编码
	w, err := OpenFileWriter(t.TempDir(), "exec.log", WriterOptions{})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()
	bad := sampleRec(StatusSuccess)
	bad.Params = json.RawMessage("{bad")
	require.ErrorContains(t, w.Append(bad), "marshal record")

	// 底层写失败：lumberjack 的 MaxSize 超限错误（writeLen > max）在打开
	// 前返回 → 触达 Write 错误包装臂。目录占位会被 lumberjack 重命名而非
	// 报错（openNew 会把目录 ren 成备份再新建文件），非注入缝。
	big := sampleRec(StatusSuccess)
	big.Params = json.RawMessage(`"` + strings.Repeat("a", 2<<20) + `"`) // >1MB
	w2, err := OpenFileWriter(t.TempDir(), "exec.log", WriterOptions{MaxSizeMB: 1})
	require.NoError(t, err)
	defer func() { _ = w2.Close() }()
	require.ErrorContains(t, w2.Append(big), "write record")
}

func TestReadRecordsMissingFile(t *testing.T) {
	_, err := ReadRecords(filepath.Join(t.TempDir(), "nope.log"))
	require.Error(t, err)
}

func TestSplitLinesTrailingSegment(t *testing.T) {
	// 无尾换行：末段仍收（113-115 臂）
	require.Equal(t, [][]byte{[]byte("a"), []byte("b")}, splitLines([]byte("a\nb")))
	require.Equal(t, [][]byte{[]byte("a"), []byte("b")}, splitLines([]byte("a\nb\n")))
	// 空内容
	require.Empty(t, splitLines(nil))
}

func TestNowMs(t *testing.T) {
	require.Greater(t, NowMs(), int64(0))
	require.LessOrEqual(t, NowMs(), time.Now().UnixMilli())
}
