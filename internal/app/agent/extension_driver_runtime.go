package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ExtensionDriver defines minimal lifecycle hooks for agent-side extension drivers.
type ExtensionDriver interface {
	Name() string
	Init(ctx context.Context, installation RuntimeInstallation) error
	Reload(ctx context.Context, installation RuntimeInstallation) error
	Stop(ctx context.Context, installationID uint) error
	Invoke(ctx context.Context, functionID string, payload []byte) ([]byte, error)
}

type ExtensionDriverSyncResult struct {
	Initialized int `json:"initialized"`
	Reloaded    int `json:"reloaded"`
	Stopped     int `json:"stopped"`
	Failed      int `json:"failed"`
}

type ExtensionDriverRuntime struct {
	mu          sync.RWMutex
	drivers     map[string]ExtensionDriver
	assignments map[uint]map[string]struct{}
	lastResult  ExtensionDriverSyncResult
	openapi     *openapiExtensionDriver
}

func NewExtensionDriverRuntime() *ExtensionDriverRuntime {
	openapi := NewOpenAPIExtensionDriver()
	rt := &ExtensionDriverRuntime{
		drivers:     map[string]ExtensionDriver{},
		assignments: map[uint]map[string]struct{}{},
		openapi:     openapi,
	}
	// Built-in drivers.
	rt.RegisterDriver(openapi)
	rt.RegisterDriver(NewNoopExtensionDriver("webhook-driver"))
	rt.RegisterDriver(NewNoopExtensionDriver("grpc-driver"))
	rt.RegisterDriver(NewNoopExtensionDriver("workflow-driver"))
	rt.RegisterDriver(NewNoopExtensionDriver("internal-ui-driver"))
	return rt
}

func (r *ExtensionDriverRuntime) SetOpenAPICaller(call externalCallFunc) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.openapi != nil {
		r.openapi.SetCaller(call)
	}
}

func (r *ExtensionDriverRuntime) RegisterDriver(driver ExtensionDriver) {
	if r == nil || driver == nil {
		return
	}
	name := strings.TrimSpace(driver.Name())
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers[name] = driver
}

func (r *ExtensionDriverRuntime) Sync(ctx context.Context, snapshot ExtensionRuntimeSnapshot) (ExtensionDriverSyncResult, error) {
	if r == nil {
		return ExtensionDriverSyncResult{}, fmt.Errorf("extension driver runtime is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	result := ExtensionDriverSyncResult{}
	nextAssignments := map[uint]map[string]struct{}{}
	var errs []string

	for _, item := range snapshot.Installations {
		names := resolveDriverNames(item)
		next := map[string]struct{}{}
		prev := r.assignments[item.InstallationID]
		for _, name := range names {
			driver, ok := r.drivers[name]
			if !ok {
				result.Failed++
				errs = append(errs, "driver not found: "+name)
				continue
			}
			if _, existed := prev[name]; existed {
				if err := driver.Reload(ctx, item); err != nil {
					result.Failed++
					errs = append(errs, name+": "+err.Error())
					continue
				}
				result.Reloaded++
			} else {
				if err := driver.Init(ctx, item); err != nil {
					result.Failed++
					errs = append(errs, name+": "+err.Error())
					continue
				}
				result.Initialized++
			}
			next[name] = struct{}{}
		}
		nextAssignments[item.InstallationID] = next
	}

	for installationID, prev := range r.assignments {
		next := nextAssignments[installationID]
		for name := range prev {
			if _, keep := next[name]; keep {
				continue
			}
			if driver, ok := r.drivers[name]; ok {
				if err := driver.Stop(ctx, installationID); err != nil {
					result.Failed++
					errs = append(errs, name+": "+err.Error())
					continue
				}
			}
			result.Stopped++
		}
	}

	r.assignments = nextAssignments
	r.lastResult = result
	if len(errs) > 0 {
		return result, fmt.Errorf("extension driver sync failed: %s", strings.Join(errs, "; "))
	}
	return result, nil
}

func (r *ExtensionDriverRuntime) LastResult() ExtensionDriverSyncResult {
	if r == nil {
		return ExtensionDriverSyncResult{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastResult
}

type noopExtensionDriver struct {
	name string
}

type openapiExtensionDriver struct {
	mu   sync.RWMutex
	call externalCallFunc
}

func NewOpenAPIExtensionDriver() *openapiExtensionDriver {
	return &openapiExtensionDriver{}
}

func (d *openapiExtensionDriver) Name() string { return "openapi-driver" }

func (d *openapiExtensionDriver) Init(ctx context.Context, installation RuntimeInstallation) error {
	return nil
}

func (d *openapiExtensionDriver) Reload(ctx context.Context, installation RuntimeInstallation) error {
	return nil
}

func (d *openapiExtensionDriver) Stop(ctx context.Context, installationID uint) error {
	return nil
}

func (d *openapiExtensionDriver) SetCaller(call externalCallFunc) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.call = call
}

func (d *openapiExtensionDriver) Invoke(ctx context.Context, functionID string, payload []byte) ([]byte, error) {
	if d == nil {
		return nil, fmt.Errorf("openapi driver is nil")
	}
	d.mu.RLock()
	call := d.call
	d.mu.RUnlock()
	out, handled, err := invokeExternalPlatformFunction(ctx, functionID, payload, call)
	if handled {
		return out, err
	}
	return nil, fmt.Errorf("openapi driver only supports external functions, got: %s", strings.TrimSpace(functionID))
}

func NewNoopExtensionDriver(name string) ExtensionDriver {
	return &noopExtensionDriver{name: strings.TrimSpace(name)}
}

func (d *noopExtensionDriver) Name() string { return d.name }

func (d *noopExtensionDriver) Init(ctx context.Context, installation RuntimeInstallation) error {
	return nil
}

func (d *noopExtensionDriver) Reload(ctx context.Context, installation RuntimeInstallation) error {
	return nil
}

func (d *noopExtensionDriver) Stop(ctx context.Context, installationID uint) error {
	return nil
}

func (d *noopExtensionDriver) Invoke(ctx context.Context, functionID string, payload []byte) ([]byte, error) {
	return append([]byte(nil), payload...), nil
}

func (r *ExtensionDriverRuntime) Invoke(ctx context.Context, driverName, functionID string, payload []byte) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("extension driver runtime is nil")
	}
	key := strings.TrimSpace(driverName)
	if key == "" {
		return nil, fmt.Errorf("driver name is required")
	}
	r.mu.RLock()
	driver := r.drivers[key]
	r.mu.RUnlock()
	if driver == nil {
		return nil, fmt.Errorf("driver not found: %s", key)
	}
	return driver.Invoke(ctx, functionID, payload)
}

func resolveDriverNames(item RuntimeInstallation) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 2)
	push := func(name string) {
		key := strings.TrimSpace(name)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}

	for _, b := range item.Bindings {
		if d := valueString(b.Spec, "driver"); d != "" {
			push(d)
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(b.TargetRef)), "driver:") {
			push(strings.TrimSpace(strings.TrimPrefix(b.TargetRef, "driver:")))
			continue
		}
		switch strings.ToLower(strings.TrimSpace(b.BindingType)) {
		case "provider", "openapi":
			push("openapi-driver")
		case "webhook":
			push("webhook-driver")
		case "grpc":
			push("grpc-driver")
		case "page", "ui", "navigation":
			push("internal-ui-driver")
		case "workflow", "function", "capability", "operation":
			push("workflow-driver")
		}
	}
	if len(out) == 0 {
		push("workflow-driver")
	}
	return out
}
