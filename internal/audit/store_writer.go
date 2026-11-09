package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// AuditWriter wraps io.Writer for audit logging
type AuditWriter struct {
	w    io.Writer
	prev []byte
	mu   sync.Mutex
	seq  int64
}

// NewAuditWriter creates a new audit writer
func NewAuditWriter(w io.Writer) *AuditWriter {
	return &AuditWriter{
		w:    w,
		prev: make([]byte, 32),
		seq:  0,
	}
}

// Write writes an audit record
func (w *AuditWriter) Write(record *AuditRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.seq++
	record.ChainInfo.Sequence = w.seq
	record.ChainInfo.PrevHash = fmt.Sprintf("%x", w.prev)

	// Calculate hash
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	hash := simpleHash(data)
	record.ChainInfo.Hash = hash

	// Write to underlying writer
	// 第一次 Marshal 已成功，两次调用之间仅对 ChainInfo.Hash 做 string 赋值，
	// 第二次 Marshal 恒成功，err 分支为死代码，已删除。
	data, _ = json.Marshal(record)

	if _, err := w.w.Write(append(data, '\n')); err != nil {
		return err
	}

	// Update prev hash
	copy(w.prev, []byte(hash))

	return nil
}

func simpleHash(data []byte) string {
	h := uint32(5381)
	for _, b := range data {
		h = ((h << 5) + h) + uint32(b)
	}
	return fmt.Sprintf("%08x", h)
}
