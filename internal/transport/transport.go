// Package transport 传输层抽象（shim）。本体已上收 core/transport（capture C1
// 前置：agents/* 依赖方向不得 import internal/，通道面归 core）；本包保留类型
// 别名与常量转发，internal 侧既有 import 路径零变化，新代码请直用 core/transport。
package transport

import "github.com/cuihairu/croupier/core/transport"

// Kind identifies a transport implementation.
type Kind = transport.Kind

const (
	KindTCP = transport.KindTCP
)

// Handler handles a request and returns the response body.
type Handler = transport.Handler

// HandlerFunc adapts a function to Handler.
type HandlerFunc = transport.HandlerFunc

// Client is a request-response transport client.
type Client = transport.Client

// Server is a request-response transport server.
type Server = transport.Server

// SessionCaller sends a request over an established TCP session and returns the response.
type SessionCaller = transport.SessionCaller
