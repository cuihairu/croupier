package platform

import (
	"context"
	"encoding/json"

	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// Call calls a platform method
func (s *Service) Call(ctx context.Context, req *CallPlatformRequest) (*CallPlatformResponse, error) {
	// Check if platform loader is initialized
	if s.svcCtx.PlatformLoader == nil {
		return &CallPlatformResponse{
			Code:    503,
			Message: "Platform integration is not enabled",
		}, nil
	}

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

	// Call platform API through registry
	response, err := s.svcCtx.PlatformLoader.Registry().Call(ctx, req.Platform, req.Method, requestData)
	if err != nil {
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
		Code:     200,
		Message:  "success",
		Response: result,
	}, nil
}

// ListPlatforms lists all available platforms
func (s *Service) ListPlatforms(ctx context.Context) (*ListPlatformsResponse, error) {
	// If platform loader is not initialized, return empty list
	if s.svcCtx.PlatformLoader == nil {
		return &ListPlatformsResponse{
			Code:      200,
			Message:   "success",
			Platforms: []PlatformInfo{},
		}, nil
	}

	// Get all registered platform providers
	providers := s.svcCtx.PlatformLoader.ListProviders()
	platforms := make([]PlatformInfo, 0, len(providers))

	for _, p := range providers {
		platforms = append(platforms, PlatformInfo{
			Name:    p.Name(),
			Enabled: p.IsEnabled(),
			Methods: p.SupportedMethods(),
		})
	}

	return &ListPlatformsResponse{
		Code:      200,
		Message:   "success",
		Platforms: platforms,
	}, nil
}

// ListMethods lists methods for a platform
func (s *Service) ListMethods(ctx context.Context, platform string) (*ListPlatformMethodsResponse, error) {
	if s.svcCtx.PlatformLoader == nil {
		return &ListPlatformMethodsResponse{
			Code:    503,
			Message: "Platform integration is not enabled",
			Methods: []string{},
		}, nil
	}

	provider, _ := s.svcCtx.PlatformLoader.GetProvider(platform)
	if provider == nil {
		return &ListPlatformMethodsResponse{
			Code:    404,
			Message: "Platform not found",
			Methods: []string{},
		}, nil
	}

	return &ListPlatformMethodsResponse{
		Code:    200,
		Message: "success",
		Methods: provider.SupportedMethods(),
	}, nil
}

// ReloadConfig reloads the platform configuration
func (s *Service) ReloadConfig(ctx context.Context) (*ReloadPlatformConfigResponse, error) {
	if s.svcCtx.PlatformLoader == nil {
		return &ReloadPlatformConfigResponse{
			Code:    503,
			Message: "Platform integration is not enabled",
			Success: false,
		}, nil
	}

	if err := s.svcCtx.PlatformLoader.Reload(ctx); err != nil {
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
