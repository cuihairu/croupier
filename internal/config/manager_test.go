package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_LoadFromFile(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{
			name: "valid yaml config",
			config: `
app:
  name: test-app
  version: 1.0.0
  env: development
  debug: true
network:
  server:
    host: localhost
    http_port: 8080
    grpc_port: 9090
    tls:
      enabled: false
database:
  enabled: true
  primary:
    host: localhost
    port: 5432
    database: testdb
    username: testuser
    password: testpass
    ssl_mode: disable
`,
			wantErr: false,
		},
		{
			name: "invalid config - missing app name",
			config: `
app:
  version: 1.0.0
  env: development
network:
  server:
    http_port: 8080
    grpc_port: 9090
`,
			wantErr: true,
		},
		{
			name: "invalid config - invalid port",
			config: `
app:
  name: test-app
  version: 1.0.0
  env: development
network:
  server:
    http_port: 70000
    grpc_port: 9090
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建临时配置文件
			tmpFile := t.TempDir() + "/config.yaml"
			err := os.WriteFile(tmpFile, []byte(tt.config), 0644)
			require.NoError(t, err)

			// 创建管理器
			mgr, err := NewManager(context.Background())
			require.NoError(t, err)
			defer mgr.Close()

			// 加载配置
			err = mgr.LoadFromFile(tmpFile)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// 验证配置内容
				config := mgr.GetConfig()
				assert.Equal(t, "test-app", config.App.Name)
				assert.Equal(t, "1.0.0", config.App.Version)
				assert.Equal(t, 8080, config.Network.Server.HTTPPort)
				assert.Equal(t, 9090, config.Network.Server.GRPCPort)
			}
		})
	}
}

func TestManager_LoadFromMultiple(t *testing.T) {
	mgr, err := NewManager(context.Background())
	require.NoError(t, err)
	defer mgr.Close()

	// 创建基础配置文件
	baseConfig := `
app:
  name: test-app
  version: 1.0.0
  env: development
network:
  server:
    http_port: 8080
    grpc_port: 9090
`
	baseFile := t.TempDir() + "/base.yaml"
	err = os.WriteFile(baseFile, []byte(baseConfig), 0644)
	require.NoError(t, err)

	// 创建覆盖配置文件
	overrideConfig := `
network:
  server:
    http_port: 8081
database:
  enabled: true
  primary:
    host: localhost
    port: 5432
    database: testdb
`
	overrideFile := t.TempDir() + "/override.yaml"
	err = os.WriteFile(overrideFile, []byte(overrideConfig), 0644)
	require.NoError(t, err)

	// 设置环境变量
	os.Setenv("CROUPIER_APP_DEBUG", "true")
	defer os.Unsetenv("CROUPIER_APP_DEBUG")

	sources := []*ConfigSource{
		NewConfigSource(SourceTypeFile, baseFile, true),
		NewConfigSource(SourceTypeFile, overrideFile, true),
		NewEnvConfigSource("CROUPIER_", false),
	}

	err = mgr.LoadFromMultiple(sources)
	require.NoError(t, err)

	// 验证合并后的配置
	config := mgr.GetConfig()
	assert.Equal(t, "test-app", config.App.Name)
	assert.Equal(t, 8081, config.Network.Server.HTTPPort) // 覆盖的值
	assert.Equal(t, 9090, config.Network.Server.GRPCPort) // 原始值
	assert.True(t, config.App.Debug)                      // 环境变量
	assert.True(t, config.Database.Enabled)
}

func TestManager_GetConfigSections(t *testing.T) {
	// 创建测试配置
	config := `
app:
  name: test-app
  version: 1.0.0
  env: development
  debug: true
network:
  server:
    http_port: 8080
    grpc_port: 9090
database:
  enabled: true
  primary:
    host: localhost
    port: 5432
    database: testdb
    username: tester
    password: secret
`

	tmpFile := t.TempDir() + "/config.yaml"
	err := os.WriteFile(tmpFile, []byte(config), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(context.Background())
	require.NoError(t, err)
	defer mgr.Close()

	err = mgr.LoadFromFile(tmpFile)
	require.NoError(t, err)

	// 测试获取配置段
	appConfig := mgr.GetAppConfig()
	assert.Equal(t, "test-app", appConfig.Name)
	assert.Equal(t, "1.0.0", appConfig.Version)

	networkConfig := mgr.GetNetworkConfig()
	assert.Equal(t, 8080, networkConfig.Server.HTTPPort)
	assert.Equal(t, 9090, networkConfig.Server.GRPCPort)

	dbConfig := mgr.GetDatabaseConfig()
	assert.True(t, dbConfig.Enabled)
	assert.Equal(t, "localhost", dbConfig.Primary.Host)
}

func TestManager_UpdateConfig(t *testing.T) {
	mgr, err := NewManager(context.Background())
	require.NoError(t, err)
	defer mgr.Close()

	// 设置初始配置
	initialConfig := &Config{
		App: AppConfig{
			Name:    "test-app",
			Version: "1.0.0",
			Env:     "development",
			Debug:   false,
		},
		Network: NetworkConfig{
			Server: ServerConfig{
				HTTPPort: 8080,
				GRPCPort: 9090,
			},
		},
	}

	mgr.currentConfig = initialConfig
	if l, ok := mgr.loader.(*loader); ok {
		l.mu.Lock()
		l.config = initialConfig
		l.mu.Unlock()
	}

	// 更新配置
	err = mgr.UpdateConfig(func(config *Config) error {
		config.App.Debug = true
		config.Network.Server.HTTPPort = 8081
		return nil
	})

	require.NoError(t, err)

	// 验证更新
	updatedConfig := mgr.GetConfig()
	assert.True(t, updatedConfig.App.Debug)
	assert.Equal(t, 8081, updatedConfig.Network.Server.HTTPPort)
	assert.Equal(t, 9090, updatedConfig.Network.Server.GRPCPort) // 未修改的值保持不变
}

func TestManager_Export(t *testing.T) {
	config := `
app:
  name: test-app
  version: 1.0.0
  env: development
network:
  server:
    http_port: 8080
    grpc_port: 9090
`

	tmpFile := t.TempDir() + "/config.yaml"
	err := os.WriteFile(tmpFile, []byte(config), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(context.Background())
	require.NoError(t, err)
	defer mgr.Close()

	err = mgr.LoadFromFile(tmpFile)
	require.NoError(t, err)

	// 测试JSON导出
	jsonData, err := mgr.Export("json")
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), "test-app")
	assert.Contains(t, string(jsonData), "8080")

	// 测试YAML导出
	yamlData, err := mgr.Export("yaml")
	require.NoError(t, err)
	assert.Contains(t, string(yamlData), "test-app")
	assert.Contains(t, string(yamlData), "8080")

	// 测试不支持的格式
	_, err = mgr.Export("xml")
	assert.Error(t, err)
}

func TestManager_Reload(t *testing.T) {
	// 创建初始配置文件
	config := `
app:
  name: test-app
  version: 1.0.0
network:
  server:
    http_port: 8080
    grpc_port: 9090
`

	tmpFile := t.TempDir() + "/config.yaml"
	err := os.WriteFile(tmpFile, []byte(config), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(context.Background())
	require.NoError(t, err)
	defer mgr.Close()

	err = mgr.LoadFromFile(tmpFile)
	require.NoError(t, err)

	originalConfig := mgr.GetConfig()
	assert.Equal(t, 8080, originalConfig.Network.Server.HTTPPort)

	// 修改配置文件
	updatedConfig := `
app:
  name: test-app
  version: 1.0.0
network:
  server:
    http_port: 8081
    grpc_port: 9090
`

	err = os.WriteFile(tmpFile, []byte(updatedConfig), 0644)
	require.NoError(t, err)

	// 重新加载配置
	err = mgr.Reload()
	require.NoError(t, err)

	// 验证配置已更新
	reloadedConfig := mgr.GetConfig()
	assert.Equal(t, 8081, reloadedConfig.Network.Server.HTTPPort)
}

func TestManager_RestartChan(t *testing.T) {
	mgr, err := NewManager(context.Background())
	require.NoError(t, err)
	defer mgr.Close()

	// 测试重启信号通道
	restartChan := mgr.RestartChan()
	assert.NotNil(t, restartChan)

	// 准备初始配置
	initial := &Config{
		App: AppConfig{
			Name:    "test",
			Version: "1.0.0",
		},
		Network: NetworkConfig{
			Server: ServerConfig{
				HTTPPort: 8080,
				GRPCPort: 9090,
			},
		},
	}
	mgr.currentConfig = initial
	if l, ok := mgr.loader.(*loader); ok {
		l.mu.Lock()
		l.config = initial
		l.mu.Unlock()
	}

	// 更新配置应该触发重启信号
	err = mgr.UpdateConfig(func(config *Config) error {
		config.App.Version = "1.0.1"
		return nil
	})
	require.NoError(t, err)

	select {
	case <-restartChan:
		// 收到信号，测试通过
	default:
		t.Error("应该收到重启信号")
	}
}

func TestConfigSource_Creators(t *testing.T) {
	// 测试文件配置源
	fileSource := NewConfigSource(SourceTypeFile, "/path/to/config.yaml", true)
	assert.Equal(t, SourceTypeFile, fileSource.Type)
	assert.Equal(t, "/path/to/config.yaml", fileSource.Path)
	assert.True(t, fileSource.Required)

	// 测试远程配置源
	headers := map[string]string{"Authorization": "Bearer token"}
	remoteSource := NewRemoteConfigSource("https://example.com/config", headers, false)
	assert.Equal(t, SourceTypeRemote, remoteSource.Type)
	assert.Equal(t, "https://example.com/config", remoteSource.URL)
	assert.Equal(t, headers, remoteSource.Headers)
	assert.False(t, remoteSource.Required)

	// 测试环境变量配置源
	envSource := NewEnvConfigSource("PREFIX_", true)
	assert.Equal(t, SourceTypeEnv, envSource.Type)
	assert.Equal(t, "PREFIX_", envSource.Prefix)
	assert.True(t, envSource.Required)

	// 测试JSON配置源
	jsonContent := `{"key": "value"}`
	jsonSource := NewJSONConfigSource(jsonContent, false)
	assert.Equal(t, SourceTypeJSON, jsonSource.Type)
	assert.Equal(t, jsonContent, jsonSource.Content)
	assert.False(t, jsonSource.Required)

	// 测试YAML配置源
	yamlContent := "key: value"
	yamlSource := NewYAMLConfigSource(yamlContent, true)
	assert.Equal(t, SourceTypeYAML, yamlSource.Type)
	assert.Equal(t, yamlContent, yamlSource.Content)
	assert.True(t, yamlSource.Required)
}

func TestManager_ContextIntegration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr, err := NewManager(ctx)
	require.NoError(t, err)
	defer mgr.Close()

	// 测试创建配置上下文
	configCtx, configCancel := mgr.CreateConfigContext("test-operation")
	defer configCancel()

	// 验证上下文
	assert.NotNil(t, configCtx)
	assert.NotNil(t, configCancel)

	// 验证超时设置（30秒默认）
	deadline, ok := configCtx.Deadline()
	assert.True(t, ok)
	assert.True(t, time.Until(deadline) > 29*time.Second)
	assert.True(t, time.Until(deadline) < 31*time.Second)
}

func TestManager_ErrorHandling(t *testing.T) {
	mgr, err := NewManager(context.Background())
	require.NoError(t, err)
	defer mgr.Close()

	// 测试加载不存在的文件
	err = mgr.LoadFromFile("/non/existent/file.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "读取配置文件失败")

	// 测试无效配置
	invalidConfig := `
app:
  name: ""  # 空名称应该验证失败
`
	tmpFile := t.TempDir() + "/invalid.yaml"
	err = os.WriteFile(tmpFile, []byte(invalidConfig), 0644)
	require.NoError(t, err)

	err = mgr.LoadFromFile(tmpFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "应用名称不能为空")
}

func TestManager_ConcurrentAccess(t *testing.T) {
	mgr, err := NewManager(context.Background())
	require.NoError(t, err)
	defer mgr.Close()

	// 设置初始配置
	config := `
app:
  name: test-app
  version: 1.0.0
network:
  server:
    http_port: 8080
    grpc_port: 9090
`
	tmpFile := t.TempDir() + "/config.yaml"
	err = os.WriteFile(tmpFile, []byte(config), 0644)
	require.NoError(t, err)

	err = mgr.LoadFromFile(tmpFile)
	require.NoError(t, err)

	// 并发读取测试
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				config := mgr.GetConfig()
				assert.NotNil(t, config)
				assert.Equal(t, "test-app", config.App.Name)
			}
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}
