package tlsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/devcert"
)

// TestClientTLSFromConfig 测试从配置创建客户端 TLS
func TestClientTLSFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ClientTLSConfig
		wantErr bool
	}{
		{
			name: "最小配置 - 仅 CA",
			cfg: ClientTLSConfig{
				CAFile:             "testdata/ca.crt",
				InsecureSkipVerify: false,
			},
			wantErr: true, // 文件不存在
		},
		{
			name: "完整 mTLS 配置",
			cfg: ClientTLSConfig{
				CertFile:           "testdata/client.crt",
				KeyFile:            "testdata/client.key",
				CAFile:             "testdata/ca.crt",
				ServerName:         "localhost",
				InsecureSkipVerify: false,
			},
			wantErr: true, // 文件不存在
		},
		{
			name: "不完整配置 - 仅有证书",
			cfg: ClientTLSConfig{
				CertFile: "testdata/client.crt",
			},
			wantErr: true, // 缺少 key_file
		},
		{
			name: "不完整配置 - 仅有密钥",
			cfg: ClientTLSConfig{
				KeyFile: "testdata/client.key",
			},
			wantErr: true, // 缺少 cert_file
		},
		{
			name: "跳过验证",
			cfg: ClientTLSConfig{
				InsecureSkipVerify: true,
			},
			wantErr: false, // 跳过验证应该成功
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ClientTLSFromConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClientTLSFromConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestClientTLS 测试客户端 TLS 创建
func TestClientTLS(t *testing.T) {
	tests := []struct {
		name     string
		certFile string
		keyFile  string
		caFile   string
		wantErr  bool
	}{
		{
			name:     "完整参数",
			certFile: "testdata/client.crt",
			keyFile:  "testdata/client.key",
			caFile:   "testdata/ca.crt",
			wantErr:  true, // 文件不存在
		},
		{
			name:     "空证书文件",
			certFile: "",
			keyFile:  "testdata/client.key",
			caFile:   "testdata/ca.crt",
			wantErr:  true, // 参数缺失
		},
		{
			name:     "空密钥文件",
			certFile: "testdata/client.crt",
			keyFile:  "",
			caFile:   "testdata/ca.crt",
			wantErr:  true, // 参数缺失
		},
		{
			name:     "空 CA 文件",
			certFile: "testdata/client.crt",
			keyFile:  "testdata/client.key",
			caFile:   "",
			wantErr:  true, // 参数缺失
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ClientTLS(tt.certFile, tt.keyFile, tt.caFile, "localhost")
			if (err != nil) != tt.wantErr {
				t.Errorf("ClientTLS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestServerTLS 测试服务器 TLS 创建
func TestServerTLS(t *testing.T) {
	tests := []struct {
		name          string
		certFile      string
		keyFile       string
		caFile        string
		requireClient bool
		wantErr       bool
	}{
		{
			name:          "基本服务器证书",
			certFile:      "testdata/server.crt",
			keyFile:       "server.key",
			caFile:        "",
			requireClient: false,
			wantErr:       true, // 文件不存在
		},
		{
			name:          "mTLS 模式",
			certFile:      "testdata/server.crt",
			keyFile:       "server.key",
			caFile:        "testdata/ca.crt",
			requireClient: true,
			wantErr:       true, // 文件不存在
		},
		{
			name:          "mTLS 但无 CA",
			certFile:      "testdata/server.crt",
			keyFile:       "server.key",
			caFile:        "",
			requireClient: true,
			wantErr:       true, // 需要 CA
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ServerTLS(tt.certFile, tt.keyFile, tt.caFile, tt.requireClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("ServerTLS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestClientTLSConfig 结构体测试
func TestClientTLSConfig(t *testing.T) {
	cfg := ClientTLSConfig{
		CertFile:           "client.crt",
		KeyFile:            "client.key",
		CAFile:             "ca.crt",
		ServerName:         "example.com",
		InsecureSkipVerify: false,
	}

	if cfg.CertFile != "client.crt" {
		t.Errorf("CertFile = %q, want 'client.crt'", cfg.CertFile)
	}

	if cfg.KeyFile != "client.key" {
		t.Errorf("KeyFile = %q, want 'client.key'", cfg.KeyFile)
	}

	if cfg.CAFile != "ca.crt" {
		t.Errorf("CAFile = %q, want 'ca.crt'", cfg.CAFile)
	}

	if cfg.ServerName != "example.com" {
		t.Errorf("ServerName = %q, want 'example.com'", cfg.ServerName)
	}

	if cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false")
	}
}

// TestClientTLSFromConfig_SkipVerify 测试跳过验证
func TestClientTLSFromConfig_SkipVerify(t *testing.T) {
	cfg := ClientTLSConfig{
		InsecureSkipVerify: true,
	}

	creds, err := ClientTLSFromConfig(cfg)
	if err != nil {
		t.Fatalf("ClientTLSFromConfig() with skip verify error = %v", err)
	}

	if creds == nil {
		t.Error("ClientTLSFromConfig should return credentials even when skipping verification")
	}
}

// TestClientTLSFromConfig_EmptyConfig 测试空配置
func TestClientTLSFromConfig_EmptyConfig(t *testing.T) {
	cfg := ClientTLSConfig{}

	creds, err := ClientTLSFromConfig(cfg)
	if err != nil {
		t.Fatalf("ClientTLSFromConfig() with empty config error = %v", err)
	}

	if creds == nil {
		t.Error("ClientTLSFromConfig should return credentials with empty config")
	}
}

// TestClientTLS_MissingCertOrKey 测试缺少证书或密钥
func TestClientTLS_MissingCertOrKey(t *testing.T) {
	tests := []struct {
		name     string
		certFile string
		keyFile  string
		caFile   string
	}{
		{
			name:     "有证书无密钥",
			certFile: "cert.pem",
			keyFile:  "",
			caFile:   "ca.pem",
		},
		{
			name:     "无证书有密钥",
			certFile: "",
			keyFile:  "key.pem",
			caFile:   "ca.pem",
		},
		{
			name:     "两者都为空",
			certFile: "",
			keyFile:  "",
			caFile:   "ca.pem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ClientTLS(tt.certFile, tt.keyFile, tt.caFile, "localhost")
			if err == nil {
				t.Error("ClientTLS() should require both cert and key files")
			}
		})
	}
}

// TestServerTLS_InvalidPaths 测试无效路径
func TestServerTLS_InvalidPaths(t *testing.T) {
	tests := []struct {
		name     string
		certFile string
		keyFile  string
		caFile   string
	}{
		{
			name:     "无效证书路径",
			certFile: "/nonexistent/cert.pem",
			keyFile:  "/nonexistent/key.pem",
			caFile:   "",
		},
		{
			name:     "无效密钥路径",
			certFile: "/nonexistent/cert.pem",
			keyFile:  "/nonexistent/key.pem",
			caFile:   "/nonexistent/ca.pem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ServerTLS(tt.certFile, tt.keyFile, tt.caFile, false)
			if err == nil {
				t.Error("ServerTLS() should fail with invalid paths")
			}
		})
	}
}

// BenchmarkClientTLSFromConfig 性能基准测试
func BenchmarkClientTLSFromConfig(b *testing.B) {
	cfg := ClientTLSConfig{
		InsecureSkipVerify: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClientTLSFromConfig(cfg)
	}
}

// BenchmarkServerTLS 性能基准测试
func BenchmarkServerTLS(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 注意：这会因为文件不存在而失败，但可以测试调用路径
		ServerTLS("/nonexistent/cert.pem", "/nonexistent/key.pem", "", false)
	}
}

// TestClientTLS_ConfigVariations 测试不同配置组合
func TestClientTLS_ConfigVariations(t *testing.T) {
	configs := []struct {
		name     string
		cfg      ClientTLSConfig
		checkErr bool
	}{
		{
			name: "仅 InsecureSkipVerify",
			cfg: ClientTLSConfig{
				InsecureSkipVerify: true,
			},
			checkErr: false,
		},
		{
			name: "CA + InsecureSkipVerify",
			cfg: ClientTLSConfig{
				CAFile:             "/nonexistent/ca.pem",
				InsecureSkipVerify: true,
			},
			checkErr: true, // 文件不存在
		},
		{
			name: "仅 ServerName",
			cfg: ClientTLSConfig{
				ServerName: "example.com",
			},
			checkErr: false,
		},
		{
			name: "ServerName + SkipVerify",
			cfg: ClientTLSConfig{
				ServerName:         "example.com",
				InsecureSkipVerify: true,
			},
			checkErr: false,
		},
	}

	for _, tt := range configs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ClientTLSFromConfig(tt.cfg)
			hasErr := err != nil

			if tt.checkErr && !hasErr {
				t.Error("ClientTLSFromConfig() should return error")
			}
		})
	}
}

// TestServerTLS_ClientCertVerification 测试客户端证书验证配置
func TestServerTLS_ClientCertVerification(t *testing.T) {
	tests := []struct {
		name          string
		requireClient bool
		caFile        string
		wantErr       bool
	}{
		{
			name:          "不需要客户端证书",
			requireClient: false,
			caFile:        "",
			wantErr:       true, // 证书文件不存在
		},
		{
			name:          "需要客户端证书但无 CA",
			requireClient: true,
			caFile:        "",
			wantErr:       true, // 需要 CA
		},
		{
			name:          "需要客户端证书有 CA",
			requireClient: true,
			caFile:        "/nonexistent/ca.pem",
			wantErr:       true, // 文件不存在
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ServerTLS("/nonexistent/cert.pem", "/nonexistent/key.pem", tt.caFile, tt.requireClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("ServerTLS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestServerTLS_WithValidCertificates tests ServerTLS with valid test certificates
func TestServerTLS_WithValidCertificates(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate test CA and server certificates
	caCrt, caKey, err := devcert.EnsureDevCA(tmpDir)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	serverCrt, serverKey, err := devcert.EnsureServerCert(tmpDir, caCrt, caKey, []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatalf("Failed to generate server cert: %v", err)
	}

	t.Run("NoClientAuth", func(t *testing.T) {
		creds, err := ServerTLS(serverCrt, serverKey, "", false)
		if err != nil {
			t.Errorf("ServerTLS() with valid certs and no client auth failed: %v", err)
		}
		if creds == nil {
			t.Error("ServerTLS() should return credentials with valid certs")
		}
	})

	t.Run("WithClientAuth", func(t *testing.T) {
		creds, err := ServerTLS(serverCrt, serverKey, caCrt, true)
		if err != nil {
			t.Errorf("ServerTLS() with valid certs and client auth failed: %v", err)
		}
		if creds == nil {
			t.Error("ServerTLS() should return credentials with valid certs and client auth")
		}
	})
}

// TestServerTLS_InvalidCertPath tests ServerTLS with invalid certificate path
func TestServerTLS_InvalidCertPath(t *testing.T) {
	_, err := ServerTLS("/nonexistent/cert.pem", "/nonexistent/key.pem", "", false)
	if err == nil {
		t.Error("ServerTLS() should fail with invalid certificate path")
	}
	if !containsString(err.Error(), "load keypair") {
		t.Errorf("ServerTLS() error should mention 'load keypair', got: %v", err)
	}
}

// TestServerTLS_WithClient_NoCA tests ServerTLS with requireClient=true but no CA file
func TestServerTLS_WithClient_NoCA(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate test CA and server certificates
	caCrt, caKey, err := devcert.EnsureDevCA(tmpDir)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	serverCrt, serverKey, err := devcert.EnsureServerCert(tmpDir, caCrt, caKey, []string{"localhost"})
	if err != nil {
		t.Fatalf("Failed to generate server cert: %v", err)
	}

	// Request mTLS without providing CA file
	_, err = ServerTLS(serverCrt, serverKey, "", true)
	if err == nil {
		t.Error("ServerTLS() should fail when requireClient is true but no CA file is provided")
	}
	if !containsString(err.Error(), "ca certificate required") {
		t.Errorf("ServerTLS() error should mention 'ca certificate required', got: %v", err)
	}
}

// TestServerTLS_InvalidPEM tests ServerTLS with invalid PEM format
func TestServerTLS_InvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invalid certificate file
	invalidCert := filepath.Join(tmpDir, "invalid.pem")
	invalidKey := filepath.Join(tmpDir, "invalid.key")

	writeFile(t, invalidCert, []byte("INVALID PEM DATA"))
	writeFile(t, invalidKey, []byte("INVALID PEM DATA"))

	_, err := ServerTLS(invalidCert, invalidKey, "", false)
	if err == nil {
		t.Error("ServerTLS() should fail with invalid PEM format")
	}
}

// TestServerTLS_MissingCertFile tests ServerTLS when cert file is missing but key exists
func TestServerTLS_MissingCertFile(t *testing.T) {
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "key.pem")
	writeFile(t, keyFile, []byte("some data"))

	_, err := ServerTLS("/nonexistent/cert.pem", keyFile, "", false)
	if err == nil {
		t.Error("ServerTLS() should fail when cert file is missing")
	}
}

// TestServerTLS_MissingKeyFile tests ServerTLS when key file is missing but cert exists
func TestServerTLS_MissingKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	writeFile(t, certFile, []byte("some data"))

	_, err := ServerTLS(certFile, "/nonexistent/key.pem", "", false)
	if err == nil {
		t.Error("ServerTLS() should fail when key file is missing")
	}
}

// TestServerTLS_ClientAuthWithInvalidCA tests ServerTLS with mTLS but invalid CA
func TestServerTLS_ClientAuthWithInvalidCA(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate test CA and server certificates
	caCrt, caKey, err := devcert.EnsureDevCA(tmpDir)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	serverCrt, serverKey, err := devcert.EnsureServerCert(tmpDir, caCrt, caKey, []string{"localhost"})
	if err != nil {
		t.Fatalf("Failed to generate server cert: %v", err)
	}

	// Use non-existent CA file
	_, err = ServerTLS(serverCrt, serverKey, "/nonexistent/ca.pem", true)
	if err == nil {
		t.Error("ServerTLS() should fail with invalid CA file for mTLS")
	}
}

// writeFile is a helper to write test data
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
}

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestClientTLSFromConfig_InsecureSkipVerify tests client TLS with skip verify
func TestClientTLSFromConfig_InsecureSkipVerify(t *testing.T) {
	cfg := ClientTLSConfig{
		InsecureSkipVerify: true,
	}

	creds, err := ClientTLSFromConfig(cfg)
	if err != nil {
		t.Errorf("ClientTLSFromConfig() with skip verify failed: %v", err)
	}
	if creds == nil {
		t.Error("ClientTLSFromConfig() should return credentials even when skipping verification")
	}
}

// TestClientTLSFromConfig_WithValidFiles tests client TLS with valid certificate files
func TestClientTLSFromConfig_WithValidFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate test CA and agent certificates
	caCrt, caKey, err := devcert.EnsureDevCA(tmpDir)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	agentCrt, agentKey, err := devcert.EnsureAgentCert(tmpDir, caCrt, caKey, "test-agent")
	if err != nil {
		t.Fatalf("Failed to generate agent cert: %v", err)
	}

	cfg := ClientTLSConfig{
		CertFile:   agentCrt,
		KeyFile:    agentKey,
		CAFile:     caCrt,
		ServerName: "localhost",
	}

	creds, err := ClientTLSFromConfig(cfg)
	if err != nil {
		t.Errorf("ClientTLSFromConfig() with valid files failed: %v", err)
	}
	if creds == nil {
		t.Error("ClientTLSFromConfig() should return credentials with valid files")
	}
}

// TestClientTLS_BothCertAndKeyRequired tests that both cert and key are required
func TestClientTLS_BothCertAndKeyRequired(t *testing.T) {
	tests := []struct {
		name     string
		certFile string
		keyFile  string
		caFile   string
	}{
		{
			name:     "missing_cert",
			certFile: "",
			keyFile:  "key.pem",
			caFile:   "ca.pem",
		},
		{
			name:     "missing_key",
			certFile: "cert.pem",
			keyFile:  "",
			caFile:   "ca.pem",
		},
		{
			name:     "missing_both",
			certFile: "",
			keyFile:  "",
			caFile:   "ca.pem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ClientTLS(tt.certFile, tt.keyFile, tt.caFile, "localhost")
			if err == nil {
				t.Error("ClientTLS() should require both cert and key files")
			}
		})
	}
}
