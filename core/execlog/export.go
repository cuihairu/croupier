package execlog

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportFormat 导出文件格式（配置位默认 jsonl，随实施报批）。
type ExportFormat string

const (
	ExportJSONL ExportFormat = "jsonl"
	ExportCSV   ExportFormat = "csv"
)

// ExportOptions 导出参数：扫 SourceDir 下审计文件（含轮转文件），按时间窗
// 过滤后写入 DestDir。零值时间窗 = 全量。
type ExportOptions struct {
	SourceDir   string
	Filename    string // 审计文件基名（OpenFileWriter 的 filename，轮转文件自动并入）
	DestDir     string
	Format      ExportFormat // 缺省 jsonl
	SinceUnixMs int64
	UntilUnixMs int64
}

// Export 导出审计记录，返回导出行数。文件名形如 execlog-export-<ts>.<fmt>。
func Export(opts ExportOptions) (int, error) {
	if opts.SourceDir == "" || opts.Filename == "" || opts.DestDir == "" {
		return 0, fmt.Errorf("execlog: export requires sourceDir, filename and destDir")
	}
	format := opts.Format
	if format == "" {
		format = ExportJSONL
	}
	if format != ExportJSONL && format != ExportCSV {
		return 0, fmt.Errorf("execlog: unsupported export format %q", format)
	}

	// lumberjack 轮转文件名：基名 + -<timestamp> + 原扩展名，活动文件为基名本体。
	ext := filepath.Ext(opts.Filename)
	base := strings.TrimSuffix(opts.Filename, ext)
	matches, err := filepath.Glob(filepath.Join(opts.SourceDir, base+"-*"+ext))
	if err != nil {
		return 0, err
	}
	if fileExists(filepath.Join(opts.SourceDir, opts.Filename)) {
		matches = append(matches, filepath.Join(opts.SourceDir, opts.Filename))
	}

	var recs []*Record
	for _, p := range matches {
		rs, err := ReadRecords(p)
		if err != nil {
			continue // 轮转中被删/半行截断容错
		}
		recs = append(recs, rs...)
	}
	recs = filterRange(recs, opts.SinceUnixMs, opts.UntilUnixMs)

	if err := os.MkdirAll(opts.DestDir, 0o755); err != nil {
		return 0, err
	}
	dest := filepath.Join(opts.DestDir, fmt.Sprintf("execlog-export-%d%s", time.Now().UnixMilli(), exportExt(format)))
	if format == ExportCSV {
		return len(recs), writeCSV(dest, recs)
	}
	return len(recs), writeJSONL(dest, recs)
}

func filterRange(recs []*Record, sinceMs, untilMs int64) []*Record {
	out := make([]*Record, 0, len(recs))
	for _, r := range recs {
		if sinceMs > 0 && r.TsUnixMs < sinceMs {
			continue
		}
		if untilMs > 0 && r.TsUnixMs > untilMs {
			continue
		}
		out = append(out, r)
	}
	return out
}

func exportExt(f ExportFormat) string {
	if f == ExportCSV {
		return ".csv"
	}
	return ".jsonl"
}

func writeJSONL(dest string, recs []*Record) error {
	var b strings.Builder
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			continue
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(dest, []byte(b.String()), 0o644)
}

var csvHeader = []string{"taskId", "operator", "agentType", "agentInstance", "action", "params", "status", "summary", "durationMs", "tsUnixMs", "gameId", "env"}

func writeCSV(dest string, recs []*Record) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(csvHeader); err != nil {
		return err
	}
	for _, r := range recs {
		if err := w.Write([]string{
			r.TaskID, r.Operator, r.AgentType, r.AgentInstance, r.Action,
			string(r.Params), string(r.Status), r.Summary,
			fmt.Sprintf("%d", r.DurationMs), fmt.Sprintf("%d", r.TsUnixMs),
			r.GameID, r.Env,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
