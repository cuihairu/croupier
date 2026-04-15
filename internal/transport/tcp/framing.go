package tcp

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	frameHeaderBytes = 4
	maxFrameBytes    = 32 << 20
)

func writeFrame(w io.Writer, payload []byte) error {
	if len(payload) > maxFrameBytes {
		return fmt.Errorf("frame too large: %d > %d", len(payload), maxFrameBytes)
	}

	header := make([]byte, frameHeaderBytes)
	binary.BigEndian.PutUint32(header, uint32(len(payload)))

	if _, err := w.Write(header); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return nil
}

func readFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, frameHeaderBytes)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(header)
	if size == 0 {
		return []byte{}, nil
	}
	if size > maxFrameBytes {
		return nil, fmt.Errorf("frame too large: %d > %d", size, maxFrameBytes)
	}

	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
