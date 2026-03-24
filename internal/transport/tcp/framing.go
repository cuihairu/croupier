package tcp

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	frameVersion = byte(0x01)
	headerBytes  = 9
)

var frameMagic = [4]byte{'C', 'R', 'P', '1'}

func writeFrame(w io.Writer, payload []byte) error {
	header := make([]byte, headerBytes)
	copy(header[:4], frameMagic[:])
	header[4] = frameVersion
	binary.BigEndian.PutUint32(header[5:9], uint32(len(payload)))

	if _, err := w.Write(header); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return nil
}

func readFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, headerBytes)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	if string(header[:4]) != string(frameMagic[:]) {
		return nil, fmt.Errorf("invalid frame magic")
	}
	if header[4] != frameVersion {
		return nil, fmt.Errorf("unsupported frame version: %d", header[4])
	}

	size := binary.BigEndian.Uint32(header[5:9])
	if size == 0 {
		return []byte{}, nil
	}

	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
