// Provider handshake auth (CR-FIX-11): the SDK Provider → Agent
// ProviderConnectRequest carries metadata["authToken"], and the Agent side can
// be configured to enforce validation. Both connection paths (the TCP local
// listener's providerSessionHandler.handleConnect and the mux path
// LocalHandler.handleProviderConnect) share this file's validation and
// extraction logic, keeping the semantics identical: auth happens before
// session registration, and a rejection rejects the connection without
// producing a half-registered session.
package agent

import (
	"crypto/subtle"
	"fmt"
	"strings"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// ProviderAuthMetadataKey is the handshake auth key inside
// ProviderConnectRequest.metadata (a platform reserved key, see
// agentlocal.ReservedMetadataKeys — it never enters session metadata, and a
// user-declared key of the same name is always dropped).
const ProviderAuthMetadataKey = "authToken"

// SetProviderAuth configures Provider handshake auth. require=true with an
// empty token is treated as a configuration error: every handshake will be
// rejected (fail-closed — prefer denying service over running unauthenticated).
// The startup side should intercept this during config validation; this code
// only guarantees it does not silently open up.
func (h *LocalHandler) SetProviderAuth(require bool, token string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.providerAuthRequired = require
	h.providerAuthToken = strings.TrimSpace(token)
}

// ProviderAuthConfig returns the current handshake auth config (read by the
// TCP local listener path; shared with the mux path).
func (h *LocalHandler) ProviderAuthConfig() (require bool, token string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.providerAuthRequired, h.providerAuthToken
}

// extractProviderAuthToken extracts the handshake token from
// ProviderConnectRequest (trimmed). It must be called before
// MergeUserProviderMetadata — merging drops reserved keys (including
// authToken), after which it is no longer readable.
func extractProviderAuthToken(req *sdkv1.ProviderConnectRequest) string {
	return strings.TrimSpace(req.GetMetadata()[ProviderAuthMetadataKey])
}

// checkProviderAuth validates the handshake token. Returning an error rejects
// the handshake (the caller must not register the session). With
// require=false it always passes (the token is ignored entirely — without
// enforcement, a missing or wrong token does not affect registration). The
// token comparison uses constant-time comparison to prevent timing side
// channels.
func checkProviderAuth(require bool, expected, got string) error {
	if !require {
		return nil
	}
	if expected == "" {
		// configuration error (require without a token): fail-closed, reject all.
		return fmt.Errorf("provider auth rejected: requireProviderAuth is on but no token configured")
	}
	if got == "" {
		return fmt.Errorf("provider auth rejected: missing %s in handshake metadata", ProviderAuthMetadataKey)
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(got)) != 1 {
		return fmt.Errorf("provider auth rejected: %s mismatch", ProviderAuthMetadataKey)
	}
	return nil
}
