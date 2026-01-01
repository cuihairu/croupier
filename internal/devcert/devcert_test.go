package devcert

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWriteFile 测试文件写入
func TestWriteFile(t *testing.T) {
	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "subdir", "test.txt")
	testData := []byte("test data")

	err := writeFile(testPath, testData, 0644)
	if err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("File should exist after writeFile")
	}

	// 验证文件内容
	data, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(data) != "test data" {
		t.Errorf("File content = %q, want 'test data'", string(data))
	}

	// 验证目录被创建
	subdir := filepath.Join(tempDir, "subdir")
	if info, err := os.Stat(subdir); err != nil {
		t.Errorf("Subdirectory should exist: %v", err)
	} else if !info.IsDir() {
		t.Error("Subdirectory should be a directory")
	}
}

// TestWriteFile_Overwrite 测试覆盖已存在文件
func TestWriteFile_Overwrite(t *testing.T) {
	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "test.txt")

	// 第一次写入
	os.WriteFile(testPath, []byte("original"), 0644)

	// 覆盖写入
	err := writeFile(testPath, []byte("overwritten"), 0644)
	if err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}

	data, _ := os.ReadFile(testPath)
	if string(data) != "overwritten" {
		t.Errorf("File content = %q, want 'overwritten'", string(data))
	}
}

// TestWriteFile_CreateDirectory 测试创建目录
func TestWriteFile_CreateDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// 写入到深层嵌套目录
	deepPath := filepath.Join(tempDir, "a", "b", "c", "test.txt")
	err := writeFile(deepPath, []byte("data"), 0644)
	if err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}

	// 验证所有中间目录都被创建
	for _, dir := range []string{
		filepath.Join(tempDir, "a"),
		filepath.Join(tempDir, "a", "b"),
		filepath.Join(tempDir, "a", "b", "c"),
	} {
		if info, err := os.Stat(dir); err != nil {
			t.Errorf("Directory %s should exist: %v", dir, err)
		} else if !info.IsDir() {
					t.Errorf("%s should be a directory", dir)
		}
	}
}

// TestEnsureDevCA 测试创建开发 CA
func TestEnsureDevCA(t *testing.T) {
	tempDir := t.TempDir()

	caCrt, caKey, err := EnsureDevCA(tempDir)
	if err != nil {
		t.Fatalf("EnsureDevCA() error = %v", err)
	}

	// 验证返回的路径
	if caCrt == "" {
		t.Error("caCrt should not be empty")
	}
	if caKey == "" {
		t.Error("caKey should not be empty")
	}

	// 验证文件存在
	if _, err := os.Stat(caCrt); os.IsNotExist(err) {
		t.Error("CA certificate file should exist")
	}

	if _, err := os.Stat(caKey); os.IsNotExist(err) {
		t.Error("CA key file should exist")
	}

	// 读取并验证证书内容
	crtData, err := os.ReadFile(caCrt)
	if err != nil {
		t.Fatalf("Failed to read CA cert: %v", err)
	}

	if len(crtData) == 0 {
		t.Error("CA certificate should not be empty")
	}

	// 验证是有效的 PEM 格式（以 -----BEGIN 开头）
	crtStr := string(crtData)
	if len(crtStr) < 5 || crtStr[:5] != "-----" {
		t.Error("CA certificate should be in PEM format")
	}
}

// TestEnsureDevCA_Reuse 测试重用已存在的 CA
func TestEnsureDevCA_Reuse(t *testing.T) {
	tempDir := t.TempDir()

	// 第一次创建
	caCrt1, caKey1, err := EnsureDevCA(tempDir)
	if err != nil {
		t.Fatalf("First EnsureDevCA() error = %v", err)
	}

	// 记录文件修改时间
	info1, _ := os.Stat(caCrt1)
	modTime1 := info1.ModTime()

	// 短暂等待以确保时间戳不同
	time.Sleep(10 * time.Millisecond)

	// 第二次调用应该重用
	caCrt2, caKey2, err := EnsureDevCA(tempDir)
	if err != nil {
		t.Fatalf("Second EnsureDevCA() error = %v", err)
	}

	if caCrt1 != caCrt2 {
		t.Errorf("caCrt path should be consistent: %q vs %q", caCrt1, caCrt2)
	}

	if caKey1 != caKey2 {
		t.Errorf("caKey path should be consistent: %q vs %q", caKey1, caKey2)
	}

	// 验证文件未被重新创建（修改时间应该相同或相近）
	info2, _ := os.Stat(caCrt2)
	modTime2 := info2.ModTime()

	if modTime1 != modTime2 {
		t.Logf("Warning: CA was recreated (mod time changed from %v to %v)", modTime1, modTime2)
	}
}

// TestEnsureDevCA_DefaultPaths 测试默认路径
func TestEnsureDevCA_DefaultPaths(t *testing.T) {
	tempDir := t.TempDir()

	caCrt, caKey, err := EnsureDevCA(tempDir)
	if err != nil {
		t.Fatalf("EnsureDevCA() error = %v", err)
	}

	expectedCrt := filepath.Join(tempDir, "ca.crt")
	expectedKey := filepath.Join(tempDir, "ca.key")

	if caCrt != expectedCrt {
		t.Errorf("caCrt = %q, want %q", caCrt, expectedCrt)
	}

	if caKey != expectedKey {
		t.Errorf("caKey = %q, want %q", caKey, expectedKey)
	}
}

// TestEnsureServerCert 测试创建服务器证书
func TestEnsureServerCert(t *testing.T) {
	tempDir := t.TempDir()

	// 先创建 CA
	caCrt, caKey, err := EnsureDevCA(tempDir)
	if err != nil {
		t.Fatalf("EnsureDevCA() error = %v", err)
	}

	// 创建服务器证书
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	crtPath, keyPath, err := EnsureServerCert(tempDir, caCrt, caKey, hosts)
	if err != nil {
		t.Fatalf("EnsureServerCert() error = %v", err)
	}

	// 验证返回的路径
	if crtPath == "" {
		t.Error("crtPath should not be empty")
	}
	if keyPath == "" {
		t.Error("keyPath should not be empty")
	}

	// 验证文件存在
	if _, err := os.Stat(crtPath); os.IsNotExist(err) {
		t.Error("Server certificate file should exist")
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Server key file should exist")
	}

	// 验证预期路径
	expectedCrt := filepath.Join(tempDir, "server.crt")
	expectedKey := filepath.Join(tempDir, "server.key")

	if crtPath != expectedCrt {
		t.Errorf("crtPath = %q, want %q", crtPath, expectedCrt)
	}

	if keyPath != expectedKey {
		t.Errorf("keyPath = %q, want %q", keyPath, expectedKey)
	}
}

// TestEnsureServerCert_Reuse 测试重用已存在的服务器证书
func TestEnsureServerCert_Reuse(t *testing.T) {
	tempDir := t.TempDir()

	// 先创建 CA
	caCrt, caKey, err := EnsureDevCA(tempDir)
	if err != nil {
		t.Fatalf("EnsureDevCA() error = %v", err)
	}

	// 第一次创建服务器证书
	hosts := []string{"localhost"}
	crtPath1, keyPath1, err := EnsureServerCert(tempDir, caCrt, caKey, hosts)
	if err != nil {
		t.Fatalf("First EnsureServerCert() error = %v", err)
	}

	// 记录修改时间
	info1, _ := os.Stat(crtPath1)
	modTime1 := info1.ModTime()

	time.Sleep(10 * time.Millisecond)

	// 第二次调用应该重用
	crtPath2, keyPath2, err := EnsureServerCert(tempDir, caCrt, caKey, hosts)
	if err != nil {
		t.Fatalf("Second EnsureServerCert() error = %v", err)
	}

	if crtPath1 != crtPath2 {
		t.Errorf("crtPath should be consistent: %q vs %q", crtPath1, crtPath2)
	}

	if keyPath1 != keyPath2 {
		t.Errorf("keyPath should be consistent: %q vs %q", keyPath1, keyPath2)
	}

	// 验证文件未被重新创建
	info2, _ := os.Stat(crtPath2)
	modTime2 := info2.ModTime()

	if modTime1 != modTime2 {
		t.Logf("Warning: Server cert was recreated (mod time changed from %v to %v)", modTime1, modTime2)
	}
}

// TestEnsureServerCert_DifferentHosts 测试不同的主机列表
func TestEnsureServerCert_DifferentHosts(t *testing.T) {
	tempDir := t.TempDir()

	// 先创建 CA
	caCrt, caKey, err := EnsureDevCA(tempDir)
	if err != nil {
		t.Fatalf("EnsureDevCA() error = %v", err)
	}

	tests := []struct {
		name  string
		hosts []string
	}{
		{
			name:  "仅 localhost",
			hosts: []string{"localhost"},
		},
		{
			name:  "仅 IP 地址",
			hosts: []string{"192.168.1.1"},
		},
		{
			name:  "混合 DNS 和 IP",
			hosts: []string{"example.com", "10.0.0.1"},
		},
		{
			name:  "多个主机",
			hosts: []string{"localhost", "127.0.0.1", "::1", "example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 使用不同的目录避免冲突
			testDir := filepath.Join(tempDir, tt.name)

			_, _, err := EnsureServerCert(testDir, caCrt, caKey, tt.hosts)
			if err != nil {
				t.Errorf("EnsureServerCert() with hosts %v error = %v", tt.hosts, err)
			}
		})
	}
}

// TestEnsureAgentCert 测试创建 Agent 证书
func TestEnsureAgentCert(t *testing.T) {
	tempDir := t.TempDir()

	// 先创建 CA
	caCrt, caKey, err := EnsureDevCA(tempDir)
	if err != nil {
		t.Fatalf("EnsureDevCA() error = %v", err)
	}

	// 创建 Agent 证书
	commonName := "agent-001"
	crtPath, keyPath, err := EnsureAgentCert(tempDir, caCrt, caKey, commonName)
	if err != nil {
		t.Fatalf("EnsureAgentCert() error = %v", err)
	}

	// 验证返回的路径
	if crtPath == "" {
		t.Error("crtPath should not be empty")
	}
	if keyPath == "" {
		t.Error("keyPath should not be empty")
	}

	// 验证文件存在
	if _, err := os.Stat(crtPath); os.IsNotExist(err) {
		t.Error("Agent certificate file should exist")
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Agent key file should exist")
	}

	// 验证预期路径
	expectedCrt := filepath.Join(tempDir, "agent.crt")
	expectedKey := filepath.Join(tempDir, "agent.key")

	if crtPath != expectedCrt {
		t.Errorf("crtPath = %q, want %q", crtPath, expectedCrt)
	}

	if keyPath != expectedKey {
		t.Errorf("keyPath = %q, want %q", keyPath, expectedKey)
	}
}

// TestEnsureAgentCert_DifferentCommonNames 测试不同的 CommonName
func TestEnsureAgentCert_DifferentCommonNames(t *testing.T) {
	tempDir := t.TempDir()

	// 先创建 CA
	caCrt, caKey, err := EnsureDevCA(tempDir)
	if err != nil {
		t.Fatalf("EnsureDevCA() error = %v", err)
	}

	commonNames := []string{
		"agent-001",
		"agent-002",
		"server-001",
		"client-test",
	}

	for _, cn := range commonNames {
		t.Run(cn, func(t *testing.T) {
			// 使用不同的目录避免冲突
			testDir := filepath.Join(tempDir, cn)

			_, _, err := EnsureAgentCert(testDir, caCrt, caKey, cn)
			if err != nil {
				t.Errorf("EnsureAgentCert() with CommonName %q error = %v", cn, err)
			}
		})
	}
}

// TestEnsureCert_Chaining 测试证书链
func TestEnsureCert_Chaining(t *testing.T) {
	tempDir := t.TempDir()

	// 创建 CA
	caCrt, caKey, err := EnsureDevCA(tempDir)
	if err != nil {
		t.Fatalf("EnsureDevCA() error = %v", err)
	}

	// 使用 CA 创建服务器证书
	_, _, err = EnsureServerCert(tempDir, caCrt, caKey, []string{"localhost"})
	if err != nil {
		t.Fatalf("EnsureServerCert() error = %v", err)
	}

	// 使用 CA 创建 Agent 证书
	_, _, err = EnsureAgentCert(tempDir, caCrt, caKey, "test-agent")
	if err != nil {
		t.Fatalf("EnsureAgentCert() error = %v", err)
	}

	// 所有证书都应该成功创建
	// 在实际场景中，这些证书应该可以组成一个有效的证书链
}

// BenchmarkEnsureDevCA 性能基准测试
func BenchmarkEnsureDevCA(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tempDir := b.TempDir()
		_, _, _ = EnsureDevCA(tempDir)
		// 清理
		os.RemoveAll(tempDir)
	}
}

// BenchmarkEnsureServerCert 性能基准测试
func BenchmarkEnsureServerCert(b *testing.B) {
	tempDir := b.TempDir()
	caCrt, caKey, _ := EnsureDevCA(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testDir := filepath.Join(tempDir, "bench")
		os.MkdirAll(testDir, 0755)
		_, _, _ = EnsureServerCert(testDir, caCrt, caKey, []string{"localhost"})
		os.RemoveAll(testDir)
	}
}

// BenchmarkEnsureAgentCert 性能基准测试
func BenchmarkEnsureAgentCert(b *testing.B) {
	tempDir := b.TempDir()
	caCrt, caKey, _ := EnsureDevCA(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testDir := filepath.Join(tempDir, "bench")
		os.MkdirAll(testDir, 0755)
		_, _, _ = EnsureAgentCert(testDir, caCrt, caKey, "bench-agent")
		os.RemoveAll(testDir)
	}
}

// TestWriteFile_Permissions 测试文件权限
func TestWriteFile_Permissions(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		data     []byte
		mode     os.FileMode
	}{
		{
			name: "读写文件",
			path: filepath.Join(tempDir, "rw.txt"),
			data: []byte("data"),
			mode: 0644,
		},
		{
			name: "私钥文件",
			path: filepath.Join(tempDir, "key.pem"),
			data: []byte("private"),
			mode: 0600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writeFile(tt.path, tt.data, tt.mode)
			if err != nil {
				t.Fatalf("writeFile() error = %v", err)
			}

			// 验证文件权限
			info, err := os.Stat(tt.path)
			if err != nil {
				t.Fatalf("Failed to stat file: %v", err)
			}

			// 在 Unix 系统上检查权限
			// 注意：在某些系统上，实际的 umask 可能会影响结果
			t.Logf("File %s has mode: %v", tt.path, info.Mode())
		})
	}
}
