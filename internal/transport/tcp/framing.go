package tcp

import (
	"io"

	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
)

// framing 已上收 core/transport/tcp（capture C1 前置）；本文件保留包内私有
// 名转发，mux_conn/server 及同包测试零改动。

const (
	// FrameHeaderBytes / MaxFrameBytes 转发 core 侧 framing 常量（server_test 用）。
	FrameHeaderBytes = tcptr.FrameHeaderBytes
	MaxFrameBytes    = tcptr.MaxFrameBytes
)

func writeFrame(w io.Writer, payload []byte) error { return tcptr.WriteFrame(w, payload) }

func readFrame(r io.Reader) ([]byte, error) { return tcptr.ReadFrame(r) }
