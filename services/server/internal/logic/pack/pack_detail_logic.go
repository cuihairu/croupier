// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PackDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPackDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PackDetailLogic {
	return &PackDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PackDetailLogic) PackDetail(req *types.PackDetailRequest) (*types.PackDetailResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看功能包详情", "admin:all", "packs:read", "packs:detail"); err != nil {
		return nil, err
	}

	packsDir := resolvePacksDir(l.svcCtx.Config)
	packPath := filepath.Join(packsDir, req.ID)

	// Check if pack exists
	if _, err := os.Stat(packPath); err != nil || os.IsNotExist(err) {
		return nil, errorx.NewNotFound("功能包不存在")
	}

	// Read manifest
	manifestPath := filepath.Join(packPath, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, errorx.NewInternalError("读取 manifest 失败: " + err.Error())
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, errorx.NewInternalError("解析 manifest 失败: " + err.Error())
	}

	// Get file info
	fileInfo, _ := os.Stat(manifestPath)

	// Count descriptors and UI schemas
	descriptorsPath := filepath.Join(packPath, "descriptors")
	uiPath := filepath.Join(packPath, "ui")

	descriptorCount := 0
	uiSchemaCount := 0

	if files, err := os.ReadDir(descriptorsPath); err == nil {
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".json" {
				descriptorCount++
			}
		}
	}

	if files, err := os.ReadDir(uiPath); err == nil {
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".json" {
				uiSchemaCount++
			}
		}
	}

	// Get canary info if available
	canaryInfo := map[string]interface{}{}
	canaryPath := filepath.Join(packPath, "canary.json")
	if data, err := os.ReadFile(canaryPath); err == nil {
		_ = json.Unmarshal(data, &canaryInfo)
	}

	// Build response
	packDetail := map[string]interface{}{
		"id":               req.ID,
		"manifest":         manifest,
		"descriptor_count": descriptorCount,
		"ui_schema_count":  uiSchemaCount,
		"updated_at":       utils.FormatTimestamp(fileInfo.ModTime()),
		"canary":           canaryInfo,
	}

	return &types.PackDetailResponse{
		Pack: packDetail,
	}, nil
}

type PackContentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPackContentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PackContentsLogic {
	return &PackContentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PackContentsLogic) PackContents(req *types.PackContentsRequest) (*types.PackContentsResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看功能包内容", "admin:all", "packs:read", "packs:contents"); err != nil {
		return nil, err
	}

	packsDir := resolvePacksDir(l.svcCtx.Config)
	packPath := filepath.Join(packsDir, req.ID)

	// Check if pack exists
	if _, err := os.Stat(packPath); err != nil || os.IsNotExist(err) {
		return nil, errorx.NewNotFound("功能包不存在")
	}

	contents := make([]types.PackContentItem, 0)

	// Walk pack directory
	_ = filepath.WalkDir(packPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && path != packPath {
			// Skip directories but continue walking
			return nil
		}
		if d.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(packPath, path)
		info, _ := d.Info()

		item := types.PackContentItem{
			Path:     relPath,
			Size:     info.Size(),
			Modified: info.ModTime().Format(time.RFC3339),
		}

		// Determine type
		switch filepath.Dir(relPath) {
		case "descriptors":
			item.Type = "descriptor"
		case "ui":
			item.Type = "ui_schema"
		case "schemas":
			item.Type = "schema"
		default:
			item.Type = "other"
		}

		contents = append(contents, item)
		return nil
	})

	return &types.PackContentsResponse{
		ID:       req.ID,
		Contents: contents,
	}, nil
}

type PackVersionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPackVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PackVersionsLogic {
	return &PackVersionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PackVersionsLogic) PackVersions(req *types.PackVersionsRequest) (*types.PackVersionsResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看功能包版本", "admin:all", "packs:read", "packs:versions"); err != nil {
		return nil, err
	}

	packsDir := resolvePacksDir(l.svcCtx.Config)
	packPath := filepath.Join(packsDir, req.ID)

	// Check if pack exists
	fileInfo, err := os.Stat(packPath)
	if err != nil || os.IsNotExist(err) {
		return nil, errorx.NewNotFound("功能包不存在")
	}

	// Get current version info from manifest
	manifestPath := filepath.Join(packPath, "manifest.json")
	manifestData, _ := os.ReadFile(manifestPath)
	var manifest map[string]interface{}
	_ = json.Unmarshal(manifestData, &manifest)

	currentVersion := ""
	if v, ok := manifest["version"].(string); ok {
		currentVersion = v
	}

	// For now, return just the current version
	// In a full implementation, you might track versions in git or a version history file
	versions := []types.PackVersionItem{
		{
			Version:   currentVersion,
			CreatedAt: fileInfo.ModTime().Format(time.RFC3339),
			IsActive:  true,
		},
	}

	return &types.PackVersionsResponse{
		ID:       req.ID,
		Versions: versions,
	}, nil
}
