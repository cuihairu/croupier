package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

// Logout 用户登出。
// 实现无出错路径（token 吊销失败仅记日志），已收紧签名去掉 error 返回。
func (s *Service) Logout(ctx context.Context, req *LogoutRequest) *LogoutResponse {
	// 递增 token_version 使该账号所有已签发 token 立即失效（含调用方
	// 当前使用的这一个）。中间件缓存意味着最长 30s 后全网生效。
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
