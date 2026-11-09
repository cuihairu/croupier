package audit

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func generateAuditID() string {
	// Use crypto/rand for more unique IDs
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("audit_%d_%x", time.Now().UnixNano(), b)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

// legacyHashBytes is the version-1 digest: a 32-bit DJB2 accumulator,
// formatted %08x. Retained only to validate pre-ARCH-008 rows; new records
// use sha256Hex.
func legacyHashBytes(data []byte) string {
	h := uint32(5381)
	for _, b := range data {
		h = ((h << 5) + h) + uint32(b)
	}
	return fmt.Sprintf("%08x", h)
}

// sha256Hex is the version-2 digest: a SHA-256 over the canonical input,
// formatted as lowercase hex (64 chars, exactly chain_hash varchar(64)).
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
