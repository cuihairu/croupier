package config

import (
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf

	Server struct {
		Addr               string `json:",default=localhost:8443"`
		Insecure           bool   `json:",default=false"`
		TLSCertFile        string `json:",optional"`
		TLSKeyFile         string `json:",optional"`
		CAFile             string `json:",optional"`
		ServerName         string `json:",optional"`
		InsecureSkipVerify bool   `json:",default=false"`
	} `json:",optional"`

	Agent struct {
		ID        string            `json:",optional"`
		GameID    string            `json:",optional"`
		Env       string            `json:",optional"`
		LocalAddr string            `json:",default=127.0.0.1:19090"`
		HTTPAddr  string            `json:",default=127.0.0.1:19091"`
		Region    string            `json:",optional"`
		Zone      string            `json:",optional"`
		Labels    map[string]string `json:",optional"`
	} `json:",optional"`

	GRPC struct {
		Host    string `json:",default=127.0.0.1"`
		Port    int    `json:",default=19090"`
		Timeout int64  `json:",default=30000"`
	} `json:",optional"`

	Upstream struct {
		HeartbeatInterval int64 `json:",default=30"`
		RetryInterval     int64 `json:",default=5"`
		MaxRetries        int   `json:",default=3"`
		Timeout           int64 `json:",default=10000"`
	} `json:",optional"`

	Job struct {
		MaxConcurrent int   `json:",default=100"`
		Timeout       int64 `json:",default=300000"`
		Retries       int   `json:",default=3"`
	} `json:",optional"`

	Metrics struct {
		Enabled bool   `json:",default=true"`
		Port    int    `json:",default=9090"`
		Path    string `json:",default=/metrics"`
	} `json:",optional"`

	TLS struct {
		Enabled            bool   `json:",default=false"`
		CertFile           string `json:",optional"`
		KeyFile            string `json:",optional"`
		CAFile             string `json:",optional"`
		InsecureSkipVerify bool   `json:",default=false"`
	} `json:",optional"`

	OutboundTLS struct {
		Enabled            bool   `json:",default=false"`
		CertFile           string `json:",optional"`
		KeyFile            string `json:",optional"`
		CAFile             string `json:",optional"`
		ServerName         string `json:",optional"`
		InsecureSkipVerify bool   `json:",default=false"`
	} `json:",optional"`

	CroupierLog CroupierLogConfig `json:",optional"`
}

// CroupierLogConfig 日志配置
type CroupierLogConfig struct {
	Level      string `json:",optional"` // debug|info|warn|error
	Format     string `json:",optional"` // console|json
	Output     string `json:",optional"` // stdout|stderr
	File       string `json:",optional"` // 日志文件路径
	MaxSize    int    `json:",optional"` // 单个日志文件最大大小（MB）
	MaxBackups int    `json:",optional"` // 保留的旧日志文件最大数量
	MaxAge     int    `json:",optional"` // 保留旧日志文件的最大天数
	Compress   bool   `json:",optional"` // 是否压缩旧日志文件
}
