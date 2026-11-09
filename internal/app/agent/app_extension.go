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
	// ExtensionRuntime.ApplyPayload 仅有的两个错误路径是 nil receiver 与
	// nil payload：前者已被上方判空排除，后者传入的 &payload 恒非 nil。
	// err 分支为死代码，已删除。
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
		// ProviderManager.SyncExtensionProviders 的实现不返回任何非 nil
		// 错误（initProvider 失败仅记日志并 continue，函数尾部固定
		// return nil），err 分支为死代码，已删除。
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
			// 扩展没有 serviceID，传空字符串
			a.store.Register(providerID, "", "", versionutil.ValidOrDefault(item.ReleaseVersion), funcs, nil)
		} else {
			a.store.RemoveProvider(providerID)
		}
		for _, fn := range funcs {
			// discoverExtensionFunctions 的元素全部经 pushWithResource 构造：
			// 恒非 nil，且 Id 为 TrimSpace 后非空的 fid，nil/空 ID 防御分支
			// 为死代码，已删除。
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
			// ParseProviderBinding 成功即保证 Provider 为 SanitizeKey 的非空
			// 产出，而 sanitizeNodeKey 与 externalfunc.SanitizeKey 的字符映射、
			// 首尾 Trim 规则逐一等价（幂等），故 name 恒非空，空名分支为
			// 死代码，已删除。
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
