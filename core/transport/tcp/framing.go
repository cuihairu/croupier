// Package tcp Croupier TCP 传输（length-prefix framing + 请求响应，原样上收
// 自 internal/transport/tcp；agents 系经本包接入注册通道，agents/* 不 import
// internal/ 的依赖方向由此成立）。
package tcp

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// FrameHeaderBytes is the length-prefix frame header size (uint32 big endian).
	FrameHeaderBytes = 4
	// MaxFrameBytes is the maximum allowed frame payload size.
	MaxFrameBytes = 32 << 20
)

// 包内私有别名（客户端沿用原名）。
var (
	writeFrame = WriteFrame
	readFrame  = ReadFrame
)

func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) > MaxFrameBytes {
		return fmt.Errorf("frame too large: %d > %d", len(payload), MaxFrameBytes)
	}

	header := make([]byte, FrameHeaderBytes)
	binary.BigEndian.PutUint32(header, uint32(len(payload)))

	if _, err := w.Write(header); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return nil
}

func ReadFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, FrameHeaderBytes)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(header)
	if size == 0 {
		return []byte{}, nil
	}
	if size > MaxFrameBytes {
		return nil, fmt.Errorf("frame too large: %d > %d", size, MaxFrameBytes)
	}

	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
