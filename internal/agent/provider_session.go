// Package agent keeps the local gateway assembly during the sidecar migration.
package agent

import "github.com/cuihairu/croupier/agents/sidecar"

// ProviderSession is the sidecar SDK Provider session.
type ProviderSession = sidecar.ProviderSession

// ProviderSessionStore is the sidecar session index.
type ProviderSessionStore = sidecar.ProviderSessionStore

// NewProviderSessionStore forwards to the role implementation.
func NewProviderSessionStore() *ProviderSessionStore { return sidecar.NewProviderSessionStore() }
