// Package events is the demo-game analytics emit hook (compile stub, Phase 2+).
//
// The platform's /api/v1/analytics/ingest is admin-JWT gated today; a
// game-side HMAC ingest contract does not exist yet. This Emitter keeps the
// client side of that future contract exercised so the game side never
// blocks on it:
//
//   - Endpoint unset (default) → Emit drops events, returns 0. The demo game
//     stays fully functional without analytics.
//   - Endpoint + secret set → Emit POSTs one IngestRequest per batch to
//     <endpoint>/api/v1/analytics/ingest with
//     X-Croupier-Timestamp / X-Croupier-Signature =
//     hex(HMAC-SHA256(secret, "<timestamp>.<body>")), mirroring the shape the
//     platform verifier is expected to check when GAME-side ingest lands.
//
// Deliberately not wired into main.go yet: the server verifier is future
// work (documented boundary in docs/design/demo-game.md).
package events

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	// EnvEndpoint enables the emitter when set (e.g. http://server:18780).
	EnvEndpoint = "ANALYTICS_INGEST_ENDPOINT"
	// EnvSecret enables HMAC signing when set alongside EnvEndpoint.
	EnvSecret = "ANALYTICS_INGEST_SECRET"
	// TimestampHeader carries the unix-second signing timestamp.
	TimestampHeader = "X-Croupier-Timestamp"
	// SignatureHeader carries the hex HMAC-SHA256 signature.
	SignatureHeader = "X-Croupier-Signature"
)

// IngestRequest mirrors internal/api/analytics IngestRequest wire shape
// (lowerCamelCase contract keys).
type IngestRequest struct {
	GameId    string         `json:"gameId"`
	Env       string         `json:"env"`
	Events    map[string]any `json:"events"`
	Timestamp string         `json:"timestamp"`
}

// Emitter posts analytics event batches. Safe for concurrent use.
type Emitter struct {
	endpoint string // base URL, empty = disabled
	secret   string
	gameID   string
	env      string
	client   *http.Client
}

// NewEmitter reads configuration from the environment; both vars unset means
// the emitter is disabled and Emit is a no-op.
func NewEmitter(gameID, env string) *Emitter {
	return &Emitter{
		endpoint: os.Getenv(EnvEndpoint),
		secret:   os.Getenv(EnvSecret),
		gameID:   gameID,
		env:      env,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

// Enabled reports whether emission is configured.
func (e *Emitter) Enabled() bool { return e.endpoint != "" }

// Emit posts one batch; returns the accepted count (0 when disabled or on
// any error — analytics must never break the game loop, so errors are
// reported via the return value, not panics).
func (e *Emitter) Emit(batch map[string]any) (int, error) {
	if !e.Enabled() {
		return 0, nil
	}
	body, err := json.Marshal(IngestRequest{
		GameId:    e.gameID,
		Env:       e.env,
		Events:    batch,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest(http.MethodPost, e.endpoint+"/api/v1/analytics/ingest", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set(TimestampHeader, ts)
	if e.secret != "" {
		req.Header.Set(SignatureHeader, Sign(e.secret, ts, body))
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("analytics ingest: status %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Accepted int `json:"accepted"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&out); err != nil {
		return 0, err
	}
	return out.Accepted, nil
}

// Sign computes hex(HMAC-SHA256(secret, "<timestamp>.<body>")) — the exact
// canonical string the future server-side verifier must reproduce.
func Sign(secret string, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
