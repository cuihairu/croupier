package tcp

import (
	"net"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
)

type (
	// ProtocolError identifies a protocol-level session failure.
	ProtocolError = tcptr.ProtocolError
	// MuxConn is a bidirectional multiplexed TCP connection.
	MuxConn = tcptr.MuxConn
)

func NewProtocolError(err error) error { return tcptr.NewProtocolError(err) }

func NewMuxConn(conn net.Conn, config *Config, handler transportcore.Handler) *MuxConn {
	return tcptr.NewMuxConn(conn, config, handler)
}
