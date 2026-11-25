package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// newOIDCState generates a state in "payload.signature" form;
// payload is base64url("nonce.timestamp") and the signature is
// HMAC-SHA256(jwtSecret, payload).
// crypto/rand.Read never returns an error since Go 1.24 (failure is fatal
// in-process), so the implementation has no failure path and the signature was
// tightened to drop the error return.
func (s *Service) newOIDCState() string {
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	payload := fmt.Sprintf("%s.%d", hex.EncodeToString(nonce), time.Now().Unix())
	sig := s.signState(payload)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func (s *Service) verifyOIDCState(state string) bool {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	if !hmac.Equal([]byte(parts[1]), []byte(s.signState(string(payload)))) {
		return false
	}
	// Validity window is 10 minutes.
	fields := strings.Split(string(payload), ".")
	if len(fields) != 2 {
		return false
	}
	var ts int64
	if _, err := fmt.Sscanf(fields[1], "%d", &ts); err != nil {
		return false
	}
	age := time.Since(time.Unix(ts, 0))
	return age >= 0 && age <= 10*time.Minute
}

func (s *Service) signState(payload string) string {
	mac := hmac.New(sha256.New, []byte(s.jwtSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
