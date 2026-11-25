package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	versionutil "github.com/cuihairu/croupier/internal/common/version"
	"github.com/cuihairu/croupier/internal/core/extension/externalfunc"
	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

func (a *App) ApplyExtensionSyncPayload(payload *extensionsync.AgentSyncPayload) (*ExtensionRuntimeApplyResult, error) {
	if a == nil || a.extensionRuntime == nil {
		return nil, errors.New("extension runtime not initialized")
	}
	result, err := a.extensionRuntime.ApplyPayload(payload)
	if err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	if err := a.syncExtensionsFromRuntime(context.Background()); err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	return result, nil
}

func (a *App) ApplyExtensionSyncPayloadJSON(raw []byte) (*ExtensionRuntimeApplyResult, error) {
	if a == nil || a.extensionRuntime == nil {
		return nil, errors.New("extension runtime not initialized")
	}
	var payload extensionsync.AgentSyncPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	// The only two error paths of ExtensionRuntime.ApplyPayload are a nil
	// receiver and a nil payload: the former is already excluded by the nil
	// check above, and the &payload passed here is never nil. The err branch
	// was dead code and has been removed.
	result, _ := a.extensionRuntime.ApplyPayload(&payload)
	if err := a.syncExtensionsFromRuntime(context.Background()); err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	return result, nil
}

func (a *App) ReconcileExtensions() (*ExtensionRuntimeApplyResult, error) {
	if a == nil || a.extensionRuntime == nil {
		return nil, errors.New("extension runtime not initialized")
	}
	result, err := a.extensionRuntime.Reload()
	if err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	if err := a.syncExtensionsFromRuntime(context.Background()); err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	return result, nil
}

func (a *App) PullExtensionSyncOnce(ctx context.Context) error {
	if a == nil || a.extensionPuller == nil {
		return errors.New("extension sync puller not initialized")
	}
	if err := a.extensionPuller.PullOnce(ctx); err != nil {
		a.extensionRuntime.RecordError(err)
		return err
	}
	if err := a.syncExtensionsFromRuntime(ctx); err != nil {
		a.extensionRuntime.RecordError(err)
		return err
	}
	return nil
}

func (a *App) syncExtensionsFromRuntime(ctx context.Context) error {
	if a == nil || a.extensionRuntime == nil {
		return nil
	}
	snap := a.extensionRuntime.Snapshot()
	if a.providerManager != nil {
		// The implementation of ProviderManager.SyncExtensionProviders never
		// returns a non-nil error (initProvider failures are only logged and
		// continued past; the function ends with a fixed return nil), so the
		// err branch was dead code and has been removed.
		entries := buildExtensionProviderEntries(snap)
		_ = a.providerManager.SyncExtensionProviders(ctx, entries)
	}
	if a.extensionDrivers != nil {
		if _, err := a.extensionDrivers.Sync(ctx, snap); err != nil {
			return err
		}
	}
	a.syncExtensionFunctionsFromRuntime()
	return nil
}

func (a *App) syncExtensionFunctionsFromRuntime() {
	if a == nil || a.store == nil || a.extensionRuntime == nil {
		return
	}
	snap := a.extensionRuntime.Snapshot()
	routes := map[string]extensionFunctionRoute{}
	for _, item := range snap.Installations {
		providerID := "extension:" + strconv.FormatUint(uint64(item.InstallationID), 10)
		funcs := discoverExtensionFunctions(item)
		if len(funcs) > 0 {
			// Extensions have no serviceID; pass an empty string
			a.store.Register(providerID, "", "", versionutil.ValidOrDefault(item.ReleaseVersion), funcs, nil)
		} else {
			a.store.RemoveProvider(providerID)
		}
		for _, fn := range funcs {
			// Every element of discoverExtensionFunctions is built via
			// pushWithResource: never nil, and Id is the TrimSpace'd fid,
			// which is non-empty, so the nil/empty-ID guard branch was dead
			// code and has been removed.
			routes[strings.TrimSpace(fn.GetId())] = extensionFunctionRoute{
				InstallationID: item.InstallationID,
				Driver:         resolveFunctionDriver(item, fn.GetId()),
			}
		}
	}
	a.extensionMu.Lock()
	a.extensionRoutes = routes
	a.extensionMu.Unlock()
}

func discoverExtensionFunctions(item RuntimeInstallation) []*sdkv1.ProviderFunctionDescriptor {
	out := make([]*sdkv1.ProviderFunctionDescriptor, 0)
	seen := map[string]bool{}
	pushWithResource := func(id, operation, resource string, tags []string) {
		fid := strings.TrimSpace(id)
		if fid == "" || seen[fid] {
			return
		}
		seen[fid] = true
		out = append(out, &sdkv1.ProviderFunctionDescriptor{
			Id:          fid,
			Version:     versionutil.ValidOrDefault(item.ReleaseVersion),
			Risk:        "unknown",
			Resource:    firstNonEmpty(resource, inferFunctionResource(fid), item.ExtensionID),
			Operation:   firstNonEmpty(operation, inferFunctionOperation(fid), "custom"),
			Tags:        append([]string{"extension", item.ExtensionID}, tags...),
			Summary:     "Extension function",
			Description: "Discovered from extension runtime binding",
			Enabled:     true,
		})
	}
	push := func(id, operation, resource string, tags []string) {
		pushWithResource(id, operation, resource, tags)
	}
	for _, f := range discoverExternalPlatformFunctions(item) {
		pushWithResource(
			f.FunctionID,
			f.Operation,
			f.Provider,
			[]string{"external-platform", f.Provider, "capability:" + f.Capability},
		)
	}

	for _, b := range item.Bindings {
		bt := strings.ToLower(strings.TrimSpace(b.BindingType))
		key := strings.TrimSpace(b.BindingKey)
		switch bt {
		case "function":
			op := valueString(b.Spec, "operation")
			resource := valueString(b.Spec, "resource")
			push(firstNonEmpty(key, valueString(b.Spec, "function_id"), valueString(b.Spec, "id")), op, resource, nil)
		case "capability":
			capID := firstNonEmpty(key, valueString(b.Spec, "capability"), valueString(b.Spec, "id"))
			ops := valueStringSlice(b.Spec, "operations")
			if len(ops) == 0 {
				push(capID, "custom", capID, []string{"capability"})
				continue
			}
			for _, op := range ops {
				push(capID+"."+sanitizeNodeKey(op), op, capID, []string{"capability"})
			}
		case "operation":
			capID := firstNonEmpty(valueString(b.Spec, "capability"), key)
			op := firstNonEmpty(valueString(b.Spec, "operation"), valueString(b.Spec, "name"), "custom")
			if capID != "" {
				push(capID+"."+sanitizeNodeKey(op), op, capID, []string{"capability"})
			}
		}
	}
	return out
}

func inferFunctionResource(functionID string) string {
	functionID = strings.TrimSpace(functionID)
	if idx := strings.Index(functionID, "."); idx > 0 {
		return functionID[:idx]
	}
	return functionID
}

func inferFunctionOperation(functionID string) string {
	functionID = strings.TrimSpace(functionID)
	if idx := strings.Index(functionID, "."); idx >= 0 && idx+1 < len(functionID) {
		return functionID[idx+1:]
	}
	return ""
}

type externalPlatformFunction struct {
	Provider   string
	Capability string
	Operation  string
	FunctionID string
}

func discoverExternalPlatformFunctions(item RuntimeInstallation) []externalPlatformFunction {
	if !externalfunc.IsExternalPlatformExtensionID(item.ExtensionID) {
		return nil
	}
	out := make([]externalPlatformFunction, 0)
	seen := map[string]struct{}{}
	bindings := make([]externalfunc.Binding, 0, len(item.Bindings))
	for _, b := range item.Bindings {
		bindings = append(bindings, externalfunc.Binding{
			BindingType: b.BindingType,
			BindingKey:  b.BindingKey,
			Spec:        b.Spec,
		})
	}
	for provider, operations := range externalfunc.DiscoverProviderOperations(bindings) {
		for _, opKey := range operations {
			fid := externalfunc.BuildFunctionID(provider, opKey)
			if _, exists := seen[fid]; exists {
				continue
			}
			seen[fid] = struct{}{}
			out = append(out, externalPlatformFunction{
				Provider:   provider,
				Capability: externalfunc.Capability(provider),
				Operation:  opKey,
				FunctionID: fid,
			})
		}
	}
	return out
}

func buildExtensionProviderEntries(snapshot ExtensionRuntimeSnapshot) map[string]ProviderEntry {
	out := map[string]ProviderEntry{}
	for _, item := range snapshot.Installations {
		if !externalfunc.IsExternalPlatformExtensionID(item.ExtensionID) {
			continue
		}
		for _, b := range item.Bindings {
			bt := strings.ToLower(strings.TrimSpace(b.BindingType))
			if bt != "provider" && bt != "openapi" {
				continue
			}
			parsed, ok := externalfunc.ParseProviderBinding(b.BindingKey, b.Spec)
			if !ok {
				continue
			}
			// A successful ParseProviderBinding guarantees Provider is a
			// non-empty product of SanitizeKey, and sanitizeNodeKey is
			// character-for-character equivalent to externalfunc.SanitizeKey
			// in its char mapping and edge Trim rules (idempotent), so name is
			// always non-empty; the empty-name branch was dead code and has
			// been removed.
			name := sanitizeNodeKey(parsed.Provider)
			cfg := map[string]interface{}{}
			for k, v := range parsed.Config {
				cfg[k] = v
			}
			if len(parsed.Operations) > 0 {
				if _, exists := cfg["methods"]; !exists {
					methods := make([]map[string]any, 0, len(parsed.Operations))
					for _, opKey := range parsed.Operations {
						methods = append(methods, map[string]any{
							"name":   opKey,
							"path":   "/" + opKey,
							"method": "POST",
						})
					}
					if len(methods) > 0 {
						cfg["methods"] = methods
					}
				}
			}
			out[name] = ProviderEntry{
				Enabled: parsed.Enabled,
				Type:    parsed.Type,
				Config:  cfg,
			}
		}
	}
	return out
}

func valueString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func valueStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s := strings.TrimSpace(fmt.Sprint(item))
		if s == "" || s == "<nil>" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (a *App) extensionRuntimeDynamicLabels() map[string]string {
	if a == nil || a.extensionRuntime == nil {
		return nil
	}
	snap := a.extensionRuntime.Snapshot()
	enabled := 0
	bindings := 0
	for _, item := range snap.Installations {
		if item.Enabled {
			enabled++
		}
		bindings += len(item.Bindings)
	}
	labels := map[string]string{
		"extensions.runtime.installations": strconv.Itoa(len(snap.Installations)),
		"extensions.runtime.enabled":       strconv.Itoa(enabled),
		"extensions.runtime.bindings":      strconv.Itoa(bindings),
		"extensions.runtime.status":        firstNonEmpty(snap.LastApplyStatus, "unknown"),
		"extensions.runtime.applied_at":    strconv.FormatInt(snap.AppliedAt, 10),
		"extensions.runtime.last_error_at": strconv.FormatInt(snap.LastErrorAt, 10),
		"extensions.runtime.last_failed":   strconv.Itoa(snap.LastFailed),
	}
	if snap.LastError != "" {
		labels["extensions.runtime.last_error"] = truncateString(snap.LastError, 160)
	}
	if a.extensionDrivers != nil {
		stats := a.extensionDrivers.LastResult()
		labels["extensions.runtime.driver_init"] = strconv.Itoa(stats.Initialized)
		labels["extensions.runtime.driver_reload"] = strconv.Itoa(stats.Reloaded)
		labels["extensions.runtime.driver_stop"] = strconv.Itoa(stats.Stopped)
		labels["extensions.runtime.driver_failed"] = strconv.Itoa(stats.Failed)
	}
	return labels
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

type extensionFunctionRoute struct {
	InstallationID uint
	Driver         string
}

func resolveFunctionDriver(item RuntimeInstallation, functionID string) string {
	fid := strings.TrimSpace(functionID)
	if _, _, ok := externalfunc.ParseFunctionID(fid); ok {
		return "openapi-driver"
	}
	for _, b := range item.Bindings {
		key := strings.TrimSpace(b.BindingKey)
		if key != "" && key != fid {
			continue
		}
		if d := valueString(b.Spec, "driver"); d != "" {
			return d
		}
		names := resolveDriverNames(RuntimeInstallation{Bindings: []RuntimeBinding{b}})
		if len(names) > 0 {
			return names[0]
		}
	}
	return "workflow-driver"
}

func (a *App) hasExtensionFunction(functionID string) bool {
	if a == nil {
		return false
	}
	key := strings.TrimSpace(functionID)
	if key == "" {
		return false
	}
	a.extensionMu.RLock()
	defer a.extensionMu.RUnlock()
	_, ok := a.extensionRoutes[key]
	return ok
}

func (a *App) invokeExtensionFunction(ctx context.Context, functionID string, payload []byte) ([]byte, error) {
	if a == nil || a.extensionDrivers == nil {
		return nil, errors.New("extension drivers not initialized")
	}
	key := strings.TrimSpace(functionID)
	if key == "" {
		return nil, errors.New("function id is required")
	}
	a.extensionMu.RLock()
	route, ok := a.extensionRoutes[key]
	a.extensionMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("extension function not found: %s", key)
	}
	driver := firstNonEmpty(route.Driver, "workflow-driver")
	if driver != "openapi-driver" {
		if out, handled, err := invokeExternalPlatformFunction(ctx, key, payload, func(ctx context.Context, provider, method string, request []byte) ([]byte, error) {
			if a.providerManager == nil {
				return nil, errors.New("provider manager not initialized")
			}
			return a.providerManager.Call(ctx, provider+"."+method, request)
		}); handled {
			return out, err
		}
	}
	return a.extensionDrivers.Invoke(ctx, driver, key, payload)
}
