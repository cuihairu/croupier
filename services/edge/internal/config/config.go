package config

import (
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf

	Server struct {
		InternalAddr string `json:",default=localhost:8443"`
		PublicAddr   string `json:",default=edge.example.com"`
		Insecure     bool   `json:",default=false"`
		TLSCertFile  string `json:",optional"`
		TLSKeyFile   string `json:",optional"`
	} `json:",optional"`

	Edge struct {
		Region string            `json:",optional"`
		Zone   string            `json:",optional"`
		Labels map[string]string `json:",optional"`
	} `json:",optional"`

	Tunnel struct {
		MaxTunnels  int   `json:",default=1000"`
		Timeout     int64 `json:",default=300000"`
		IdleTimeout int64 `json:",default=60000"`
		BufferSize  int   `json:",default=32768"`
	} `json:",optional"`

	Proxy struct {
		MaxConnections int   `json:",default=10000"`
		RequestTimeout  int64 `json:",default=30000"`
		ReadTimeout     int64 `json:",default=30000"`
		WriteTimeout    int64 `json:",default=30000"`
	} `json:",optional"`

	LoadBalancer struct {
		Strategy       string `json:",default=round_robin"`
		HealthCheck    bool   `json:",default=true"`
		HealthInterval int64  `json:",default=30"`
		HealthTimeout  int64  `json:",default=5000"`
	} `json:",optional"`

	Metrics struct {
		Enabled bool   `json:",default=true"`
		Port    int    `json:",default=9091"`
		Path    string `json:",default=/metrics"`
	} `json:",optional"`

	Log struct {
		ServiceName string `json:",default=croupier-edge"`
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