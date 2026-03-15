package platform

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/core/extension/externalfunc"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{
		svcCtx: svcCtx,
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
	// Extension-only path: invoke external.<platform>.<method> through dispatcher.
	if functionID := buildExternalFunctionID(req.Platform, req.Method); functionID != "" && s.svcCtx.Dispatcher != nil {
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
		return &CallPlatformResponse{
			Code:    500,
			Message: err.Error(),
			Source:  "extension",
		}, nil
	}
	return &CallPlatformResponse{
		Code:    503,
		Message: "Platform extension runtime is not available",
		Source:  "extension",
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
		Source:  resolveMethodsSource(usedExtension),
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

func resolveMethodsSource(useExtension bool) string {
	if useExtension {
		return "extension"
	}
	return ""
}
