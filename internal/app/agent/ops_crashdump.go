package agent

// 崩溃产物只读枚举（通用能力，跨平台）：
//   Linux  = systemd-coredump 目录 /var/lib/systemd/coredump（文件名自带 exe/pid/uid/signal/epoch）
//   Windows = WER 报告目录（ReportArchive/ReportQueue/LocalDumps，AppCrash_*.hdmp/.dmp/.wer）
//   macOS  = DiagnosticReports（用户域 + 系统域，.ips 头行 JSON / .crash 文本）
// 身份与边界（2026-10-20 已代决，用户口径「只看不管理」「通用能力」）：
//   - 只读目录项与文件名/首行元数据；不执行 coredumpctl/werutil/spindump 等诊断命令；
//   - 不读 dump 正文（core/dmp/ips 全文含进程内存或敏感栈，属敏感数据）；
//   - 不提权：只访问 agent 进程普通用户可读路径，不可读目录静默跳过，永不阻塞启动。

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// crashDumpListMax 单平台枚举上限（防目录膨胀拖慢巡检）。
const crashDumpListMax = 20

// CrashDumpEntry 是一条崩溃产物的只读元数据。
type CrashDumpEntry struct {
	Platform   string    `json:"platform"`
	Source     string    `json:"source"`
	Name       string    `json:"name"`
	Executable string    `json:"executable,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	SizeBytes  int64     `json:"sizeBytes"`
}

// crashDumpDirs 返回各平台的扫描目录（包级变量，测试注入临时目录）。
var crashDumpDirs = func(goos string) map[string][]string {
	switch goos {
	case "linux":
		return map[string][]string{"systemd-coredump": {"/var/lib/systemd/coredump"}}
	case "windows":
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		wer := filepath.Join(base, "Microsoft", "Windows", "WER")
		return map[string][]string{
			"wer-archive": {filepath.Join(wer, "ReportArchive")},
			"wer-queue":   {filepath.Join(wer, "ReportQueue")},
			"wer-local":   {filepath.Join(wer, "LocalDumps")},
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		dirs := map[string][]string{}
		if home != "" {
			dirs["diagnostic-reports-user"] = []string{filepath.Join(home, "Library", "Logs", "DiagnosticReports")}
		}
		dirs["diagnostic-reports-system"] = []string{"/Library/Logs/DiagnosticReports"}
		return dirs
	default:
		return nil
	}
}

// linuxCoreName systemd-coredump 文件名：<exe>-<pid>+<uid>+<sig>+<epoch>.core[.zst|.lz4|.gz]
var linuxCoreName = regexp.MustCompile(`^(.+)-(\d+)\+(\d+)\+(\d+)\+(\d+)\.core(?:\.(?:zst|lz4|gz))?$`)

// darwinIpsHeader .ips 首行 JSON 的最小字段面（app_name 可选）。
var darwinIpsHeader = regexp.MustCompile(`"app_name"\s*:\s*"([^"]+)"`)

// crashDumpExecutableWin WER 报告目录名形态：AppCrash_<exe>_... / <exe>_...，
// 取首个路径段整体作为可执行名（目录名整段进 executable）。
var crashDumpExecutableWin = regexp.MustCompile(`^([A-Za-z0-9._-]+)`)

// crashDumpPlatform 由 source 名解析目录内产物规范所属的平台。
// 扫描目录由 runtime.GOOS 决定（本机能看见哪些目录），但目录里的产物规范属于
// source：systemd-coredump 是 linux、diagnostic-reports-* 是 darwin、wer-* 是
// windows。两者分开后，解析器不依赖本机 GOOS 就能单独验证（注入 crashDumpDirs
// 即可），也不会在「本机是 linux 但目录是 WER 布局」时解析错位。
func crashDumpPlatform(source string) string {
	switch {
	case strings.HasPrefix(source, "systemd-coredump"):
		return "linux"
	case strings.HasPrefix(source, "diagnostic-reports"):
		return "darwin"
	case strings.HasPrefix(source, "wer-"):
		return "windows"
	default:
		return ""
	}
}

// scanCrashDumps 枚举当前平台崩溃产物（只读、best-effort、有界）。
func scanCrashDumps() []CrashDumpEntry {
	var out []CrashDumpEntry
	for source, dirs := range crashDumpDirs(runtime.GOOS) {
		goos := crashDumpPlatform(source)
		for _, dir := range dirs {
			out = append(out, scanCrashDumpDir(goos, source, dir)...)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	if len(out) > crashDumpListMax {
		out = out[:crashDumpListMax]
	}
	return out
}

// scanCrashDumpDir 扫描单个目录；目录不存在/不可读一律静默返回 nil。
func scanCrashDumpDir(goos, source, dir string) []CrashDumpEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []CrashDumpEntry
	for _, e := range entries {
		if e.IsDir() {
			// Windows WER：报告是目录，取目录内最大的 dump/wer 文件做元数据。
			if goos == "windows" {
				if entry, ok := scanWindowsReportDir(source, dir, e); ok {
					out = append(out, entry)
				}
			}
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		entry := CrashDumpEntry{
			Platform:   goos,
			Source:     source,
			Name:       e.Name(),
			Timestamp:  info.ModTime(),
			SizeBytes:  info.Size(),
			Executable: "",
		}
		switch goos {
		case "linux":
			// 只认 systemd-coredump 文件名（<exe>-<pid>+<uid>+<sig>+<epoch>.core[.zst|…]）：
			// 目录里的说明文件或手工放进去的文件不是转储，混进来只会污染崩溃列表。
			m := linuxCoreName.FindStringSubmatch(e.Name())
			if m == nil {
				continue
			}
			entry.Executable = m[1]
			if epoch, err := strconv.ParseInt(m[5], 10, 64); err == nil && epoch > 0 {
				entry.Timestamp = time.Unix(epoch, 0)
			}
		case "darwin":
			if strings.EqualFold(filepath.Ext(e.Name()), ".ips") || strings.EqualFold(filepath.Ext(e.Name()), ".crash") {
				entry.Executable = darwinExecutable(filepath.Join(dir, e.Name()))
			}
		default:
			// 非预期平台分支保持原始元数据（名称/大小/mtime 仍有观测价值）。
		}
		out = append(out, entry)
	}
	return out
}

// windowsReportScanDepth WER 报告目录树的下钻上限（ReportQueue/AppCrash_*/<日期>/文件），
// 有界避免在异常目录上无限下钻。
const windowsReportScanDepth = 4

// windowsDumpExt WER 报告目录里视为转储/报告载荷的文件扩展名。
var windowsDumpExt = map[string]bool{".dmp": true, ".hdmp": true, ".mdmp": true, ".wer": true}

// windowsDump 是 WER 报告目录树里最大的转储文件。
type windowsDump struct {
	rel   string // 相对报告目录的路径
	size  int64
	mtime time.Time
}

// scanWindowsReportDir 从 WER 报告目录提取最大转储文件的元数据（只读目录项）。
// 报告是目录树：ReportArchive|ReportQueue/AppCrash_*/<日期>/{*.dmp,*.hdmp,*.wer}，
// 也兼容 Report.wer 直接落在报告目录的旧布局；报告目录内没有任何转储文件就跳过。
func scanWindowsReportDir(source, parent string, e os.DirEntry) (CrashDumpEntry, bool) {
	dump, ok := largestWindowsDump(filepath.Join(parent, e.Name()), "", 0)
	if !ok {
		return CrashDumpEntry{}, false
	}
	entry := CrashDumpEntry{
		Platform:  "windows",
		Source:    source,
		Name:      filepath.Join(e.Name(), dump.rel),
		Timestamp: dump.mtime,
		SizeBytes: dump.size,
	}
	// 可执行名取转储所在的 AppCrash_* 报告目录；旧布局里报告目录名就是它。
	if seg, _, cut := strings.Cut(dump.rel, string(filepath.Separator)); cut {
		if m := crashDumpExecutableWin.FindStringSubmatch(seg); m != nil {
			entry.Executable = m[1]
		}
	} else if m := crashDumpExecutableWin.FindStringSubmatch(e.Name()); m != nil {
		entry.Executable = m[1]
	}
	return entry, true
}

// largestWindowsDump 在下钻深度内找最大的转储文件，返回相对 dir 的路径。
func largestWindowsDump(dir, rel string, depth int) (windowsDump, bool) {
	if depth > windowsReportScanDepth {
		return windowsDump{}, false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return windowsDump{}, false
	}
	var best windowsDump
	var seen bool
	for _, f := range entries {
		if f.IsDir() {
			if d, ok := largestWindowsDump(filepath.Join(dir, f.Name()), filepath.Join(rel, f.Name()), depth+1); ok {
				if !seen || d.size > best.size {
					best, seen = d, true
				}
			}
			continue
		}
		if !windowsDumpExt[strings.ToLower(filepath.Ext(f.Name()))] {
			continue
		}
		info, err := f.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if !seen || info.Size() > best.size {
			best = windowsDump{rel: filepath.Join(rel, f.Name()), size: info.Size(), mtime: info.ModTime()}
			seen = true
		}
	}
	return best, seen
}

// darwinExecutable 只读 .ips 首行（头部 JSON）提取 app_name；.crash 走文本 "Process:" 行。
// 失败返回空串（best-effort，不读全文）。
func darwinExecutable(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	head := string(buf[:n])
	if idx := strings.IndexByte(head, '\n'); idx >= 0 {
		head = head[:idx] // .ips 只看首行 JSON
	}
	if m := darwinIpsHeader.FindStringSubmatch(head); m != nil {
		return m[1]
	}
	if proc := regexp.MustCompile(`^Process:\s+(\S+)`).FindStringSubmatch(head); proc != nil {
		return proc[1]
	}
	return ""
}
