package config

import (
	"github.com/cuihairu/croupier/internal/cli/common"
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
		// IPC 地址（可选），用于本地高性能通信到 Server
		// 例如: "ipc://croupier-server"
		IPCAddr string `json:",optional"`
	} `json:",optional"`

	// LocalNNG 配置本地 NNG 服务器监听地址（用于 SDK 连接）
	LocalNNG struct {
		Addr    string `json:",default=:19090"` // 例如: ":19090" 或 ":19090,ipc://croupier-agent"
		IPCAddr string `json:",optional"`       // 例如: "ipc://croupier-agent"
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

	CroupierLog common.LogConfig `json:",optional"`
}
