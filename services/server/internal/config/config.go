// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	Registry    RegistryConfig   `json:"registry" yaml:"registry"`
	Auth        AuthConfig       `json:"auth" yaml:"auth"`
	Descriptors DescriptorConfig `json:"descriptors" yaml:"descriptors"`
	Components  ComponentsConfig `json:"components" yaml:"components"`
	Schemas     SchemasConfig    `json:"schemas" yaml:"schemas"`
	Packs       PacksConfig      `json:"packs" yaml:"packs"`
}

type RegistryConfig struct {
	AssignmentsPath      string `json:"assignments_path" yaml:"assignments_path"`
	AnalyticsFiltersPath string `json:"analytics_filters_path" yaml:"analytics_filters_path"`
	RateLimitsPath       string `json:"rate_limits_path" yaml:"rate_limits_path"`
}

type AuthConfig struct {
	JWTSecret string `json:"jwt_secret" yaml:"jwt_secret"`
}

type DescriptorConfig struct {
	Dir string `json:"dir" yaml:"dir"`
}

type ComponentsConfig struct {
	DataDir    string `json:"data_dir" yaml:"data_dir"`
	StagingDir string `json:"staging_dir" yaml:"staging_dir"`
}

type SchemasConfig struct {
	Dir string `json:"dir" yaml:"dir"`
}

type PacksConfig struct {
	Dir string `json:"dir" yaml:"dir"`
}
