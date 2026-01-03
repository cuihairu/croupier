package tlsutil

import (
	"testing"
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
