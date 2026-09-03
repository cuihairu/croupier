package common

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// TestLoadWithIncludes_MissingBaseFileV9 覆盖 config.go:13-15 基础配置读取失败分支。
func TestLoadWithIncludes_MissingBaseFileV9(t *testing.T) {
	tmpDir := t.TempDir()
	v, err := LoadWithIncludes(filepath.Join(tmpDir, "no-such-base.yaml"), nil)
	if err == nil {
		t.Fatal("expected error for missing base config file")
	}
	if v != nil {
		t.Fatal("expected nil viper on base read error")
	}
}

// TestApplySectionAndProfile_ProfileMissingInSectionV9 覆盖 config.go:57-59
// section 内存在 profiles 但请求的 profile 不存在。
func TestApplySectionAndProfile_ProfileMissingInSectionV9(t *testing.T) {
	v := viper.New()
	v.Set("server.port", 8080)
	v.Set("server.profiles.dev.port", 8081)

	_, err := ApplySectionAndProfile(v, "server", "staging")
	if err == nil {
		t.Fatal("expected error for missing profile inside section")
	}
}

// TestColoredTextHandler_HandleNonTerminalFileV9 覆盖 logging.go:58-60 与
// isTerminal：writer 为非终端 *os.File 时退回底层 handler。
func TestColoredTextHandler_HandleNonTerminalFileV9(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "log.txt"))
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer f.Close()

	handler := newColoredTextHandler(f, nil)
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "plain", 0)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	info, _ := f.Stat()
	if info.Size() == 0 {
		t.Fatal("expected delegated handler to write output")
	}
}

// TestIsTerminalV9 覆盖 logging.go:106-109 的 true/false 两个分支。
func TestIsTerminalV9(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "regular.txt"))
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("regular file should not be a terminal")
	}

	devNull, err := os.Open("/dev/null")
	if err != nil {
		t.Fatalf("open /dev/null: %v", err)
	}
	defer devNull.Close()
	if !isTerminal(devNull) {
		t.Error("/dev/null is a char device and should be reported as terminal")
	}
}

// TestColoredTextHandler_HandleWithAttrsV9 覆盖 logging.go:90-96 属性循环。
func TestColoredTextHandler_HandleWithAttrsV9(t *testing.T) {
	var buf bytes.Buffer
	handler := newColoredTextHandler(&buf, nil)

	record := slog.NewRecord(time.Now(), slog.LevelWarn, "with attrs", 0)
	record.AddAttrs(slog.String("key", "value"), slog.Int("num", 7))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("key=value")) {
		t.Errorf("output should contain attr key=value, got %q", out)
	}
}

// TestCountHandler_HandleAllLevelsV9 覆盖 logging.go:187-194 debug/warn/error 计数分支。
func TestCountHandler_HandleAllLevelsV9(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	ch := &countHandler{next: base}

	levels := []slog.Level{slog.LevelDebug, slog.LevelWarn, slog.LevelError}
	for _, lvl := range levels {
		rec := slog.NewRecord(time.Now(), lvl, "msg", 0)
		if err := ch.Handle(context.Background(), rec); err != nil {
			t.Fatalf("Handle(%v): %v", lvl, err)
		}
	}
	if buf.Len() == 0 {
		t.Fatal("expected underlying handler output")
	}
}

// TestSetupLoggerWithFile_OutputEnvStderrV9 覆盖 logging.go:117-122 两个环境变量分支。
func TestSetupLoggerWithFile_OutputEnvStderrV9(t *testing.T) {
	t.Setenv("LOG_OUTPUT", "stderr")
	SetupLoggerWithFile("info", "console", "", 1, 1, 1, false)

	t.Setenv("LOG_OUTPUT", "")
	t.Setenv("CROUPIER_LOG_OUTPUT", "stderr")
	SetupLoggerWithFile("info", "console", "", 1, 1, 1, false)
	slog.Info("after env stderr")
}

// TestSetupLoggerWithFile_MkdirFailV9 覆盖 logging.go:128-131 日志目录创建失败分支。
func TestSetupLoggerWithFile_MkdirFailV9(t *testing.T) {
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	logFile := filepath.Join(blocker, "sub", "app.log")
	SetupLoggerWithFile("info", "console", logFile, 1, 1, 1, false)
	slog.Info("mkdir failed fallback")
}

// TestSetupLoggerWithFile_WarnErrorLevelsV9 覆盖 logging.go:148-151 warn/error 级别分支。
func TestSetupLoggerWithFile_WarnErrorLevelsV9(t *testing.T) {
	SetupLoggerWithFile("warn", "console", "", 1, 1, 1, false)
	slog.Warn("warn level")

	SetupLoggerWithFile("error", "console", "", 1, 1, 1, false)
	slog.Error("error level")
}

// TestValidateTLS_NonStrictMissingKeyAndCAV9 覆盖 validate.go:44-51：
// 非 strict 模式下提供了 key/ca 但文件不存在。
func TestValidateTLS_NonStrictMissingKeyAndCAV9(t *testing.T) {
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "key.pem")
	caFile := filepath.Join(tmpDir, "ca.pem")
	os.WriteFile(keyFile, []byte("k"), 0o644)
	os.WriteFile(caFile, []byte("c"), 0o644)

	if err := ValidateTLS("", filepath.Join(tmpDir, "missing-key"), "", false); err == nil {
		t.Error("expected error for missing key file in non-strict mode")
	}
	if err := ValidateTLS("", "", filepath.Join(tmpDir, "missing-ca"), false); err == nil {
		t.Error("expected error for missing ca file in non-strict mode")
	}
	if err := ValidateTLS("", keyFile, caFile, false); err != nil {
		t.Errorf("existing key/ca should pass: %v", err)
	}
}

// TestValidateServerConfig_ErrorPathsV9 覆盖 ValidateServerConfig 的 TLS/数据文件错误分支。
func TestValidateServerConfig_ErrorPathsV9(t *testing.T) {
	tmpDir := t.TempDir()
	badJSON := filepath.Join(tmpDir, "bad.json")
	os.WriteFile(badJSON, []byte("{ not json"), 0o644)
	validRBAC := filepath.Join(tmpDir, "rbac.json")
	os.WriteFile(validRBAC, []byte(`{"allow": {}}`), 0o644)
	validUsers := filepath.Join(tmpDir, "users.json")
	os.WriteFile(validUsers, []byte(`[]`), 0o644)
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")
	caFile := filepath.Join(tmpDir, "ca.pem")
	os.WriteFile(certFile, []byte("cert"), 0o644)
	os.WriteFile(keyFile, []byte("key"), 0o644)
	os.WriteFile(caFile, []byte("ca"), 0o644)

	newV := func(kv map[string]any) *viper.Viper {
		v := viper.New()
		for k, val := range kv {
			v.Set(k, val)
		}
		return v
	}
	base := map[string]any{
		"server.addr":      "localhost:8443",
		"server.http_addr": "localhost:18780",
	}

	tests := []struct {
		name   string
		extra  map[string]any
		strict bool
	}{
		{
			name: "strict 模式 TLS 文件齐全",
			extra: map[string]any{
				"server.cert": certFile, "server.key": keyFile, "server.ca": caFile,
			},
			strict: true,
		},
		{
			name:   "TLS cert 不存在",
			extra:  map[string]any{"server.cert": filepath.Join(tmpDir, "no-cert.pem")},
			strict: false,
		},
		{
			name:   "rbac_config 解析失败",
			extra:  map[string]any{"server.rbac_config": badJSON},
			strict: false,
		},
		{
			name:   "users_config 解析失败",
			extra:  map[string]any{"server.users_config": badJSON},
			strict: false,
		},
		{
			name: "strict 缺少 users_config",
			extra: map[string]any{
				"server.cert": certFile, "server.key": keyFile, "server.ca": caFile,
				"server.rbac_config": validRBAC,
			},
			strict: true,
		},
		{
			name:   "games_config 文件不存在",
			extra:  map[string]any{"server.games_config": filepath.Join(tmpDir, "no-games.yaml")},
			strict: false,
		},
		{
			name: "strict 缺少 games_config",
			extra: map[string]any{
				"server.cert": certFile, "server.key": keyFile, "server.ca": caFile,
				"server.rbac_config": validRBAC, "server.users_config": validUsers,
			},
			strict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv := make(map[string]any, len(base)+len(tt.extra))
			for k, v := range base {
				kv[k] = v
			}
			for k, v := range tt.extra {
				kv[k] = v
			}
			if err := ValidateServerConfig(newV(kv), tt.strict); err == nil {
				t.Errorf("ValidateServerConfig(%s) should return error", tt.name)
			}
		})
	}
}

// TestValidateAgentConfig_ErrorPathsV9 覆盖 validate.go:128-133 agent TLS 与 http_addr 错误分支。
func TestValidateAgentConfig_ErrorPathsV9(t *testing.T) {
	newV := func(kv map[string]any) *viper.Viper {
		v := viper.New()
		for k, val := range kv {
			v.Set(k, val)
		}
		return v
	}

	if err := ValidateAgentConfig(newV(map[string]any{
		"agent.local_addr":  "localhost:19091",
		"agent.server_addr": "localhost:19090",
		"agent.http_addr":   "localhost:18780",
		"agent.cert":        "/nonexistent/cert.pem",
	}), false); err == nil {
		t.Error("expected TLS error for agent with missing cert")
	}

	if err := ValidateAgentConfig(newV(map[string]any{
		"agent.local_addr":  "localhost:19091",
		"agent.server_addr": "localhost:19090",
		"agent.http_addr":   "localhost",
	}), false); err == nil {
		t.Error("expected http_addr error for agent")
	}
}
