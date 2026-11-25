package console

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Helpers for server-side idempotency of page binding execution (D4 safe retry):
// request header normalization and request content fingerprinting.

// idempotencyKeyMaxLen matches the approval/task IdempotencyKey column width
// (varchar(128)).
const idempotencyKeyMaxLen = 128

// normalizeIdempotencyKey canonicalizes the Idempotency-Key request header: any
// invalid form (too long, containing control characters, or outside the restricted
// character set) is treated as if the header were absent -- with no key the
// behavior is exactly that of the existing non-idempotent path, and a bad header
// must not block execution.
func normalizeIdempotencyKey(raw string) string {
	key := strings.TrimSpace(raw)
	if key == "" || len(key) > idempotencyKeyMaxLen {
		return ""
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-' || r == ':':
		default:
			return ""
		}
	}
	return key
}

// pageExecuteIdempotencyHash builds the idempotent request content fingerprint:
// the same key with a differing page version, binding, function, or payload = a
// client key mis-reuse (409 idempotency_key_reused). The payload is derived purely
// from the renderer context by the binding selectors (stable json.Marshal key
// order), so repeated submissions with the same rendering context are
// byte-identical and the fingerprints are comparable.
func pageExecuteIdempotencyHash(parts ...string) string {
	h := sha256.New()
	for i, part := range parts {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(part))
	}
	return hex.EncodeToString(h.Sum(nil))
}
