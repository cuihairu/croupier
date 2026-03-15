package platform

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/core/extension/externalfunc"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/migrationflags"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
	legacy legacyPlatformGateway
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{
		svcCtx: svcCtx,
		legacy: newLegacyPlatformGateway(svcCtx),
	}
}

// Call calls a platform method
func (s *Service) Call(ctx context.Context, req *CallPlatformRequest) (*CallPlatformResponse, error) {
	// Validate required parameters
	if req.Platform == "" {
		return &CallPlatformResponse{
			Code:    400,
			Message: "platform is required",
		}, nil
	}
	if req.Method == "" {
		return &CallPlatformResponse{
			Code:    400,
			Message: "method is required",
		}, nil
	}

	// Convert JSON string request to []byte
	var requestData []byte
	if req.Request != "" {
		requestData = []byte(req.Request)
	}
	extensionOnly := isPlatformExtensionOnly()
	legacyDisabled := isPlatformLegacyDisabled()

	// Extension-first path: try invoking external.<platform>.<method> through dispatcher.
	var invokeErr error
	invokeAttempted := false
	if functionID := buildExternalFunctionID(req.Platform, req.Method); functionID != "" && s.svcCtx.Dispatcher != nil {
		invokeAttempted = true
		response, err := s.svcCtx.Dispatcher.Invoke(ctx, functionID, requestData)
		if err == nil {
			var result interface{}
			if len(response) > 0 {
				if unmarshalErr := json.Unmarshal(response, &result); unmarshalErr != nil {
					result = string(response)
				}
			}
			return &CallPlatformResponse{
				Code:     200,
				Message:  "success",
				Response: result,
				Source:   "extension",
			}, nil
		}
		invokeErr = err
	}
	if extensionOnly || legacyDisabled {
		if invokeErr != nil {
			return &CallPlatformResponse{
				Code:    500,
				Message: invokeErr.Error(),
				Source:  "extension",
			}, nil
		}
		return &CallPlatformResponse{
			Code:    503,
			Message: "Platform extension runtime is not available",
			Source:  "extension",
		}, nil
	}
	if !allowLegacyFallbackAfterExtensionError() && invokeAttempted && invokeErr != nil {
		return &CallPlatformResponse{
			Code:    500,
			Message: invokeErr.Error(),
			Source:  "extension",
		}, nil
	}

	// Fallback to legacy platform loader.
	if s.legacy == nil {
		if invokeErr != nil {
			return &CallPlatformResponse{
				Code:    500,
				Message: invokeErr.Error(),
			}, nil
		}
		return &CallPlatformResponse{
			Code:    503,
			Message: "Platform integration is not enabled",
		}, nil
	}

	fallbackUsed := invokeAttempted && invokeErr != nil
	response, err := s.legacy.Call(ctx, req.Platform, req.Method, requestData)
	if err != nil {
		if invokeErr != nil {
			return &CallPlatformResponse{
				Code:    500,
				Message: invokeErr.Error(),
			}, nil
		}
		return &CallPlatformResponse{
			Code:    500,
			Message: err.Error(),
		}, nil
	}

	// Parse response to return structured data
	var result interface{}
	if len(response) > 0 {
		_ = json.Unmarshal(response, &result)
	}

	return &CallPlatformResponse{
		Code:           200,
		Message:        "success",
		Response:       result,
		Source:         "legacy",
		Fallback:       fallbackUsed,
		FallbackReason: fallbackReason(fallbackUsed),
	}, nil
}

// ListPlatforms lists all available platforms
func (s *Service) ListPlatforms(ctx context.Context) (*ListPlatformsResponse, error) {
	discovered := s.discoverExternalPlatforms(ctx)
	platforms := make([]PlatformInfo, 0, len(discovered))
	seen := map[string]bool{}
	for name, methods := range discovered {
		platforms = append(platforms, PlatformInfo{
			Name:    name,
			Enabled: true,
			Methods: methods,
			Source:  "extension",
		})
		seen[strings.ToLower(strings.TrimSpace(name))] = true
	}

	// Merge legacy platform loader providers unless extension-only mode is enabled.
	if useLegacyPlatformFallback() && s.legacy != nil {
		providers := s.legacy.ListProviders()
		for _, p := range providers {
			name := strings.TrimSpace(p.Name)
			key := strings.ToLower(name)
			if seen[key] {
				continue
			}
			platforms = append(platforms, PlatformInfo{
				Name:    name,
				Enabled: p.Enabled,
				Methods: p.Methods,
				Source:  "legacy",
			})
			seen[key] = true
		}
	}

	return &ListPlatformsResponse{
		Code:      200,
		Message:   "success",
		Platforms: platforms,
	}, nil
}

// ListMethods lists methods for a platform
func (s *Service) ListMethods(ctx context.Context, platform string) (*ListPlatformMethodsResponse, error) {
	p := strings.TrimSpace(platform)
	if p == "" {
		return &ListPlatformMethodsResponse{
			Code:    400,
			Message: "platform is required",
			Methods: []string{},
		}, nil
	}
	discovered := s.discoverExternalPlatforms(ctx)
	merged := map[string]struct{}{}
	methods := make([]string, 0)
	addMethods := func(list []string) {
		for _, m := range list {
			key := strings.ToLower(strings.TrimSpace(m))
			if key == "" {
				continue
			}
			if _, ok := merged[key]; ok {
				continue
			}
			merged[key] = struct{}{}
			methods = append(methods, strings.TrimSpace(m))
		}
	}
	addMethods(discovered[strings.ToLower(p)])
	usedExtension := len(methods) > 0

	usedLegacy := false
	if useLegacyPlatformFallback() && s.legacy != nil {
		methodsFromLegacy, ok := s.legacy.ListMethods(platform)
		if ok {
			addMethods(methodsFromLegacy)
			usedLegacy = true
		}
	}

	if len(methods) == 0 {
		return &ListPlatformMethodsResponse{
			Code:    404,
			Message: "Platform not found",
			Methods: []string{},
		}, nil
	}

	return &ListPlatformMethodsResponse{
		Code:    200,
		Message: "success",
		Methods: methods,
		Source:  resolveMethodsSource(usedExtension, usedLegacy),
	}, nil
}

// ReloadConfig reloads the platform configuration
func (s *Service) ReloadConfig(ctx context.Context) (*ReloadPlatformConfigResponse, error) {
	if !useLegacyPlatformFallback() {
		return &ReloadPlatformConfigResponse{
			Code:    200,
			Message: "Platform legacy fallback disabled; no legacy config to reload",
			Success: true,
		}, nil
	}
	if s.legacy == nil {
		return &ReloadPlatformConfigResponse{
			Code:    503,
			Message: "Platform integration is not enabled",
			Success: false,
		}, nil
	}

	if err := s.legacy.Reload(ctx); err != nil {
		return &ReloadPlatformConfigResponse{
			Code:    500,
			Message: err.Error(),
			Success: false,
		}, nil
	}

	return &ReloadPlatformConfigResponse{
		Code:    200,
		Message: "Platform configuration reloaded successfully",
		Success: true,
	}, nil
}

func buildExternalFunctionID(platform, method string) string {
	return externalfunc.BuildFunctionID(platform, method)
}

func (s *Service) discoverExternalPlatforms(ctx context.Context) map[string][]string {
	result := map[string][]string{}
	if s.svcCtx == nil {
		return result
	}

	// 1) Discover from registry functions (runtime-ready view).
	if store := s.svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		for _, agent := range store.AgentsUnsafe() {
			if agent == nil {
				continue
			}
			for functionID, meta := range agent.Functions {
				if !meta.Enabled {
					continue
				}
				provider, method, ok := parseExternalFunctionID(functionID)
				if !ok {
					continue
				}
				if _, exists := result[provider]; !exists {
					result[provider] = []string{}
				}
				if !stringInSlice(result[provider], method) {
					result[provider] = append(result[provider], method)
				}
			}
		}
		store.Mu().RUnlock()
	}

	// 2) Discover from installation bindings (planned/declared view), useful before agent sync.
	if s.svcCtx.Extensions != nil && s.svcCtx.Extensions.Installation != nil {
		items, _, err := s.svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
			Limit:  1000,
			Offset: 0,
		})
		if err == nil {
			for _, item := range items {
				if !externalfunc.IsExternalPlatformExtensionID(item.ExtensionID) {
					continue
				}
				if strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") ||
					strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled") {
					continue
				}
				bindings, bindErr := s.svcCtx.Extensions.Installation.ListBindings(ctx, item.ID)
				if bindErr != nil {
					continue
				}
				declared := extractPlatformMethodsFromBindings(bindings)
				for provider, methods := range declared {
					if _, exists := result[provider]; !exists {
						result[provider] = []string{}
					}
					for _, method := range methods {
						if !stringInSlice(result[provider], method) {
							result[provider] = append(result[provider], method)
						}
					}
				}
			}
		}
	}
	return result
}

func extractPlatformMethodsFromBindings(bindings []model.ExtensionRuntimeBinding) map[string][]string {
	inputs := make([]externalfunc.Binding, 0, len(bindings))
	for _, b := range bindings {
		spec := map[string]any{}
		if strings.TrimSpace(b.SpecJSON) != "" {
			_ = json.Unmarshal([]byte(b.SpecJSON), &spec)
		}
		inputs = append(inputs, externalfunc.Binding{
			BindingType: b.BindingType,
			BindingKey:  b.BindingKey,
			Spec:        spec,
		})
	}
	return externalfunc.DiscoverProviderOperations(inputs)
}

func parseExternalFunctionID(functionID string) (provider string, method string, ok bool) {
	return externalfunc.ParseFunctionID(functionID)
}

func stringInSlice(list []string, target string) bool {
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func resolveMethodsSource(useExtension, useLegacy bool) string {
	switch {
	case useExtension && useLegacy:
		return "mixed"
	case useExtension:
		return "extension"
	case useLegacy:
		return "legacy"
	default:
		return ""
	}
}

func isPlatformExtensionOnly() bool {
	return migrationflags.IsExtensionOnly()
}

func isPlatformLegacyDisabled() bool {
	return migrationflags.IsLegacyDisabled()
}

func useLegacyPlatformFallback() bool {
	return migrationflags.UseLegacyFallback()
}

func allowLegacyFallbackAfterExtensionError() bool {
	return migrationflags.AllowLegacyFallbackAfterExtensionError()
}

func fallbackReason(fallback bool) string {
	if fallback {
		return "extension_error"
	}
	return ""
}
