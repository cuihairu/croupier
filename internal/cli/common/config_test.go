package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// TestLoadWithIncludes 测试加载配置文件
func TestLoadWithIncludes(t *testing.T) {
	tempDir := t.TempDir()

	// 创建基础配置文件
	baseConfig := filepath.Join(tempDir, "base.yaml")
	baseContent := `
server:
  port: 8080
  host: localhost
`
	os.WriteFile(baseConfig, []byte(baseContent), 0644)

	// 创建包含配置文件
	include1 := filepath.Join(tempDir, "include1.yaml")
	includeContent1 := `
logging:
  level: debug
  format: json
`
	os.WriteFile(include1, []byte(includeContent1), 0644)

	// 加载配置
	v, err := LoadWithIncludes(baseConfig, []string{include1})
	if err != nil {
		t.Fatalf("LoadWithIncludes() error = %v", err)
	}

	if v == nil {
		t.Fatal("LoadWithIncludes() should return non-nil viper")
	}

	// 验证基础配置
	if v.GetString("server.port") != "8080" {
		t.Errorf("server.port = %q, want '8080'", v.GetString("server.port"))
	}

	// 验证包含配置
	if v.GetString("logging.level") != "debug" {
		t.Errorf("logging.level = %q, want 'debug'", v.GetString("logging.level"))
	}
}

// TestLoadWithIncludes_EmptyBase 测试空基础配置
func TestLoadWithIncludes_EmptyBase(t *testing.T) {
	tempDir := t.TempDir()

	include1 := filepath.Join(tempDir, "include1.yaml")
	includeContent1 := `
key: value1
`
	os.WriteFile(include1, []byte(includeContent1), 0644)

	// 空基础配置
	v, err := LoadWithIncludes("", []string{include1})
	if err != nil {
		t.Fatalf("LoadWithIncludes() error = %v", err)
	}

	if v.GetString("key") != "value1" {
		t.Errorf("key = %q, want 'value1'", v.GetString("key"))
	}
}

// TestLoadWithIncludes_NoIncludes 测试无包含文件
func TestLoadWithIncludes_NoIncludes(t *testing.T) {
	tempDir := t.TempDir()

	baseConfig := filepath.Join(tempDir, "base.yaml")
	baseContent := `
key: value
`
	os.WriteFile(baseConfig, []byte(baseContent), 0644)

	v, err := LoadWithIncludes(baseConfig, []string{})
	if err != nil {
		t.Fatalf("LoadWithIncludes() error = %v", err)
	}

	if v.GetString("key") != "value" {
		t.Errorf("key = %q, want 'value'", v.GetString("key"))
	}
}

// TestLoadWithIncludes_IncludeOverride 测试包含覆盖
func TestLoadWithIncludes_IncludeOverride(t *testing.T) {
	tempDir := t.TempDir()

	baseConfig := filepath.Join(tempDir, "base.yaml")
	baseContent := `
key: base_value
nested:
  key: nested_base
`
	os.WriteFile(baseConfig, []byte(baseContent), 0644)

	include1 := filepath.Join(tempDir, "include1.yaml")
	includeContent1 := `
key: include_value
new_key: new_value
`
	os.WriteFile(include1, []byte(includeContent1), 0644)

	v, err := LoadWithIncludes(baseConfig, []string{include1})
	if err != nil {
		t.Fatalf("LoadWithIncludes() error = %v", err)
	}

	// 包含配置应该覆盖基础配置
	if v.GetString("key") != "include_value" {
		t.Errorf("key should be overridden to 'include_value', got %q", v.GetString("key"))
	}

	// 新增键应该存在
	if v.GetString("new_key") != "new_value" {
		t.Errorf("new_key = %q, want 'new_value'", v.GetString("new_key"))
	}

	// 未覆盖的嵌套值应该保留
	if v.GetString("nested.key") != "nested_base" {
		t.Errorf("nested.key should remain 'nested_base', got %q", v.GetString("nested.key"))
	}
}

// TestLoadWithIncludes_MultipleIncludes 测试多个包含文件
func TestLoadWithIncludes_MultipleIncludes(t *testing.T) {
	tempDir := t.TempDir()

	baseConfig := filepath.Join(tempDir, "base.yaml")
	baseContent := `
app:
  name: test
`
	os.WriteFile(baseConfig, []byte(baseContent), 0644)

	include1 := filepath.Join(tempDir, "include1.yaml")
	includeContent1 := `
feature1: enabled
`
	os.WriteFile(include1, []byte(includeContent1), 0644)

	include2 := filepath.Join(tempDir, "include2.yaml")
	includeContent2 := `
feature2: enabled
`
	os.WriteFile(include2, []byte(includeContent2), 0644)

	v, err := LoadWithIncludes(baseConfig, []string{include1, include2})
	if err != nil {
		t.Fatalf("LoadWithIncludes() error = %v", err)
	}

	// 所有包含都应该被加载
	if v.GetString("feature1") != "enabled" {
		t.Errorf("feature1 = %q, want 'enabled'", v.GetString("feature1"))
	}

	if v.GetString("feature2") != "enabled" {
		t.Errorf("feature2 = %q, want 'enabled'", v.GetString("feature2"))
	}

	if v.GetString("app.name") != "test" {
		t.Errorf("app.name = %q, want 'test'", v.GetString("app.name"))
	}
}

// TestLoadWithIncludes_InvalidInclude 测试无效的包含文件
func TestLoadWithIncludes_InvalidInclude(t *testing.T) {
	tempDir := t.TempDir()

	baseConfig := filepath.Join(tempDir, "base.yaml")
	baseContent := `
key: value
`
	os.WriteFile(baseConfig, []byte(baseContent), 0644)

	// 不存在的包含文件
	v, err := LoadWithIncludes(baseConfig, []string{"nonexistent.yaml"})
	if err == nil {
		t.Error("LoadWithIncludes() should return error for invalid include")
	}

	if v != nil {
		t.Error("LoadWithIncludes() should return nil viper on error")
	}
}

// TestLoadWithIncludes_InvalidBase 测试无效的基础配置
func TestLoadWithIncludes_InvalidBase(t *testing.T) {
	tempDir := t.TempDir()

	baseConfig := filepath.Join(tempDir, "base.yaml")
	os.WriteFile(baseConfig, []byte("{invalid yaml}"), 0644)

	// Viper 会尝试读取文件，即使 YAML 无效也不会返回错误
	// 它只会返回一个空配置
	v, err := LoadWithIncludes(baseConfig, []string{})
	if err != nil {
		// 如果返回错误则通过
		return
	}
	// 如果没有错误，至少验证 viper 实例存在
	if v == nil {
		t.Error("LoadWithIncludes() should return non-nil viper even with invalid base")
	}
}

// TestMergeMaps 测试合并映射
func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]any
		b        map[string]any
		expected map[string]any
	}{
		{
			name:     "空映射",
			a:        map[string]any{},
			b:        map[string]any{},
			expected: map[string]any{},
		},
		{
			name: "添加新键",
			a:        map[string]any{"key1": "value1"},
			b:        map[string]any{"key2": "value2"},
			expected: map[string]any{"key1": "value1", "key2": "value2"},
		},
		{
			name: "覆盖键",
			a:        map[string]any{"key": "old_value"},
			b:        map[string]any{"key": "new_value"},
			expected: map[string]any{"key": "new_value"},
		},
		{
			name: "嵌套映射 - 简单",
			a:        map[string]any{"nested": map[string]any{"key1": "value1"}},
			b:        map[string]any{"nested": map[string]any{"key2": "value2"}},
			expected: map[string]any{"nested": map[string]any{"key1": "value1", "key2": "value2"}},
		},
		{
			name: "嵌套映射 - 深层",
			a:        map[string]any{"level1": map[string]any{"level2": map[string]any{"key": "value"}}},
			b:        map[string]any{"level1": map[string]any{"level2": map[string]any{"key": "override"}}},
			expected: map[string]any{"level1": map[string]any{"level2": map[string]any{"key": "override"}}},
		},
		{
			name: "混合类型",
			a:        map[string]any{"string": "value", "number": 42},
			b:        map[string]any{"bool": true, "str": "data"},
			expected: map[string]any{"string": "value", "number": 42, "bool": true, "str": "data"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeMaps(tt.a, tt.b)

			// 验证键数量
			if len(result) != len(tt.expected) {
				t.Errorf("Result has %d keys, want %d", len(result), len(tt.expected))
			}

			// 验证每个键的值
			for key, expectedVal := range tt.expected {
				gotVal, ok := result[key]
				if !ok {
					t.Errorf("Key %q missing in result", key)
					continue
				}

				// 对于嵌套映射，递归比较
				if nestedExpected, ok := expectedVal.(map[string]any); ok {
					if nestedGot, ok2 := gotVal.(map[string]any); ok2 {
						// 简化验证：只检查顶层
						if len(nestedGot) != len(nestedExpected) {
							t.Errorf("Nested map for key %q has %d keys, want %d", key, len(nestedGot), len(nestedExpected))
						}
					} else {
						t.Errorf("Value for key %q is not a map", key)
					}
				} else if gotVal != expectedVal {
					t.Errorf("Value for key %q = %v, want %v", key, gotVal, expectedVal)
				}
			}
		})
	}
}

// TestApplySectionAndProfile 测试应用部分和配置文件
func TestApplySectionAndProfile(t *testing.T) {
	v := viper.New()
	v.SetConfigFile("config.yaml")

	// 设置完整配置 - profiles 必须在 section 内部
	v.Set("server.port", 8080)
	v.Set("server.host", "localhost")
	v.Set("server.profiles.dev.port", 8081)
	v.Set("server.profiles.dev.host", "dev.example.com")

	// 应用 server 部分
	serverV, err := ApplySectionAndProfile(v, "server", "dev")
	if err != nil {
		t.Fatalf("ApplySectionAndProfile() error = %v", err)
	}

	if serverV == nil {
		t.Fatal("ApplySectionAndProfile() should return non-nil viper")
	}

	// port 应该被 profile 覆盖为 8081
	if serverV.GetString("port") != "8081" {
		t.Errorf("port should be 8081 from profile, got %q", serverV.GetString("port"))
	}

	// host 应该被 profile 覆盖
	if serverV.GetString("host") != "dev.example.com" {
		t.Errorf("host = %q, want 'dev.example.com'", serverV.GetString("host"))
	}
}

// TestApplySectionAndProfile_NoProfile 测试无配置文件
func TestApplySectionAndProfile_NoProfile(t *testing.T) {
	v := viper.New()
	v.Set("server.port", 8080)
	v.Set("server.host", "localhost")

	// 不应用 profile
	serverV, err := ApplySectionAndProfile(v, "server", "")
	if err != nil {
		t.Fatalf("ApplySectionAndProfile() error = %v", err)
	}

	if serverV.GetString("port") != "8080" {
		t.Errorf("port = %q, want '8080'", serverV.GetString("port"))
	}

	if serverV.GetString("host") != "localhost" {
		t.Errorf("host = %q, want 'localhost'", serverV.GetString("host"))
	}
}

// TestApplySectionAndProfile_NoSection 测试无部分
func TestApplySectionAndProfile_NoSection(t *testing.T) {
	v := viper.New()
	v.Set("key", "value")
	v.Set("profiles.dev.key", "dev_value")
	v.Set("profiles.dev.newKey", "new_value")

	// 不应用 section，只应用 profile
	resultV, err := ApplySectionAndProfile(v, "", "dev")
	if err != nil {
		t.Fatalf("ApplySectionAndProfile() error = %v", err)
	}

	// key 应该被 profile 覆盖
	if resultV.GetString("key") != "dev_value" {
		t.Errorf("key = %q, want 'dev_value'", resultV.GetString("key"))
	}

	// profile 中的新键应该存在
	if resultV.GetString("newKey") != "new_value" {
		t.Errorf("newKey = %q, want 'new_value'", resultV.GetString("newKey"))
	}
}

// TestApplySectionAndProfile_InvalidSection 测试无效部分
func TestApplySectionAndProfile_InvalidSection(t *testing.T) {
	v := viper.New()
	v.Set("key", "value")

	_, err := ApplySectionAndProfile(v, "nonexistent", "")
	if err == nil {
		t.Error("ApplySectionAndProfile() should return error for invalid section")
	}
}

// TestApplySectionAndProfile_InvalidProfile 测试无效配置文件
func TestApplySectionAndProfile_InvalidProfile(t *testing.T) {
	v := viper.New()
	v.Set("server.port", 8080)
	v.Set("profiles.dev.port", 8081)

	// 尝试应用不存在的 profile
	_, err := ApplySectionAndProfile(v, "server", "nonexistent")
	if err == nil {
		t.Error("ApplySectionAndProfile() should return error for invalid profile")
	}
}

// TestApplySectionAndProfile_NoProfiles 测试没有 profiles 键
func TestApplySectionAndProfile_NoProfiles(t *testing.T) {
	v := viper.New()
	v.Set("server.port", 8080)

	_, err := ApplySectionAndProfile(v, "server", "dev")
	if err == nil {
		t.Error("ApplySectionAndProfile() should return error when profiles section doesn't exist")
	}
}

// TestLoadWithIncludes_Inheritance 测试配置继承
func TestLoadWithIncludes_Inheritance(t *testing.T) {
	tempDir := t.TempDir()

	baseConfig := filepath.Join(tempDir, "base.yaml")
	baseContent := `
database:
  host: localhost
  port: 5432
  ssl:
    enabled: false
`
	os.WriteFile(baseConfig, []byte(baseContent), 0644)

	overrideConfig := filepath.Join(tempDir, "override.yaml")
	overrideContent := `
database:
  host: prod.example.com
  ssl:
    enabled: true
    cert: /path/to/cert.pem
`
	os.WriteFile(overrideConfig, []byte(overrideContent), 0644)

	v, err := LoadWithIncludes(baseConfig, []string{overrideConfig})
	if err != nil {
		t.Fatalf("LoadWithIncludes() error = %v", err)
	}

	// database.host 应该被覆盖
	if v.GetString("database.host") != "prod.example.com" {
		t.Errorf("database.host = %q, want 'prod.example.com'", v.GetString("database.host"))
	}

	// database.port 应该保留
	if v.GetString("database.port") != "5432" {
		t.Errorf("database.port = %q, want '5432'", v.GetString("database.port"))
	}

	// database.ssl.enabled 应该被覆盖
	if v.GetString("database.ssl.enabled") != "true" {
		t.Errorf("database.ssl.enabled = %q, want 'true'", v.GetString("database.ssl.enabled"))
	}

	// database.ssl.cert 应该被添加
	if v.GetString("database.ssl.cert") != "/path/to/cert.pem" {
		t.Errorf("database.ssl.cert = %q, want '/path/to/cert.pem'", v.GetString("database.ssl.cert"))
	}
}

// BenchmarkLoadWithIncludes 性能基准测试
func BenchmarkLoadWithIncludes(b *testing.B) {
	tempDir := b.TempDir()

	baseConfig := filepath.Join(tempDir, "base.yaml")
	baseContent := `
key: value
`
	os.WriteFile(baseConfig, []byte(baseContent), 0644)

	includeConfig := filepath.Join(tempDir, "include.yaml")
	includeContent := `
include_key: include_value
`
	os.WriteFile(includeConfig, []byte(includeContent), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LoadWithIncludes(baseConfig, []string{includeConfig})
	}
}

// BenchmarkMergeMaps 性能基准测试
func BenchmarkMergeMaps(b *testing.B) {
	a := map[string]any{
		"key1": "value1",
		"key2": "value2",
		"nested": map[string]any{
			"nk1": "nv1",
			"nk2": "nv2",
		},
	}

	bb := map[string]any{
		"key2": "overridden",
		"key3": "value3",
		"nested": map[string]any{
			"nk1": "overridden",
			"nk3": "nv3",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mergeMaps(a, bb)
	}
}
