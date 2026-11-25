package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

// Logout logs the user out.
// The implementation has no failure path (token revocation failure is only
// logged); the signature was tightened to drop the error return.
func (s *Service) Logout(ctx context.Context, req *LogoutRequest) *LogoutResponse {
	// Bumping token_version immediately invalidates every token issued for the
	// account (including the one the caller is currently using). Middleware
	// caching means the effect fleet-wide lands within 30s at most.
	if req != nil && req.Username != "" {
		if admin, err := s.adminModel.FindByUsername(ctx, req.Username); err == nil && admin != nil {
			if err := s.adminModel.BumpTokenVersion(ctx, admin.ID); err != nil {
				slog.Default().Warn("logout token revocation failed", "username", req.Username, "error", err)
			}
		}
	}
	return &LogoutResponse{}
}

func (s *Service) Check(ctx context.Context, username string, req *CheckRequest) (*CheckResponse, error) {
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	allowed, err := s.permSvc.CheckPermission(ctx, admin.ID, strings.TrimSpace(req.Resource), strings.TrimSpace(req.Action))
	if err != nil {
		return &CheckResponse{Allowed: false, Reason: err.Error()}, nil
	}
	if !allowed {
		return &CheckResponse{Allowed: false, Reason: "permission denied"}, nil
	}
	return &CheckResponse{Allowed: true}, nil
}

func (s *Service) BatchCheck(ctx context.Context, username string, req *BatchCheckRequest) (*BatchCheckResponse, error) {
	results := make([]CheckResponse, 0, len(req.Checks))
	for _, check := range req.Checks {
		resp, err := s.Check(ctx, username, &check)
		if err != nil {
			return nil, err
		}
		results = append(results, *resp)
	}
	return &BatchCheckResponse{Results: results}, nil
}
