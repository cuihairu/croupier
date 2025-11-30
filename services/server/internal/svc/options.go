package svc

import (
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// Option configures the service context.
type Option func(*ServiceContext)

// WithRegistryStore injects an existing registry store instance.
func WithRegistryStore(store *reg.Store) Option {
	return func(ctx *ServiceContext) {
		if store != nil {
			ctx.RegistryStore = store
		}
	}
}

// WithDispatcher injects a custom dispatcher implementation.
func WithDispatcher(d *dispatch.Dispatcher) Option {
	return func(ctx *ServiceContext) {
		if d != nil {
			ctx.Dispatcher = d
		}
	}
}
