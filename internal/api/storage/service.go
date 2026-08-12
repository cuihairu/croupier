package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) requireStore() (objstore.Store, error) {
	if s == nil || s.svcCtx == nil || s.svcCtx.ObjectStore == nil {
		return nil, errorx.NewBadRequest("对象存储未配置")
	}
	return s.svcCtx.ObjectStore, nil
}

func normalizeStoragePath(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "\\", "/")
	hadTrailingSlash := strings.HasSuffix(raw, "/")
	raw = strings.TrimLeft(raw, "/")

	parts := strings.Split(raw, "/")
	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		cleanParts = append(cleanParts, part)
	}
	clean := path.Join(cleanParts...)
	if clean == "." {
		clean = ""
	}
	if clean != "" && hadTrailingSlash {
		clean += "/"
	}
	return clean
}

// SignedUrl generates a signed URL for object access
func (s *Service) SignedUrl(ctx context.Context, req *SignedUrlRequest) (*SignedUrlResponse, error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	path := normalizeStoragePath(req.Path)
	if path == "" {
		return nil, errorx.NewBadRequest("path is required")
	}
	expiry := time.Duration(req.Expire) * time.Second
	url, err := store.SignedURL(ctx, path, "GET", expiry)
	if err != nil {
		return nil, errorx.NewInternalError(fmt.Sprintf("获取签名链接失败: %v", err))
	}
	return &SignedUrlResponse{URL: url}, nil
}

// ListObjects lists objects in the storage
func (s *Service) ListObjects(ctx context.Context, req *ListObjectsRequest) (*ListObjectsResponse, error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	prefix := normalizeStoragePath(req.Prefix)
	if prefix != "" && strings.HasSuffix(req.Prefix, "/") && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	limit := req.MaxKeys
	if limit <= 0 {
		limit = req.Limit
	}
	result, err := store.List(ctx, prefix, normalizeStoragePath(req.Marker), req.Delimiter, limit)
	if err != nil {
		return nil, errorx.NewInternalError(fmt.Sprintf("获取对象列表失败: %v", err))
	}

	objects := make([]ObjectInfo, 0, len(result.Objects))
	for _, obj := range result.Objects {
		objects = append(objects, ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified.Format(time.RFC3339),
			ETag:         obj.ETag,
			StorageClass: obj.StorageClass,
		})
	}

	return &ListObjectsResponse{
		Objects:     objects,
		Prefixes:    result.Prefixes,
		IsTruncated: result.IsTruncated,
		NextMarker:  result.NextMarker,
	}, nil
}

// UploadObject uploads an object to storage
func (s *Service) UploadObject(ctx context.Context, req *UploadObjectRequest) (*UploadObjectResponse, error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}

	path := normalizeStoragePath(req.Path)
	if path == "" {
		path = normalizeStoragePath(req.PreassignedID)
	}
	if path == "" {
		path = normalizeStoragePath(req.OriginalName)
	}
	if path == "" {
		return nil, errorx.NewBadRequest("path is required")
	}

	reader := req.File
	size := req.Size
	contentType := strings.TrimSpace(req.ContentType)
	if reader == nil {
		if strings.TrimSpace(req.Content) == "" {
			return nil, errorx.NewBadRequest("file is required")
		}
		payload := strings.NewReader(req.Content)
		reader = payload
		size = int64(payload.Len())
		if contentType == "" {
			contentType = "text/plain; charset=utf-8"
		}
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, errorx.NewInternalError(fmt.Sprintf("重置上传流失败: %v", err))
	}
	if err := store.Put(ctx, path, reader, size, contentType); err != nil {
		return nil, errorx.NewInternalError(fmt.Sprintf("上传对象失败: %v", err))
	}
	return &UploadObjectResponse{Path: path}, nil
}

// DeleteObject deletes an object from storage
func (s *Service) DeleteObject(ctx context.Context, req *DeleteObjectRequest) (*DeleteObjectResponse, error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	path := normalizeStoragePath(req.Path)
	if path == "" {
		return nil, errorx.NewBadRequest("path is required")
	}
	if strings.HasSuffix(req.Path, "/") && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	if err := store.Delete(ctx, path); err != nil {
		return nil, errorx.NewInternalError(fmt.Sprintf("删除对象失败: %v", err))
	}
	return &DeleteObjectResponse{Path: path}, nil
}

// BatchDeleteObjects batch deletes objects from storage
func (s *Service) BatchDeleteObjects(ctx context.Context, req *BatchDeleteObjectsRequest) (*BatchDeleteObjectsResponse, error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	if len(req.Paths) == 0 {
		return nil, errorx.NewBadRequest("paths is required")
	}

	resp := &BatchDeleteObjectsResponse{
		Deleted: make([]string, 0, len(req.Paths)),
		Failed:  make([]string, 0),
	}
	for _, rawPath := range req.Paths {
		path := normalizeStoragePath(rawPath)
		if path == "" {
			continue
		}
		if strings.HasSuffix(rawPath, "/") && !strings.HasSuffix(path, "/") {
			path += "/"
		}
		if err := store.Delete(ctx, path); err != nil {
			slog.ErrorContext(ctx, "批量删除对象失败", "path", path, "error", err)
			resp.Failed = append(resp.Failed, path)
			continue
		}
		resp.Deleted = append(resp.Deleted, path)
	}
	return resp, nil
}

// CreateDirectory creates a directory in storage
func (s *Service) CreateDirectory(ctx context.Context, req *CreateDirectoryRequest) (*CreateDirectoryResponse, error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}

	prefix := normalizeStoragePath(req.Prefix)
	if prefix == "" {
		return nil, errorx.NewBadRequest("prefix is required")
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	if err := store.CreatePrefix(ctx, prefix); err != nil {
		slog.ErrorContext(ctx, "创建目录失败", "prefix", prefix, "error", err)
		return nil, errorx.NewInternalError(fmt.Sprintf("创建目录失败: %v", err))
	}

	slog.InfoContext(ctx, "创建目录成功", "prefix", prefix)
	return &CreateDirectoryResponse{Prefix: prefix}, nil
}

// RenameDirectory renames a directory in storage
func (s *Service) RenameDirectory(ctx context.Context, req *RenameDirectoryRequest) (*RenameDirectoryResponse, error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	oldPrefix := normalizeStoragePath(req.OldPrefix)
	newPrefix := normalizeStoragePath(req.NewPrefix)
	if oldPrefix == "" || newPrefix == "" {
		return nil, errorx.NewBadRequest("oldPrefix and newPrefix are required")
	}
	if !strings.HasSuffix(oldPrefix, "/") {
		oldPrefix += "/"
	}
	if !strings.HasSuffix(newPrefix, "/") {
		newPrefix += "/"
	}
	if err := store.RenamePrefix(ctx, oldPrefix, newPrefix); err != nil {
		return nil, errorx.NewInternalError(fmt.Sprintf("重命名目录失败: %v", err))
	}
	return &RenameDirectoryResponse{
		OldPrefix: oldPrefix,
		NewPrefix: newPrefix,
	}, nil
}
