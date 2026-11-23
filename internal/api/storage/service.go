package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
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

// 通用对象接口的权限 ID（AUD-016）：catalog 见 configs/permissions.json 的
// uploads:write（文件上传）；超管经 HasAdminRole 直通，无需具体权限行。
// 该权限同时把守写类操作（上传/覆盖/删除/目录变更）与常规签名访问——
// 签名链接会下发对象的临时下载能力，属同一操作域。
const storagePermission = "uploads:write"

// requirePermission 是服务层权限门禁（AUD-016，与 backup 域同范式）：登录
// 身份本身不等于操作权——此前仅凭认证即可覆盖共享备份/发布对象。
func (s *Service) requirePermission(ctx context.Context, message string, required ...string) error {
	if s == nil || s.svcCtx == nil {
		return errorx.NewInternalError("对象存储服务未初始化")
	}
	_, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, message, required...)
	return err
}

// auditStorage 是对象存储敏感写路径的统一留痕（AUD-049）：denied（权限
// 门禁拒绝）与 success（副作用完成后）共用同一事件类型，差异只在 outcome。
// actor 取请求上下文用户名（无上下文时留空，记录仍可按事件类型 + path 回查）。
func (s *Service) auditStorage(ctx context.Context, event audit.AuditEventType, state, operation, path string, extra map[string]interface{}) {
	details := map[string]interface{}{"operation": operation}
	for k, v := range extra {
		details[k] = v
	}
	actor := ""
	if name, err := utils.CurrentUsername(ctx); err == nil {
		actor = name
	}
	audit.LogWrite(s.svcCtx.AuditService, ctx, event, state, actor, "storage_object", path, details)
}

// requireStoragePermission 拒绝对通用对象存储的操作（AUD-016）。
func (s *Service) requireStoragePermission(ctx context.Context) error {
	return s.requirePermission(ctx, "无权操作对象存储", "admin:all", storagePermission)
}

// requireSignedPermission 按路径域决定签名访问所需权限（AUD-016）：受保护
// 前缀的读能力镜像所属子系统的既有授权（backups/ → backup:read，与备份
// 下载一致；releases/ → 仅超管），其余路径走存储域权限。否则仅持
// uploads:write 的操作员也能为备份对象签发下载链接，绕过备份下载授权。
func (s *Service) requireSignedPermission(ctx context.Context, p string) error {
	if kind, ok := protectedPrefixKind(p); ok {
		switch kind {
		case "backups":
			return s.requirePermission(ctx, "无权下载备份对象", "admin:all", "backup:read")
		case "releases":
			return s.requirePermission(ctx, "无权访问发布产物", "admin:all")
		}
	}
	return s.requireStoragePermission(ctx)
}

// 受保护前缀（AUD-016）：这些前缀下的对象由平台服务写入（备份执行器、
// 发布上传、归档导出），通用对象接口不得覆盖、删除或目录变更，避免低权限
// 账号或误操作破坏平台的恢复/发布能力。读取（列表/签名下载）不在此限制内，
// 由读权限把关。
var protectedStoragePrefixes = []string{
	"backups/",
	"releases/",
	"archives/",
	"exports/",
}

// protectedPrefixKind 返回路径命中的受保护前缀（按目录边界匹配）并附上
// 该前缀代表的对象类别。匹配规则：路径恰为该前缀的目录，或落在该前缀之下
// （含通配符或尾部斜杠写法），均视为命中；仅按字面前缀相等。
func protectedPrefixKind(path string) (string, bool) {
	for _, prefix := range protectedStoragePrefixes {
		// 匹配裸目录名（"backups"）与目录前缀（"backups/"、"backups/x"）两种
		// 写法：目录标记对象与目录下对象都属于平台保留命名空间。
		trimmed := strings.TrimSuffix(prefix, "/")
		if path == trimmed || strings.HasPrefix(path, prefix) {
			return trimmed, true
		}
	}
	return "", false
}

// guardWritablePath 拒绝一切指向受保护前缀的写类操作（AUD-016）。调用方
// 传入已归一化的路径；空路径由各操作自身的必填校验先行拦截。
func guardWritablePath(path string) error {
	if path == "" {
		return nil
	}
	if kind, ok := protectedPrefixKind(path); ok {
		return errorx.NewForbiddenWithCode("retention_prefix_forbidden",
			fmt.Sprintf("%s 对象由平台维护，通用对象接口不允许写入或变更", kind), nil)
	}
	return nil
}

// joinStoragePath 是 path.Join 的测试缝隙：空参数切片时 Join 返回 ""，
// normalizeStoragePath 中 clean == "." 的兜底分支真实调用不可达，
// 仅测试可注入恒返回 "." 的实现验证。
var joinStoragePath = path.Join

// normalizeStoragePath 是路径归一化函数的测试缝隙（var 形式便于注入）：
// 默认实现对非空结果恒保留尾部斜杠，ListObjects/DeleteObject/
// BatchDeleteObjects 中"补回尾部斜杠"的防御分支真实调用不可达，
// 仅测试可注入去斜杠实现验证。
var normalizeStoragePath = func(raw string) string {
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
	clean := joinStoragePath(cleanParts...)
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
	if err := s.requireSignedPermission(ctx, path); err != nil {
		return nil, err
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
	// AUD-039：列表读取面同属存储管理面，按粗粒度统一门禁（admin:all |
	// uploads:write，与 AUD-016 写面同一权限）；前缀级 ABAC 后续拆。
	if err := s.requireStoragePermission(ctx); err != nil {
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
	if err := s.requireStoragePermission(ctx); err != nil {
		s.auditStorage(ctx, audit.EventStorageUpload, audit.StateDenied, "upload", path, nil)
		return nil, err
	}
	if err := guardWritablePath(path); err != nil {
		return nil, err
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
	s.auditStorage(ctx, audit.EventStorageUpload, audit.StateSuccess, "upload", path, nil)
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
	if err := s.requireStoragePermission(ctx); err != nil {
		s.auditStorage(ctx, audit.EventStorageDelete, audit.StateDenied, "delete", path, nil)
		return nil, err
	}
	if err := guardWritablePath(path); err != nil {
		return nil, err
	}
	if strings.HasSuffix(req.Path, "/") && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	if err := store.Delete(ctx, path); err != nil {
		return nil, errorx.NewInternalError(fmt.Sprintf("删除对象失败: %v", err))
	}
	s.auditStorage(ctx, audit.EventStorageDelete, audit.StateSuccess, "delete", path, nil)
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
	if err := s.requireStoragePermission(ctx); err != nil {
		s.auditStorage(ctx, audit.EventStorageDelete, audit.StateDenied, "batch_delete", "", nil)
		return nil, err
	}
	// 受保护前缀整单拒绝而非逐条跳过：避免「其余已删、受保护条目失败」的
	// 部分执行状态，调用方可以拆单重试。
	for _, rawPath := range req.Paths {
		if err := guardWritablePath(normalizeStoragePath(rawPath)); err != nil {
			return nil, err
		}
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
	// AUD-049：批量删除逐条成功/失败计数一并留痕（失败条目见各 store.Delete 日志）。
	s.auditStorage(ctx, audit.EventStorageDelete, audit.StateSuccess, "batch_delete", "",
		map[string]interface{}{"deleted": len(resp.Deleted), "failed": len(resp.Failed)})
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
	if err := s.requireStoragePermission(ctx); err != nil {
		s.auditStorage(ctx, audit.EventStorageDirectory, audit.StateDenied, "directory_create", prefix, nil)
		return nil, err
	}
	if err := guardWritablePath(prefix); err != nil {
		return nil, err
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	if err := store.CreatePrefix(ctx, prefix); err != nil {
		slog.ErrorContext(ctx, "创建目录失败", "prefix", prefix, "error", err)
		return nil, errorx.NewInternalError(fmt.Sprintf("创建目录失败: %v", err))
	}

	slog.InfoContext(ctx, "创建目录成功", "prefix", prefix)
	s.auditStorage(ctx, audit.EventStorageDirectory, audit.StateSuccess, "directory_create", prefix, nil)
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
	if err := s.requireStoragePermission(ctx); err != nil {
		s.auditStorage(ctx, audit.EventStorageDirectory, audit.StateDenied, "directory_rename", oldPrefix, nil)
		return nil, err
	}
	// 重命名双向边界：移出受保护前缀同样改变平台命名空间布局。
	if err := guardWritablePath(oldPrefix); err != nil {
		return nil, err
	}
	if err := guardWritablePath(newPrefix); err != nil {
		return nil, err
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
	s.auditStorage(ctx, audit.EventStorageDirectory, audit.StateSuccess, "directory_rename", oldPrefix, nil)
	return &RenameDirectoryResponse{
		OldPrefix: oldPrefix,
		NewPrefix: newPrefix,
	}, nil
}
