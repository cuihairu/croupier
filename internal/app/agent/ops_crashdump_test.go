package agent

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func withCrashDumpDirs(t *testing.T, dirs map[string][]string) {
	t.Helper()
	orig := crashDumpDirs
	crashDumpDirs = func(string) map[string][]string { return dirs }
	t.Cleanup(func() { crashDumpDirs = orig })
}

func TestScanLinuxCoreDumpsParsesFilenameMetadata(t *testing.T) {
	dir := t.TempDir()
	withCrashDumpDirs(t, map[string][]string{"systemd-coredump": {dir}})
	epoch := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC).Unix()
	name := "game-server-1234+1000+11+" + itoa(epoch) + ".core.zst"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	// 非法名与子目录必须被跳过。
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	entries := scanCrashDumps()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d: %+v", len(entries), entries)
	}
	got := entries[0]
	if got.Executable != "game-server" || got.Platform != "linux" || got.Source != "systemd-coredump" {
		t.Fatalf("metadata wrong: %+v", got)
	}
	if !got.Timestamp.Equal(time.Unix(epoch, 0)) {
		t.Fatalf("epoch should drive timestamp: %v", got.Timestamp)
	}
}

func TestScanCrashDumpsUnreadableDirIsSilent(t *testing.T) {
	withCrashDumpDirs(t, map[string][]string{"systemd-coredump": {filepath.Join(t.TempDir(), "missing")}})
	if entries := scanCrashDumps(); entries != nil {
		t.Fatalf("missing dir must be silent, got %+v", entries)
	}
}

func TestScanDarwinIpsExtractsAppNameFromHeader(t *testing.T) {
	dir := t.TempDir()
	withCrashDumpDirs(t, map[string][]string{"diagnostic-reports-user": {dir}})
	ips := "{\"app_name\":\"MyGame\",\"timestamp\":\"2026-10-20T08:00:00Z\"}\n{\n  \"other\": 1\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "MyGame-2026-10-20-080000.ips"), []byte(ips), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Plain-2026.ips"), []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := scanCrashDumps()
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	var sawApp, sawSilent bool
	for _, e := range entries {
		if e.Executable == "MyGame" {
			sawApp = true
		}
		if filepath.Base(e.Name) == "Plain-2026.ips" && e.Executable == "" {
			sawSilent = true
		}
	}
	if !sawApp || !sawSilent {
		t.Fatalf("ips parse broken: %+v", entries)
	}
}

func TestScanWindowsReportDirPicksLargestDump(t *testing.T) {
	base := t.TempDir()
	report := filepath.Join(base, "ReportArchive", "ReportQueue", "AppCrash_MyGame_exe", "2026-10-20")
	withCrashDumpDirs(t, map[string][]string{"wer-archive": {filepath.Join(base, "ReportArchive")}})
	if err := os.MkdirAll(report, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(report, "memory.hdmp"), make([]byte, 10), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(report, "report.wer"), make([]byte, 99), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := scanCrashDumpDir("windows", "wer-archive", filepath.Join(base, "ReportArchive"))
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d: %+v", len(entries), entries)
	}
	got := entries[0]
	if got.SizeBytes != 99 || got.Executable != "AppCrash_MyGame_exe" && got.Executable != "AppCrash_MyGame" {
		// 目录名整体进 exe（首段正则），大小取最大文件。
		t.Fatalf("windows metadata wrong: %+v", got)
	}
}

func TestScanCrashDumpsSortsNewestFirstAndCaps(t *testing.T) {
	dir := t.TempDir()
	withCrashDumpDirs(t, map[string][]string{"systemd-coredump": {dir}})
	for i := 0; i < crashDumpListMax+5; i++ {
		epoch := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour).Unix()
		name := "p-1+1+6+" + itoa(epoch) + ".core"
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	entries := scanCrashDumps()
	if len(entries) != crashDumpListMax {
		t.Fatalf("cap %d expected, got %d", crashDumpListMax, len(entries))
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp.After(entries[i-1].Timestamp) {
			t.Fatalf("not sorted newest-first: %+v", entries)
		}
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func TestCrashDumpDirsUnsupportedPlatformIsNil(t *testing.T) {
	if got := crashDumpDirs("plan9"); got != nil {
		t.Fatalf("unsupported platform must be nil, got %+v", got)
	}
}

func TestCrashDumpDirsWindowsFallsBackWhenProgramDataMissing(t *testing.T) {
	t.Setenv("ProgramData", "")
	dirs := crashDumpDirs("windows")
	if len(dirs["wer-archive"]) != 1 || dirs["wer-archive"][0] == "" {
		t.Fatalf("windows fallback base broken: %+v", dirs)
	}
}

func TestScanDarwinCrashTextExtractsProcess(t *testing.T) {
	dir := t.TempDir()
	withCrashDumpDirs(t, map[string][]string{"diagnostic-reports-system": {dir}})
	body := "Process:             MyServer [42]\nPath:                /usr/local/bin/MyServer\nDate/Time:           2026-10-20 08:00:00 +0800\n"
	if err := os.WriteFile(filepath.Join(dir, "MyServer-2026-10-20-080000.crash"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := scanCrashDumps()
	if len(entries) != 1 || entries[0].Executable != "MyServer" {
		t.Fatalf("crash text parse broken: %+v", entries)
	}
}

func TestScanWindowsReportDirSkipsDirWithoutDumps(t *testing.T) {
	base := t.TempDir()
	report := filepath.Join(base, "r1", "AppCrash_X")
	if err := os.MkdirAll(report, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(report, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := scanWindowsReportDir("wer-archive", base, dirEntryStub{name: "AppCrash_X"}); ok {
		t.Fatal("report dir without dumps must be skipped")
	}
}

func TestScanCrashDumpDirUnsupportedPlatformKeepsRawMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mystery.bin"), []byte("xy"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := scanCrashDumpDir("plan9", "src", dir)
	if len(entries) != 1 || entries[0].Name != "mystery.bin" || entries[0].SizeBytes != 2 {
		t.Fatalf("raw metadata must survive: %+v", entries)
	}
}

// dirEntryStub 只实现 scanWindowsReportDir 用到的表面。
type dirEntryStub struct{ name string }

func (d dirEntryStub) Name() string               { return d.name }
func (d dirEntryStub) IsDir() bool                { return true }
func (d dirEntryStub) Type() os.FileMode          { return os.ModeDir }
func (d dirEntryStub) Info() (os.FileInfo, error) { return nil, os.ErrNotExist }
