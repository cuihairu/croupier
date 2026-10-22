// Package agent keeps the local gateway assembly during the sidecar migration.
package agent

import (
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/agents/sidecar"
)

// ProviderKeepalive is the sidecar provider session keepalive probe.
type ProviderKeepalive = sidecar.ProviderKeepalive

// NewProviderKeepalive forwards to the role implementation.
func NewProviderKeepalive(store *ProviderSessionStore, interval time.Duration, logger *slog.Logger) *ProviderKeepalive {
	return sidecar.NewProviderKeepalive(store, interval, logger)
}
