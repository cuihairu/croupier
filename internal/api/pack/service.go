package pack

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns the list of packs
func (s *Service) List(ctx context.Context, req *PacksListRequest) (*PacksListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看功能包列表", "admin:all", "packs:list", "packs:read", "packs:reload"); err != nil {
		return nil, err
	}

	packsDir := resolvePacksDir(s.svcCtx.Config)
	summaries, err := loadPackSummaries(packsDir)
	if err != nil {
		return nil, err
	}

	manifest, packEntries := aggregateManifest(summaries)
	counts := map[string]int{
		"descriptors": 0,
		"ui_schema":   0,
	}
	for _, summary := range summaries {
		counts["descriptors"] += summary.DescriptorCount
		counts["ui_schema"] += summary.UISchemaCount
	}

	manifestBytes, _ := json.Marshal(manifest)
	etag := ""
	if len(manifestBytes) > 0 {
		sum := sha256.Sum256(manifestBytes)
		etag = fmt.Sprintf("%x", sum[:])
	}

	return &PacksListResponse{
		Manifest:           manifest,
		Packs:              packEntries,
		Counts:             counts,
		ETag:               etag,
		ExportAuthRequired: false,
	}, nil
}

// Export exports packs as an archive
func (s *Service) Export(ctx context.Context, req *PacksExportRequest) (*PacksExportResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权导出功能包", "admin:all", "packs:read", "packs:list", "packs:reload"); err != nil {
		return nil, err
	}

	packsDir := resolvePacksDir(s.svcCtx.Config)
	filename, data, err := buildPacksArchive(packsDir)
	if err != nil {
		return nil, err
	}

	return &PacksExportResponse{
		Filename:    filename,
		ContentType: "application/gzip",
		Content:     data,
	}, nil
}

// Import imports a pack archive
func (s *Service) Import(ctx context.Context, req *PacksImportRequest) (*PacksImportResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权导入功能包", "admin:all", "packs:reload"); err != nil {
		return nil, err
	}

	if req == nil || req.Archive == "" {
		return nil, errors.New("archive payload is required")
	}

	data, err := base64.StdEncoding.DecodeString(req.Archive)
	if err != nil {
		return nil, err
	}

	packsDir := resolvePacksDir(s.svcCtx.Config)
	destDir := filepath.Join(packsDir, "dist")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}

	if err := extractArchive(data, destDir); err != nil {
		return nil, err
	}

	return &PacksImportResponse{
		Code:    0,
		Message: "Imported",
	}, nil
}

// Reload reloads packs from disk
func (s *Service) Reload(ctx context.Context, req *PacksReloadRequest) (*PacksReloadResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权重新加载功能包", "admin:all", "packs:reload"); err != nil {
		return nil, err
	}

	packsDir := resolvePacksDir(s.svcCtx.Config)
	_, err := loadPackSummaries(packsDir)
	if err != nil {
		return nil, err
	}

	return &PacksReloadResponse{
		OK:        true,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// Plugin returns the web plugin content for a pack
func (s *Service) Plugin(ctx context.Context, req *PacksPluginRequest) (*PacksPluginResponse, error) {
	// This is not implemented in the original code
	return nil, errors.New("PacksPlugin not implemented")
}
