package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultValidator_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "development",
					Debug:   true,
				},
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 9090,
					},
				},
				Database: DatabaseConfig{
					Enabled: true,
					Primary: DatabaseInstance{
						Host:     "localhost",
						Port:     5432,
						Database: "testdb",
						Username: "testuser",
						Password: "testpass",
						SSLMode:  "disable",
					},
					ConnectionPool: ConnectionPoolConfig{
						MaxOpenConns:    25,
						MaxIdleConns:    5,
						ConnMaxLifetime: time.Hour,
					},
				},
				Security: SecurityConfig{
					JWT: JWTConfig{
						Enabled:       true,
						Secret:        "this-is-a-very-long-secret-key-that-is-at-least-32-characters",
						Expiry:        time.Hour,
						RefreshExpiry: 24 * time.Hour,
					},
					PasswordPolicy: PasswordPolicyConfig{
						MinLength:        12,
						RequireUppercase: true,
						RequireLowercase: true,
						RequireNumbers:   true,
					},
				},
				Observability: ObservabilityConfig{
					Logging: LoggingConfig{
						Enabled: true,
						Level:   "info",
						Format:  "json",
					},
					Metrics: MetricsConfig{
						Enabled: true,
						Port:    9090,
						Path:    "/metrics",
					},
				},
				Business: BusinessConfig{
					Games: GamesConfig{
						MaxConcurrentGames: 1000,
						MaxPlayersPerGame:  10,
						DefaultGameTimeout: time.Hour,
					},
					Functions: FunctionsConfig{
						Registry: RegistryConfig{
							MaxSize: 10000,
						},
						Execution: ExecutionConfig{
							DefaultTimeout: 30 * time.Second,
							MaxTimeout:     5 * time.Minute,
						},
					},
					Jobs: JobsConfig{
						Queue: QueueConfig{
							MaxSize: 5000,
						},
						Retry: RetryConfig{
							MaxAttempts:  3,
							InitialDelay: time.Second,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid app name",
			config: &Config{
				App: AppConfig{
					Name:    "", // 空名称
					Version: "1.0.0",
					Env:     "development",
				},
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 9090,
					},
				},
			},
			wantErr: true,
			errMsg:  "应用名称不能为空",
		},
		{
			name: "invalid version format",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0", // 不符合语义化版本
					Env:     "development",
				},
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 9090,
					},
				},
			},
			wantErr: true,
			errMsg:  "应用版本必须符合语义化版本格式",
		},
		{
			name: "invalid port range",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "development",
				},
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 70000, // 超出范围
						GRPCPort: 9090,
					},
				},
			},
			wantErr: true,
			errMsg:  "HTTP端口必须在1-65535范围内",
		},
		{
			name: "production debug mode",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "production",
					Debug:   true, // 生产环境不应该启用调试模式
				},
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 9090,
					},
				},
			},
			wantErr: true,
			errMsg:  "生产环境不能启用调试模式",
		},
		{
			name: "database config validation",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "development",
				},
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 9090,
					},
				},
				Database: DatabaseConfig{
					Enabled: true,
					Primary: DatabaseInstance{
						Host:     "", // 缺少主机
						Port:     5432,
						Database: "testdb",
						Username: "testuser",
						Password: "testpass",
					},
				},
			},
			wantErr: true,
			errMsg:  "Primary数据库主机不能为空",
		},
		{
			name: "JWT config validation",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "development",
				},
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 9090,
					},
				},
				Security: SecurityConfig{
					JWT: JWTConfig{
						Enabled: true,
						Secret:  "short", // 密钥太短
						Expiry:  time.Hour,
					},
				},
			},
			wantErr: true,
			errMsg:  "JWT密钥长度不能少于32个字符",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewDefaultValidator()
			err := validator.Validate(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAppValidationRule(t *testing.T) {
	rule := &AppValidationRule{}

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid app config",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "development",
					Debug:   true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid app name with special chars",
			config: &Config{
				App: AppConfig{
					Name:    "test@app", // 包含无效字符
					Version: "1.0.0",
					Env:     "development",
				},
			},
			wantErr: true,
		},
		{
			name: "app name too long",
			config: &Config{
				App: AppConfig{
					Name:    string(make([]byte, 101)), // 101个字符
					Version: "1.0.0",
					Env:     "development",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid environment",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "invalid-env",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNetworkValidationRule(t *testing.T) {
	rule := &NetworkValidationRule{}

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid network config",
			config: &Config{
				Network: NetworkConfig{
					Server: ServerConfig{
						Host:     "localhost",
						HTTPPort: 8080,
						GRPCPort: 9090,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate ports",
			config: &Config{
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 8080, // 重复端口
					},
				},
			},
			wantErr: true,
		},
		{
			name: "TLS without cert file",
			config: &Config{
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 9090,
						TLS: TLSConfig{
							Enabled:  true,
							KeyFile:  "key.pem",
							CertFile: "", // 缺少证书文件
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid CORS origin",
			config: &Config{
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 9090,
						CORS: CORSConfig{
							Enabled:        true,
							AllowedOrigins: []string{"invalid-url"}, // 无效的URL
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCustomValidationRule(t *testing.T) {
	// 创建自定义验证规则
	customRule := NewCustomValidationRule(
		"CustomRule",
		"自定义验证规则",
		func(config *Config) error {
			if config.App.Name == "forbidden-name" {
				return assert.AnError
			}
			return nil
		},
	)

	assert.Equal(t, "CustomRule", customRule.Name())
	assert.Equal(t, "自定义验证规则", customRule.Description())

	// 测试正常情况
	config := &Config{
		App: AppConfig{
			Name:    "allowed-name",
			Version: "1.0.0",
			Env:     "development",
		},
	}
	err := customRule.Validate(config)
	assert.NoError(t, err)

	// 测试失败情况
	config.App.Name = "forbidden-name"
	err = customRule.Validate(config)
	assert.Error(t, err)
}

func TestValidationHelpers(t *testing.T) {
	// 测试端口验证
	assert.NoError(t, ValidatePort(8080))
	assert.Error(t, ValidatePort(0))
	assert.Error(t, ValidatePort(70000))

	// 测试URL验证
	assert.NoError(t, ValidateURL("https://example.com"))
	assert.Error(t, ValidateURL("invalid-url"))

	// 测试邮箱验证
	assert.NoError(t, ValidateEmail("test@example.com"))
	assert.Error(t, ValidateEmail("invalid-email"))

	// 测试持续时间验证
	assert.NoError(t, ValidateDuration(time.Hour))
	assert.Error(t, ValidateDuration(0))

	// 测试范围验证
	assert.NoError(t, ValidateRange(5, 1, 10, "test"))
	assert.Error(t, ValidateRange(0, 1, 10, "test"))
	assert.Error(t, ValidateRange(11, 1, 10, "test"))

	// 测试字符串枚举验证
	validValues := []string{"option1", "option2", "option3"}
	assert.NoError(t, ValidateStringEnum("option1", validValues, "test"))
	assert.Error(t, ValidateStringEnum("invalid", validValues, "test"))
}

func TestDefaultValidator_RuleManagement(t *testing.T) {
	validator := NewDefaultValidator()

	// 检查默认规则数量
	assert.Greater(t, len(validator.rules), 0)

	// 添加自定义规则
	customRule := NewCustomValidationRule(
		"TestRule",
		"测试规则",
		func(config *Config) error { return nil },
	)

	validator.AddRule(customRule)
	assert.Equal(t, customRule, validator.rules[len(validator.rules)-1])

	// 移除规则
	validator.RemoveRule("TestRule")
	found := false
	for _, rule := range validator.rules {
		if rule.Name() == "TestRule" {
			found = true
			break
		}
	}
	assert.False(t, found)
}

func TestDatabaseValidationRule(t *testing.T) {
	rule := &DatabaseValidationRule{}

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "disabled database",
			config: &Config{
				Database: DatabaseConfig{
					Enabled: false,
				},
			},
			wantErr: false,
		},
		{
			name: "valid database config",
			config: &Config{
				Database: DatabaseConfig{
					Enabled: true,
					Primary: DatabaseInstance{
						Host:     "localhost",
						Port:     5432,
						Database: "testdb",
						Username: "user",
						Password: "pass",
						SSLMode:  "disable",
					},
					ConnectionPool: ConnectionPoolConfig{
						MaxOpenConns:    100,
						MaxIdleConns:    10,
						ConnMaxLifetime: time.Hour,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid connection pool config",
			config: &Config{
				Database: DatabaseConfig{
					Enabled: true,
					Primary: DatabaseInstance{
						Host:     "localhost",
						Port:     5432,
						Database: "testdb",
						Username: "user",
						Password: "pass",
					},
					ConnectionPool: ConnectionPoolConfig{
						MaxOpenConns: 10,
						MaxIdleConns: 20, // 空闲连接数超过最大连接数
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStorageValidationRule(t *testing.T) {
	rule := &StorageValidationRule{}

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "disabled storage",
			config: &Config{
				Storage: StorageConfig{
					Files:   FilesConfig{Enabled: false},
					Objects: ObjectsConfig{Enabled: false},
				},
			},
			wantErr: false,
		},
		{
			name: "valid S3 storage",
			config: &Config{
				Storage: StorageConfig{
					Objects: ObjectsConfig{
						Enabled:  true,
						Provider: "s3",
						S3: S3Config{
							Bucket:    "test-bucket",
							Region:    "us-east-1",
							AccessKey: "access-key",
							SecretKey: "secret-key",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid S3 storage - missing bucket",
			config: &Config{
				Storage: StorageConfig{
					Objects: ObjectsConfig{
						Enabled:  true,
						Provider: "s3",
						S3: S3Config{
							Region:    "us-east-1",
							AccessKey: "access-key",
							SecretKey: "secret-key",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "unsupported storage provider",
			config: &Config{
				Storage: StorageConfig{
					Objects: ObjectsConfig{
						Enabled:  true,
						Provider: "unsupported",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
