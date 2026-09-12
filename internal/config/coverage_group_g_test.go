package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// canonical `log:` 块中只有 canonicalLogConfig 显式 lowerCamelCase tag 才能命中的
// 键（maxSize/maxBackups）对 plain Config 不可见（common.LogConfig 走字段名小写
// maxsize/maxbackups，大小写敏感不匹配），因此 decoded.Logging 保持零值，兼容层
// 选中 canonical 分支回填。
func TestUnmarshalYAMLCanonicalLogCamelKeysFallbackG(t *testing.T) {
	var cfg Config
	src := "log:\n  maxSize: 20\n  maxBackups: 5\n"
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Logging.MaxSize != 20 || cfg.Logging.MaxBackups != 5 {
		t.Fatalf("canonical log fallback not applied: %+v", cfg.Logging)
	}
}

// plain 解码成功（maxSize 键对 common.LogConfig 不可见）而 canonical 解码失败：
// 序列值无法解码进 canonicalLogConfig.MaxSize int，第三次 Decode 返回错误。
func TestUnmarshalYAMLCanonicalLogDecodeErrorG(t *testing.T) {
	var cfg Config
	src := "log:\n  maxSize: [1, 2]\n"
	if err := yaml.Unmarshal([]byte(src), &cfg); err == nil {
		t.Fatal("expected canonical log decode error, got nil")
	}
}
