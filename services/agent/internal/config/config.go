package config

import (
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf

	Server struct {
		Addr           string `json:",default=localhost:8443"`
		Insecure       bool   `json:",default=false"`
		TLSCertFile    string `json:",optional"`
		TLSKeyFile     string `json:",optional"`
		CAFile         string `json:",optional"`
	} `json:",optional"`

	Agent struct {
		ID       string            `json:",optional"`
		GameID   string            `json:",optional"`
		Env      string            `json:",optional"`
		LocalAddr string           `json:",default=127.0.0.1:19090"`
		HTTPAddr  string           `json:",default=127.0.0.1:19091"`
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

	Log struct {
		ServiceName string `json:",default=croupier-agent"`
		Mode        string `json:",default=console"`
		Level       string `json:",default=info"`
	} `json:",optional"`

	TLS struct {
		Enabled            bool   `json:",default=false"`
		CertFile           string `json:",optional"`
		KeyFile            string `json:",optional"`
		CAFile             string `json:",optional"`
		InsecureSkipVerify bool   `json:",default=false"`
	} `json:",optional"`
}