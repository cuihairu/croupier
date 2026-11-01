package batchop

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// newBatchID returns a time-prefixed random id (sortable, unique enough).
func newBatchID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "batch-" + time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b[:])
}
