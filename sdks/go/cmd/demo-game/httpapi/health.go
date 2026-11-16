// Package httpapi serves the demo-game health endpoints consumed by the
// deployment stack's healthcheck.
//
// Contract: probes may return minimal payloads and are exempt from the
// platform JSON envelope (see CLAUDE.md API Response Contract, section 4).
package httpapi

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

// Ready tracks process serving state: set once the SDK client is registered
// and Serve has been launched, cleared on shutdown. The public SDK exposes no
// connection-state accessor, so this is process readiness, not agent-link
// health — the compose healthcheck uses it to keep the container out of the
// pool until registration completed.
type Ready struct {
	v atomic.Bool
}

// SetReady flips the readiness flag.
func (r *Ready) SetReady(ready bool) { r.v.Store(ready) }

// Healthz is the liveness probe: always 200 while the process runs.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeProbe(w, http.StatusOK, "ok")
}

// Readyz is the readiness probe: 200 once serving, 503 before registration
// completes and during shutdown.
func Readyz(ready *Ready) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if ready == nil || !ready.v.Load() {
			writeProbe(w, http.StatusServiceUnavailable, "not_ready")
			return
		}
		writeProbe(w, http.StatusOK, "ok")
	}
}

func writeProbe(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}
