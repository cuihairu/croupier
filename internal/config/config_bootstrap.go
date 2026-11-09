package config

import (
	"gopkg.in/yaml.v3"
)

type BootstrapDataConfig struct {
	BaseDir string `json:"baseDir,omitempty" yaml:"baseDir,omitempty"`
}

func (c *BootstrapDataConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain BootstrapDataConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		BaseDir string `yaml:"BaseDir,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.BaseDir == "" {
		decoded.BaseDir = compat.BaseDir
	}
	*c = BootstrapDataConfig(decoded)
	return nil
}

type DescriptorConfig struct {
	Dir string `json:"dir,omitempty" yaml:"dir,omitempty"`
	// SchemaDiffWarn 注册时 schema 兼容性告警开关（默认开启）：破坏性
	// schema 变更写入注册警告并随 RegisterResponse.warnings 返回 agent，
	// 不阻断注册。仅显式 false 关闭。
	SchemaDiffWarn *bool `json:"schemaDiffWarn,omitempty" yaml:"schemaDiffWarn,omitempty"`
}

// SchemaDiffWarnEnabled 返回生效值（nil 默认开启）。
func (c *DescriptorConfig) SchemaDiffWarnEnabled() bool {
	return c.SchemaDiffWarn == nil || *c.SchemaDiffWarn
}

func (c *DescriptorConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain DescriptorConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	var compat struct {
		Dir string `yaml:"Dir,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}

	if decoded.Dir == "" {
		decoded.Dir = compat.Dir
	}

	*c = DescriptorConfig(decoded)
	return nil
}

type SchemasConfig struct {
	Dir string `json:"dir,omitempty" yaml:"dir,omitempty"`
}

func (c *SchemasConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain SchemasConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	var compat struct {
		Dir string `yaml:"Dir,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}

	if decoded.Dir == "" {
		decoded.Dir = compat.Dir
	}

	*c = SchemasConfig(decoded)
	return nil
}
