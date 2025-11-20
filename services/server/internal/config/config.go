// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Server      ServerConfig      `json:"server" yaml:"server"`
	Registry    RegistryConfig    `json:"registry" yaml:"registry"`
	Auth        AuthConfig        `json:"auth" yaml:"auth"`
	Descriptors DescriptorConfig  `json:"descriptors" yaml:"descriptors"`
	Components  ComponentsConfig  `json:"components" yaml:"components"`
	Schemas     SchemasConfig     `json:"schemas" yaml:"schemas"`
	Packs       PacksConfig       `json:"packs" yaml:"packs"`
	Storage     StorageConfig     `json:"storage" yaml:"storage"`
	CroupierLog CroupierLogConfig `json:"croupier_log" yaml:"croupier_log"`
	Metrics     MetricsConfig     `json:"metrics" yaml:"metrics"`
	Profiles    map[string]ProfileConfig `json:"profiles" yaml:"profiles"`
}

type ServerConfig struct {
	Addr     string          `json:"addr" yaml:"addr"`
	HttpAddr string          `json:"http_addr,optional" yaml:"http_addr,optional"`
	Cert     string          `json:"cert,optional" yaml:"cert,optional"`
	Key      string          `json:"key,optional" yaml:"key,optional"`
	CA       string          `json:"ca,optional" yaml:"ca,optional"`
	Database DatabaseConfig  `json:"db" yaml:"db"`
}

type DatabaseConfig struct {
	Driver   string `json:"driver,optional" yaml:"driver,optional"`
	DataSource string `json:"datasource,optional" yaml:"datasource,optional"`
}

type RegistryConfig struct {
	AssignmentsPath      string `json:"assignments_path,optional" yaml:"assignments_path,optional"`
	AnalyticsFiltersPath string `json:"analytics_filters_path,optional" yaml:"analytics_filters_path,optional"`
	RateLimitsPath       string `json:"rate_limits_path,optional" yaml:"rate_limits_path,optional"`
}

type AuthConfig struct {
	JWTSecret    string `json:"jwt_secret,optional" yaml:"jwt_secret,optional"`
	RBACConfig   string `json:"rbac_config,optional" yaml:"rbac_config,optional"`
	UsersConfig  string `json:"users_config,optional" yaml:"users_config,optional"`
	GamesConfig  string `json:"games_config,optional" yaml:"games_config,optional"`
}

type DescriptorConfig struct {
	Dir string `json:"dir,optional" yaml:"dir,optional"`
}

type ComponentsConfig struct {
	DataDir    string `json:"data_dir,optional" yaml:"data_dir,optional"`
	StagingDir string `json:"staging_dir,optional" yaml:"staging_dir,optional"`
}

type SchemasConfig struct {
	Dir string `json:"dir,optional" yaml:"dir,optional"`
}

type PacksConfig struct {
	Dir string `json:"dir,optional" yaml:"dir,optional"`
}

type StorageConfig struct {
	Driver           string `json:"driver,optional" yaml:"driver,optional"`
	Bucket           string `json:"bucket,optional" yaml:"bucket,optional"`
	Region           string `json:"region,optional" yaml:"region,optional"`
	Endpoint         string `json:"endpoint,optional" yaml:"endpoint,optional"`
	AccessKey        string `json:"access_key,optional" yaml:"access_key,optional"`
	SecretKey        string `json:"secret_key,optional" yaml:"secret_key,optional"`
	ForcePathStyle   bool   `json:"force_path_style,optional" yaml:"force_path_style,optional"`
	SignedURLTTL     string `json:"signed_url_ttl,optional" yaml:"signed_url_ttl,optional"`
	BaseDir          string `json:"base_dir,optional" yaml:"base_dir,optional"`
}

type CroupierLogConfig struct {
	Level      string `json:"level,optional" yaml:"level,optional"`
	Format     string `json:"format,optional" yaml:"format,optional"`
	Output     string `json:"output,optional" yaml:"output,optional"`
	File       string `json:"file,optional" yaml:"file,optional"`
	MaxSize    int    `json:"max_size,optional" yaml:"max_size,optional"`
	MaxBackups int    `json:"max_backups,optional" yaml:"max_backups,optional"`
	MaxAge     int    `json:"max_age,optional" yaml:"max_age,optional"`
	Compress   bool   `json:"compress,optional" yaml:"compress,optional"`
}

type MetricsConfig struct {
	PerFunction    bool `json:"per_function,optional" yaml:"per_function,optional"`
	PerGameDenies  bool `json:"per_game_denies,optional" yaml:"per_game_denies,optional"`
}

type ProfileConfig struct {
	Log     map[string]interface{} `json:"log" yaml:"log"`
	DB      map[string]interface{} `json:"db" yaml:"db"`
	Storage map[string]interface{} `json:"storage" yaml:"storage"`
}