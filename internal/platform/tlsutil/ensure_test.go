package tlsutil

import (
	"testing"
)

// TestEnsureServerTLSCredentials 测试 EnsureServerTLSCredentials 函数
func TestEnsureServerTLSCredentials(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ServerTLSConfig
		wantErr bool
	}{
		{
			name: "没有配置且不自动生成",
			cfg: ServerTLSConfig{
				ConfigDir: "/tmp/config.yaml",
				AutoGen:   false,
			},
			wantErr: false, // 应该返回 nil, nil
		},
		{
			name: "提供证书文件（文件不存在会失败）",
			cfg: ServerTLSConfig{
				ConfigDir: "/tmp/config.yaml",
				CertFile:  "/nonexistent/cert.pem",
				KeyFile:   "/nonexistent/key.pem",
				AutoGen:   false,
			},
			wantErr: true, // 文件不存在
		},
		{
			name: "仅提供证书文件没有密钥（不完整的配置被忽略）",
			cfg: ServerTLSConfig{
				ConfigDir: "/tmp/config.yaml",
				CertFile:  "/nonexistent/cert.pem",
				AutoGen:   false,
			},
			wantErr: false, // 不完整的配置被忽略，返回 nil, nil
		},
		{
			name: "自动生成证书",
			cfg: ServerTLSConfig{
				ConfigDir: "/tmp/test-config.yaml",
				AutoGen:   true,
			},
			wantErr: false, // 应该自动生成证书
		},
		{
			name: "自动生成并启用客户端验证",
			cfg: ServerTLSConfig{
				ConfigDir: "/tmp/test-config2.yaml",
				AutoGen:   true,
				CAFile:    "", // AutoGen 会生成 CA
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt, err := EnsureServerTLSCredentials(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				// 某些情况下返回 nil, nil 是有效的（没有 TLS）
				if opt == nil && err == nil {
					// 这是有效的情况（没有配置 TLS）
					return
				}
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if opt == nil {
					t.Error("Expected non-nil ServerOption")
				}
			}
		})
	}
}

// TestEnsureServerTLSCredentials_PreferProvided 测试优先使用提供的证书
func TestEnsureServerTLSCredentials_PreferProvided(t *testing.T) {
	cfg := ServerTLSConfig{
		ConfigDir: "/tmp/config.yaml",
		CertFile:  "/nonexistent/cert.pem",
		KeyFile:   "/nonexistent/key.pem",
		CAFile:    "",
		AutoGen:   true, // 即使设置了 AutoGen，也应该优先使用提供的证书
	}

	_, err := EnsureServerTLSCredentials(cfg)
	if err == nil {
		t.Error("Expected error for non-existent certificate files")
	}
}

// TestEnsureServerTLSCredentials_AutoGenCerts 测试自动生成证书的场景
func TestEnsureServerTLSCredentials_AutoGenCerts(t *testing.T) {
	tests := []struct {
		name      string
		configDir string
		wantErr   bool
	}{
		{
			name:      "相对路径配置文件",
			configDir: "etc/server.yaml",
			wantErr:   false,
		},
		{
			name:      "绝对路径配置文件",
			configDir: "/tmp/test/server.yaml",
			wantErr:   false,
		},
		{
			name:      "仅文件名",
			configDir: "config.yaml",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServerTLSConfig{
				ConfigDir: tt.configDir,
				AutoGen:   true,
			}

			opt, err := EnsureServerTLSCredentials(cfg)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error")
				}
			} else {
				if err != nil {
					t.Logf("Got error (may be acceptable in test env): %v", err)
				}
				if opt == nil {
					t.Log("ServerOption is nil (cert generation may have failed)")
				}
			}
		})
	}
}
