package chain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNewWriter 测试 Writer 创建
func TestNewWriter(t *testing.T) {
	// 创建临时文件路径
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// 创建 Writer
	w, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	// 验证文件已创建
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}

// TestNewWriter_CreateDirectory 测试自动创建目录
func TestNewWriter_CreateDirectory(t *testing.T) {
	// 创建深层嵌套路径
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "level1", "level2", "level3", "audit.log")

	w, err := NewWriter(deepPath)
	if err != nil {
		t.Fatalf("NewWriter with nested path failed: %v", err)
	}
	defer w.Close()

	// 验证文件存在
	if _, err := os.Stat(deepPath); os.IsNotExist(err) {
		t.Error("Log file was not created in nested directory")
	}
}

// TestWriter_Log 测试日志记录
func TestWriter_Log(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	w, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	// 记录事件
	err = w.Log("login", "user1", "system", map[string]string{
		"ip":       "192.168.1.1",
		"success":  "true",
		"duration": "100ms",
	})
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// 读取日志文件验证内容
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// 验证 JSON 格式
	var event Event
	if err := json.Unmarshal(content, &event); err != nil {
		t.Fatalf("Failed to parse event JSON: %v", err)
	}

	// 验证事件内容
	if event.Kind != "login" {
		t.Errorf("Expected kind 'login', got '%s'", event.Kind)
	}
	if event.Actor != "user1" {
		t.Errorf("Expected actor 'user1', got '%s'", event.Actor)
	}
	if event.Target != "system" {
		t.Errorf("Expected target 'system', got '%s'", event.Target)
	}
	if event.Meta["ip"] != "192.168.1.1" {
		t.Errorf("Expected meta IP '192.168.1.1', got '%s'", event.Meta["ip"])
	}

	// 验证必填字段
	if event.Hash == "" {
		t.Error("Hash should not be empty")
	}
	if event.Prev == "" {
		t.Error("Prev should not be empty (initial hash)")
	}
	if event.Time.IsZero() {
		t.Error("Time should not be zero")
	}
}

// TestWriter_LogChainIntegrity 测试链完整性
func TestWriter_LogChainIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	w, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	events := []struct {
		kind   string
		actor  string
		target string
		meta   map[string]string
	}{
		{"login", "user1", "system", nil},
		{"action", "user1", "resource1", nil},
		{"logout", "user1", "system", nil},
	}

	var previousHash string

	// 记录多个事件并验证链式关系
	for i, ev := range events {
		err := w.Log(ev.kind, ev.actor, ev.target, ev.meta)
		if err != nil {
			t.Fatalf("Log %d failed: %v", i, err)
		}

		// 读取最后写入的事件
		content, _ := os.ReadFile(logPath)
		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		var event Event
		json.Unmarshal([]byte(lines[len(lines)-1]), &event)

		// 验证前一个哈希匹配（除第一个事件外）
		if i > 0 {
			if event.Prev != previousHash {
				t.Errorf("Event %d: prev hash mismatch. Expected '%s', got '%s'",
					i, previousHash, event.Prev)
			}
		}
		previousHash = event.Hash

		// 验证哈希非空
		if event.Hash == "" {
			t.Errorf("Event %d: hash is empty", i)
		}
	}
}

// TestWriter_ConcurrentLogs 测试并发日志记录
func TestWriter_ConcurrentLogs(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	w, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	// 并发写入
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			err := w.Log("action", "user", "target", map[string]string{"index": string(rune(idx))})
			if err != nil {
				t.Errorf("Concurrent log %d failed: %v", idx, err)
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证文件内容
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 10 {
		t.Errorf("Expected 10 log entries, got %d", len(lines))
	}
}

// TestWriter_LogTimestamp 测试时间戳
func TestWriter_LogTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	w, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	before := time.Now().UTC()
	err = w.Log("test", "actor", "target", nil)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	after := time.Now().UTC()

	// 读取事件
	content, _ := os.ReadFile(logPath)
	var event Event
	json.Unmarshal(content, &event)

	// 验证时间戳在合理范围内
	if event.Time.Before(before) {
		t.Error("Event time is before log was called")
	}
	if event.Time.After(after) {
		t.Error("Event time is after log was called")
	}

	// 验证 UTC 时区
	if _, offset := event.Time.Zone(); offset != 0 {
		t.Error("Event time should be in UTC")
	}
}

// TestWriter_EmptyMeta 测试空元数据
func TestWriter_EmptyMeta(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	w, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	// 使用 nil 元数据
	err = w.Log("test", "actor", "target", nil)
	if err != nil {
		t.Fatalf("Log with nil meta failed: %v", err)
	}

	// 使用空元数据
	err = w.Log("test", "actor", "target", map[string]string{})
	if err != nil {
		t.Fatalf("Log with empty meta failed: %v", err)
	}

	content, _ := os.ReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	for i, line := range lines {
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("Failed to parse event %d: %v", i, err)
		}

		// Meta 可能为 nil 或空 map，都是有效的
		// JSON 解析会将空对象解析为空 map，而不是 nil
		_ = event.Meta // 只要能解析就 OK
	}
}

// TestFilepathDir 测试 filepathDir 辅助函数
func TestFilepathDir(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/to/file.txt", "/path/to"},
		{"./relative/path/file.txt", "./relative/path"},
		{"file.txt", "."},
		{"", "."}, // 空字符串返回 "."
		{"/a/b/c/d/e/file.txt", "/a/b/c/d/e"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := filepathDir(tt.input)
			if result != tt.expected {
				t.Errorf("filepathDir(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestWriter_Close 测试关闭 Writer
func TestWriter_Close(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	w, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	// 写入一些数据
	err = w.Log("test", "actor", "target", nil)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// 关闭 Writer
	err = w.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 验证文件已刷新
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file after close: %v", err)
	}

	if len(content) == 0 {
		t.Error("Log file is empty after close")
	}
}

// TestWriter_HashUniqueness 测试哈希唯一性
func TestWriter_HashUniqueness(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	w, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	hashes := make(map[string]bool)

	// 记录 100 个相同的事件
	for i := 0; i < 100; i++ {
		err := w.Log("action", "user", "target", nil)
		if err != nil {
			t.Fatalf("Log %d failed: %v", i, err)
		}

		// 读取最后一个事件的哈希
		content, _ := os.ReadFile(logPath)
		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		var event Event
		json.Unmarshal([]byte(lines[len(lines)-1]), &event)

		// 验证哈希唯一性（因为 prev hash 不同）
		if hashes[event.Hash] {
			t.Errorf("Duplicate hash found: %s", event.Hash)
		}
		hashes[event.Hash] = true
	}

	// 验证所有哈希都不同
	if len(hashes) != 100 {
		t.Errorf("Expected 100 unique hashes, got %d", len(hashes))
	}
}

// BenchmarkWriter_LogSingleThreaded 单线程基准测试
func BenchmarkWriter_LogSingleThreaded(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	w, _ := NewWriter(logPath)
	defer w.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Log("action", "user", "target", nil)
	}
}

// BenchmarkWriter_LogConcurrent 并发基准测试
func BenchmarkWriter_LogConcurrent(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	w, _ := NewWriter(logPath)
	defer w.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w.Log("action", "user", "target", nil)
		}
	})
}
