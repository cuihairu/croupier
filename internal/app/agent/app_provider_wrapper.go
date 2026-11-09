package agent

import (
	"context"
	"errors"
)

// providerManagerWrapper wraps ProviderManager to implement agent.ProviderManager interface
type providerManagerWrapper struct {
	pm  *ProviderManager
	app *App
}

func (w *providerManagerWrapper) IsPlatformFunction(functionID string) bool {
	if w != nil && w.app != nil && w.app.hasExtensionFunction(functionID) {
		return true
	}
	if w == nil || w.pm == nil {
		return false
	}
	return w.pm.IsPlatformFunction(functionID)
}

func (w *providerManagerWrapper) Call(ctx context.Context, functionID string, request []byte) ([]byte, error) {
	if w != nil && w.app != nil && w.app.hasExtensionFunction(functionID) {
		return w.app.invokeExtensionFunction(ctx, functionID, request)
	}
	if w == nil || w.pm == nil {
		return nil, errors.New("provider manager not initialized")
	}
	return w.pm.Call(ctx, functionID, request)
}
