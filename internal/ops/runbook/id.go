package runbook

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// newRunbookID returns a time-prefixed random id (sortable, unique enough).
func newRunbookID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "rb-" + time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b[:])
}

// newRunID returns a time-prefixed random id for one runbook execution.
func newRunID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "run-" + time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b[:])
}
