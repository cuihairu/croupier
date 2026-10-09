package tcp

import (
	"crypto/tls"
	"net"

	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
)

// TCP 客户端已上收 core/transport/tcp（capture C1 前置：agents/* 依赖方向不
// 得 import internal/，通道面归 core）。本文件保留类型别名与私有名转发，
// mux_conn/server 及同包测试零改动；新代码请直用 core/transport/tcp。

type (
	// Config 传输配置（core/transport/tcp.Config 别名）。
	Config = tcptr.Config
	// Client 请求响应式 TCP 传输客户端（core 别名）。
	Client = tcptr.Client
)

// NewClient 建立连接并返回客户端。
func NewClient(config *Config) (*Client, error) { return tcptr.NewClient(config) }

// Dial 建立裸 TCP/TLS 连接。
func Dial(config *Config) (net.Conn, error) { return tcptr.Dial(config) }

func normalizeAddr(addr string) string { return tcptr.NormalizeAddr(addr) }

func createClientTLSConfig(config *Config) (*tls.Config, error) {
	return tcptr.CreateClientTLSConfig(config)
}
