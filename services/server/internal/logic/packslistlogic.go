// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PacksListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPacksListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksListLogic {
	return &PacksListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksListLogic) PacksList() (*types.PacksListResponse, error) {
	packDir := l.svcCtx.PackDir()
	if packDir == "" {
		return nil, errors.New("pack directory not configured")
	}
	manifestPath := filepath.Join(packDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest interface{}
	_ = json.Unmarshal(data, &manifest)
	counts := map[string]int{"descriptors": 0, "ui_schema": 0}
	_ = filepath.Walk(filepath.Join(packDir, "descriptors"), func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && filepath.Ext(path) == ".json" {
			counts["descriptors"]++
		}
		return nil
	})
	_ = filepath.Walk(filepath.Join(packDir, "ui"), func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && filepath.Ext(path) == ".json" {
			counts["ui_schema"]++
		}
		return nil
	})
	resp := &types.PacksListResponse{
		Manifest:           manifest,
		Counts:             counts,
		Etag:               computePackETag(packDir),
		ExportAuthRequired: false,
	}
	return resp, nil
}
