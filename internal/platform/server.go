package platform

import (
	"context"
	"encoding/json"
	"log/slog"

	platformv1 "github.com/cuihairu/croupier/generated/platform/v1"
	"github.com/cuihairu/croupier/internal/platform/provider"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server implements the PlatformService gRPC server.
type Server struct {
	platformv1.UnimplementedPlatformServiceServer
	loader *Loader
	logger *slog.Logger
}

// NewServer creates a new PlatformService server.
func NewServer(loader *Loader, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		loader: loader,
		logger: logger,
	}
}

// CallPlatform invokes a method on a third-party platform.
func (s *Server) CallPlatform(ctx context.Context, req *platformv1.CallPlatformRequest) (*platformv1.CallPlatformResponse, error) {
	s.logger.Debug("CallPlatform", "platform", req.Platform, "method", req.Method)

	// Validate request
	if req.Platform == "" {
		return nil, status.Error(codes.InvalidArgument, "platform is required")
	}
	if req.Method == "" {
		return nil, status.Error(codes.InvalidArgument, "method is required")
	}

	// Call through registry
	response, err := s.loader.Registry().Call(ctx, req.Platform, req.Method, req.Request)
	if err != nil {
		// Check for specific error types
		if _, ok := err.(*provider.ProviderNotFoundError); ok {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if _, ok := err.(*provider.ProviderDisabledError); ok {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if _, ok := err.(*provider.MethodNotSupportedError); ok {
			return nil, status.Error(codes.Unimplemented, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &platformv1.CallPlatformResponse{
		Response: response,
	}, nil
}

// ListPlatforms returns all available platforms.
func (s *Server) ListPlatforms(ctx context.Context, req *platformv1.ListPlatformsRequest) (*platformv1.ListPlatformsResponse, error) {
	providers := s.loader.ListProviders()

	platforms := make([]*platformv1.PlatformInfo, 0, len(providers))
	for _, p := range providers {
		platforms = append(platforms, &platformv1.PlatformInfo{
			Name:    p.Name(),
			Enabled: p.IsEnabled(),
			Methods: p.SupportedMethods(),
		})
	}

	return &platformv1.ListPlatformsResponse{
		Platforms: platforms,
	}, nil
}

// ListPlatformMethods returns the methods supported by a platform.
func (s *Server) ListPlatformMethods(ctx context.Context, req *platformv1.ListPlatformMethodsRequest) (*platformv1.ListPlatformMethodsResponse, error) {
	if req.Platform == "" {
		return nil, status.Error(codes.InvalidArgument, "platform is required")
	}

	p, exists := s.loader.GetProvider(req.Platform)
	if !exists {
		return nil, status.Error(codes.NotFound, "platform not found")
	}

	return &platformv1.ListPlatformMethodsResponse{
		Methods: p.SupportedMethods(),
	}, nil
}

// ReloadPlatformConfig reloads the platform configuration.
func (s *Server) ReloadPlatformConfig(ctx context.Context, req *emptypb.Empty) (*platformv1.ReloadPlatformConfigResponse, error) {
	s.logger.Info("reloading platform configuration")

	if err := s.loader.Reload(ctx); err != nil {
		s.logger.Error("failed to reload platform configuration", "error", err)
		return &platformv1.ReloadPlatformConfigResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &platformv1.ReloadPlatformConfigResponse{
		Success: true,
		Message: "platform configuration reloaded successfully",
	}, nil
}

// MarshalRequest converts a struct to JSON bytes for the request.
func MarshalRequest(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalResponse parses JSON response bytes into a struct.
func UnmarshalResponse(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
