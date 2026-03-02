package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// TestFileExists 测试文件存在检查
func TestFileExists(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantErr     bool
		errContains string
	}{
		{
			name:        "空路径",
			path:        "",
			wantErr:     true,
			errContains: "empty path",
		},
		{
			name:    "文件不存在",
			path:    "/nonexistent/file.txt",
			wantErr: true,
		},
		{
			name:    "存在的主机文件",
			path:    os.Args[0], // 当前测试二进制文件
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fileExists(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("fileExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error should contain %q, got %q", tt.errContains, err.Error())
				}
			}
		})
	}
}

// TestValidateTLS 测试 TLS 证书验证
func TestValidateTLS(t *testing.T) {
	// 创建临时证书文件用于测试
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")
	caFile := filepath.Join(tmpDir, "ca.pem")

	// 创建空文件
	os.WriteFile(certFile, []byte("cert"), 0644)
	os.WriteFile(keyFile, []byte("key"), 0644)
	os.WriteFile(caFile, []byte("ca"), 0644)

	tests := []struct {
		name    string
		cert    string
		key     string
		ca      string
		strict  bool
		wantErr bool
	}{
		{
			name:    "strict 模式 - 所有文件存在",
			cert:    certFile,
			key:     keyFile,
			ca:      caFile,
			strict:  true,
			wantErr: false,
		},
		{
			name:    "strict 模式 - cert 不存在",
			cert:    "/nonexistent/cert",
			key:     keyFile,
			ca:      caFile,
			strict:  true,
			wantErr: true,
		},
		{
			name:    "strict 模式 - key 不存在",
			cert:    certFile,
			key:     "/nonexistent/key",
			ca:      caFile,
			strict:  true,
			wantErr: true,
		},
		{
			name:    "strict 模式 - ca 不存在",
			cert:    certFile,
			key:     keyFile,
			ca:      "/nonexistent/ca",
			strict:  true,
			wantErr: true,
		},
		{
			name:    "非 strict 模式 - 所有空",
			cert:    "",
			key:     "",
			ca:      "",
			strict:  false,
			wantErr: false,
		},
		{
			name:    "非 strict 模式 - cert 存在",
			cert:    certFile,
			key:     "",
			ca:      "",
			strict:  false,
			wantErr: false,
		},
		{
			name:    "非 strict 模式 - cert 不存在",
			cert:    "/nonexistent/cert",
			key:     "",
			ca:      "",
			strict:  false,
			wantErr: true,
		},
		{
			name:    "非 strict 模式 - key 存在",
			cert:    "",
			key:     keyFile,
			ca:      "",
			strict:  false,
			wantErr: false,
		},
		{
			name:    "非 strict 模式 - ca 存在",
			cert:    "",
			key:     "",
			ca:      caFile,
			strict:  false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTLS(tt.cert, tt.key, tt.ca, tt.strict)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTLS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateAddr 测试地址验证
func TestValidateAddr(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{
			name:    "有效地址 - localhost",
			addr:    "localhost:18780",
			wantErr: false,
		},
		{
			name:    "有效地址 - 127.0.0.1",
			addr:    "127.0.0.1:9090",
			wantErr: false,
		},
		{
			name:    "有效地址 - 0.0.0.0",
			addr:    "0.0.0.0:8443",
			wantErr: false,
		},
		{
			name:    "空地址",
			addr:    "",
			wantErr: true,
		},
		{
			name:    "无效地址 - 缺少端口",
			addr:    "localhost",
			wantErr: true,
		},
		{
			name:    "无效地址 - 缺少冒号",
			addr:    "localhost8080",
			wantErr: true,
		},
		{
			name:    "无效地址 - 端口超出范围",
			addr:    "localhost:99999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddr(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAddr() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateServerConfig 测试服务器配置验证
func TestValidateServerConfig(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	rbacFile := filepath.Join(tmpDir, "rbac.yaml")
	usersFile := filepath.Join(tmpDir, "users.yaml")
	gamesFile := filepath.Join(tmpDir, "games.yaml")
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")
	caFile := filepath.Join(tmpDir, "ca.pem")

	// 创建空文件（使用有效的 RBAC 和 users 格式）
	os.WriteFile(rbacFile, []byte(`{"allow": {}}`), 0644)
	os.WriteFile(usersFile, []byte(`[]`), 0644)
	os.WriteFile(gamesFile, []byte("games: []"), 0644)
	os.WriteFile(certFile, []byte("cert"), 0644)
	os.WriteFile(keyFile, []byte("key"), 0644)
	os.WriteFile(caFile, []byte("ca"), 0644)

	tests := []struct {
		name    string
		setup   func(*viper.Viper)
		strict  bool
		wantErr bool
	}{
		{
			name: "有效配置 - 非 strict",
			setup: func(v *viper.Viper) {
				v.Set("server.addr", "localhost:8443")
				v.Set("server.http_addr", "localhost:18780")
				v.Set("server.cert", "")
				v.Set("server.key", "")
				v.Set("server.ca", "")
			},
			strict:  false,
			wantErr: false,
		},
		{
			name: "有效配置 - strict 模式",
			setup: func(v *viper.Viper) {
				v.Set("server.addr", "localhost:8443")
				v.Set("server.http_addr", "localhost:18780")
				v.Set("server.cert", certFile)
				v.Set("server.key", keyFile)
				v.Set("server.ca", caFile)
				v.Set("server.rbac_config", rbacFile)
				v.Set("server.users_config", usersFile)
				v.Set("server.games_config", gamesFile)
			},
			strict:  true,
			wantErr: false,
		},
		{
			name: "无效 addr",
			setup: func(v *viper.Viper) {
				v.Set("server.addr", "")
				v.Set("server.http_addr", "localhost:18780")
			},
			strict:  false,
			wantErr: true,
		},
		{
			name: "无效 http_addr",
			setup: func(v *viper.Viper) {
				v.Set("server.addr", "localhost:8443")
				v.Set("server.http_addr", "")
			},
			strict:  false,
			wantErr: true,
		},
		{
			name: "无效 edge_addr",
			setup: func(v *viper.Viper) {
				v.Set("server.addr", "localhost:8443")
				v.Set("server.http_addr", "localhost:18780")
				v.Set("server.edge_addr", "invalid")
			},
			strict:  false,
			wantErr: true,
		},
		{
			name: "strict 模式缺少 rbac_config",
			setup: func(v *viper.Viper) {
				v.Set("server.addr", "localhost:8443")
				v.Set("server.http_addr", "localhost:18780")
				v.Set("server.cert", certFile)
				v.Set("server.key", keyFile)
				v.Set("server.ca", caFile)
			},
			strict:  true,
			wantErr: true,
		},
		{
			name: "使用 server section",
			setup: func(v *viper.Viper) {
				v.Set("addr", "localhost:8443")
				v.Set("http_addr", "localhost:18780")
			},
			strict:  false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			tt.setup(v)
			err := ValidateServerConfig(v, tt.strict)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServerConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateAgentConfig 测试 Agent 配置验证
func TestValidateAgentConfig(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")
	caFile := filepath.Join(tmpDir, "ca.pem")

	os.WriteFile(certFile, []byte("cert"), 0644)
	os.WriteFile(keyFile, []byte("key"), 0644)
	os.WriteFile(caFile, []byte("ca"), 0644)

	tests := []struct {
		name    string
		setup   func(*viper.Viper)
		strict  bool
		wantErr bool
	}{
		{
			name: "有效配置 - 非 strict",
			setup: func(v *viper.Viper) {
				v.Set("agent.local_addr", "localhost:19090")
				v.Set("agent.server_addr", "localhost:8443")
				v.Set("agent.http_addr", "localhost:18780")
			},
			strict:  false,
			wantErr: false,
		},
		{
			name: "有效配置 - strict 模式",
			setup: func(v *viper.Viper) {
				v.Set("agent.local_addr", "localhost:19090")
				v.Set("agent.server_addr", "localhost:8443")
				v.Set("agent.http_addr", "localhost:18780")
				v.Set("agent.cert", certFile)
				v.Set("agent.key", keyFile)
				v.Set("agent.ca", caFile)
			},
			strict:  true,
			wantErr: false,
		},
		{
			name: "使用 core_addr (向后兼容)",
			setup: func(v *viper.Viper) {
				v.Set("agent.local_addr", "localhost:19090")
				v.Set("agent.core_addr", "localhost:8443")
				v.Set("agent.http_addr", "localhost:18780")
			},
			strict:  false,
			wantErr: false,
		},
		{
			name: "server_addr 优先于 core_addr",
			setup: func(v *viper.Viper) {
				v.Set("agent.local_addr", "localhost:19090")
				v.Set("agent.server_addr", "localhost:8443")
				v.Set("agent.core_addr", "localhost:9999")
				v.Set("agent.http_addr", "localhost:18780")
			},
			strict:  false,
			wantErr: false,
		},
		{
			name: "缺少 server_addr 和 core_addr",
			setup: func(v *viper.Viper) {
				v.Set("agent.local_addr", "localhost:19090")
				v.Set("agent.http_addr", "localhost:18780")
			},
			strict:  false,
			wantErr: true,
		},
		{
			name: "无效 local_addr",
			setup: func(v *viper.Viper) {
				v.Set("agent.local_addr", "")
				v.Set("agent.server_addr", "localhost:8443")
				v.Set("agent.http_addr", "localhost:18780")
			},
			strict:  false,
			wantErr: true,
		},
		{
			name: "使用 agent section",
			setup: func(v *viper.Viper) {
				v.Set("local_addr", "localhost:19090")
				v.Set("server_addr", "localhost:8443")
				v.Set("http_addr", "localhost:18780")
			},
			strict:  false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			tt.setup(v)
			err := ValidateAgentConfig(v, tt.strict)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAgentConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
