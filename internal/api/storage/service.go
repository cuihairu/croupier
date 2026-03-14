package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// SignedUrl generates a signed URL for object access
func (s *Service) SignedUrl(ctx context.Context, req *SignedUrlRequest) (*SignedUrlResponse, error) {
	return nil, errorx.NewNotImplemented("SignedUrl not implemented")
}

// ListObjects lists objects in the storage
func (s *Service) ListObjects(ctx context.Context, req *ListObjectsRequest) (*ListObjectsResponse, error) {
	return nil, errorx.NewNotImplemented("ListObjects not implemented")
}

// UploadObject uploads an object to storage
func (s *Service) UploadObject(ctx context.Context, req *UploadObjectRequest) (*UploadObjectResponse, error) {
	return nil, errorx.NewNotImplemented("UploadObject not implemented")
}

// DeleteObject deletes an object from storage
func (s *Service) DeleteObject(ctx context.Context, req *DeleteObjectRequest) (*DeleteObjectResponse, error) {
	return nil, errorx.NewNotImplemented("DeleteObject not implemented")
}

// BatchDeleteObjects batch deletes objects from storage
func (s *Service) BatchDeleteObjects(ctx context.Context, req *BatchDeleteObjectsRequest) (*BatchDeleteObjectsResponse, error) {
	return nil, errorx.NewNotImplemented("BatchDeleteObjects not implemented")
}

// CreateDirectory creates a directory in storage
func (s *Service) CreateDirectory(ctx context.Context, req *CreateDirectoryRequest) (*CreateDirectoryResponse, error) {
	// Check if object store is initialized
	if s.svcCtx.ObjectStore == nil {
		return &CreateDirectoryResponse{
			Code:    -1,
			Message: "对象存储未配置",
		}, nil
	}

	// Validate request parameters
	if req.Prefix == "" {
		return &CreateDirectoryResponse{
			Code:    -1,
			Message: "目录前缀不能为空",
		}, nil
	}

	// Create directory
	err := s.svcCtx.ObjectStore.CreatePrefix(ctx, req.Prefix)
	if err != nil {
		slog.ErrorContext(ctx, "创建目录失败", "prefix", req.Prefix, "error", err)
		return &CreateDirectoryResponse{
			Code:    -1,
			Message: fmt.Sprintf("创建目录失败: %v", err),
		}, nil
	}

	slog.InfoContext(ctx, "创建目录成功", "prefix", req.Prefix)
	return &CreateDirectoryResponse{
		Code:    0,
		Message: "OK",
	}, nil
}

// RenameDirectory renames a directory in storage
func (s *Service) RenameDirectory(ctx context.Context, req *RenameDirectoryRequest) (*RenameDirectoryResponse, error) {
	return nil, errorx.NewNotImplemented("RenameDirectory not implemented")
}
