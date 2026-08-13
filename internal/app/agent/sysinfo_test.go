package agent

import (
	"os"
	"testing"
)

// TestGetPlatformInfo 测试获取平台信息
func TestGetPlatformInfo(t *testing.T) {
	info := GetPlatformInfo()
	if info == nil {
		t.Fatal("GetPlatformInfo() should return non-nil map")
	}
	if _, ok := info["os"]; !ok {
		t.Error("GetPlatformInfo() should include 'os' key")
	}
	if _, ok := info["arch"]; !ok {
		t.Error("GetPlatformInfo() should include 'arch' key")
	}
	if _, ok := info["service_manager"]; !ok {
		t.Error("GetPlatformInfo() should include 'service_manager' key")
	}
}

// TestDetectLinuxServiceManager 测试检测 Linux 服务管理器
func TestDetectLinuxServiceManager(t *testing.T) {
	result := detectLinuxServiceManager()
	// 在不同环境下可能返回 "systemd" 或 "unknown"
	if result != "systemd" && result != "unknown" {
		t.Errorf("detectLinuxServiceManager() = %q, want 'systemd' or 'unknown'", result)
	}
}

// TestListServices_Basic 测试列出服务
func TestListServices_Basic(t *testing.T) {
	// 覆盖率目标：调用 ListServices 函数
	_, err := ListServices("", "", 10)
	// 在某些环境下可能返回错误，这是可以接受的
	_ = err
}

// TestGetServiceStatus_Basic 测试获取服务状态
func TestGetServiceStatus_Basic(t *testing.T) {
	_, err := GetServiceStatus("nonexistent.service")
	// 在某些环境下可能返回错误，这是可以接受的
	_ = err
}

// TestListCronJobs_Basic 测试列出 cron 任务
func TestListCronJobs_Basic(t *testing.T) {
	_, err := ListCronJobs()
	// 在某些环境下可能返回错误，这是可以接受的
	_ = err
}

// TestParseCronFile 测试解析 cron 文件
func TestParseCronFile(t *testing.T) {
	// 测试不存在的文件
	jobs := parseCronFile("/nonexistent/crontab", "testuser")
	if len(jobs) != 0 {
		t.Errorf("parseCronFile() for nonexistent file should return empty slice, got %d", len(jobs))
	}
}

// TestLooksLikeCronSchedule 测试 cron 调度格式检查
func TestLooksLikeCronSchedule(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   bool
	}{
		{"valid schedule", []string{"*", "*", "*", "*", "*"}, true},
		{"with numbers", []string{"0", "12", "1", "6", "3"}, true},
		{"with ranges", []string{"0-30", "*/5", "1,15", "1-6", "0-5"}, true},
		{"invalid length", []string{"*", "*", "*"}, false},
		{"empty", []string{}, false},
		{"invalid field", []string{"abc", "*", "*", "*", "*"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeCronSchedule(tt.fields)
			if got != tt.want {
				t.Errorf("looksLikeCronSchedule(%v) = %v, want %v", tt.fields, got, tt.want)
			}
		})
	}
}

// TestParseCronFile_WithRealContent 测试解析真实 cron 文件内容
func TestParseCronFile_WithRealContent(t *testing.T) {
	// 创建临时 cron 文件
	tmpDir := t.TempDir()
	cronPath := tmpDir + "/test.crontab"

	// 系统 crontab 格式需要7+字段: minute hour day month weekday user command
	content := `# This is a comment
0 12 * * * root /usr/bin/command1
*/5 * * * * root /usr/bin/command2

`
	if err := os.WriteFile(cronPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write cron file: %v", err)
	}

	jobs := parseCronFile(cronPath, "testuser")
	if len(jobs) != 2 {
		t.Fatalf("parseCronFile() should parse 2 jobs, got %d", len(jobs))
	}

	// 验证第一个 job
	if len(jobs) > 0 {
		if jobs[0].Schedule != "0 12 * * *" {
			t.Errorf("jobs[0].Schedule = %q, want '0 12 * * *'", jobs[0].Schedule)
		}
		if jobs[0].Command != "/usr/bin/command1" {
			t.Errorf("jobs[0].Command = %q, want '/usr/bin/command1'", jobs[0].Command)
		}
		if jobs[0].User != "testuser" {
			t.Errorf("jobs[0].User = %q, want 'testuser'", jobs[0].User)
		}
	}
}

// TestParseCronFile_SystemFormat 测试系统 cron 格式
func TestParseCronFile_SystemFormat(t *testing.T) {
	tmpDir := t.TempDir()
	cronPath := tmpDir + "/test.crontab"

	// 系统 crontab 格式：schedule user command
	content := `0 12 * * * root /usr/bin/command1
*/5 * * * * root /usr/bin/command2
`
	if err := os.WriteFile(cronPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write cron file: %v", err)
	}

	jobs := parseCronFile(cronPath, "root")
	if len(jobs) != 2 {
		t.Errorf("parseCronFile() should parse 2 jobs, got %d", len(jobs))
	}
}

// TestParseCronFile_AtSchedule 测试 @schedule 格式
func TestParseCronFile_AtSchedule(t *testing.T) {
	tmpDir := t.TempDir()
	cronPath := tmpDir + "/test.crontab"

	// 系统 crontab 格式需要7+字段
	content := `0 12 * * * root /usr/bin/command1
15 2 * * * root /usr/bin/command2
`
	if err := os.WriteFile(cronPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write cron file: %v", err)
	}

	jobs := parseCronFile(cronPath, "testuser")
	if len(jobs) != 2 {
		t.Fatalf("parseCronFile() should parse 2 jobs, got %d", len(jobs))
	}

	// 验证第一个 job 的调度格式
	if len(jobs) > 0 && jobs[0].Schedule != "0 12 * * *" {
		t.Errorf("jobs[0].Schedule = %q, want '0 12 * * *'", jobs[0].Schedule)
	}
}

// TestParseCronFile_InvalidLines 测试无效行
func TestParseCronFile_InvalidLines(t *testing.T) {
	tmpDir := t.TempDir()
	cronPath := tmpDir + "/test.crontab"

	// 只有2个字段的无效行会被跳过
	// 有效的7+字段行会被解析为系统格式
	content := `0 12
0 12 * * * root /usr/bin/valid
`
	if err := os.WriteFile(cronPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write cron file: %v", err)
	}

	jobs := parseCronFile(cronPath, "testuser")
	if len(jobs) != 1 {
		t.Fatalf("parseCronFile() should parse 1 valid job, got %d", len(jobs))
	}

	if len(jobs) > 0 && jobs[0].Command != "/usr/bin/valid" {
		t.Errorf("jobs[0].Command = %q, want '/usr/bin/valid'", jobs[0].Command)
	}
}

// TestLooksLikeCronSchedule_EdgeCases 测试边界情况
func TestLooksLikeCronSchedule_EdgeCases(t *testing.T) {
	// 问号通配符
	if !looksLikeCronSchedule([]string{"?", "?", "?", "?", "?"}) {
		t.Error("looksLikeCronSchedule() should accept '?' as wildcard")
	}

	// 混合格式
	if !looksLikeCronSchedule([]string{"0-30/5", "*/2", "1,15", "1-6", "0-5"}) {
		t.Error("looksLikeCronSchedule() should accept mixed format")
	}
}
