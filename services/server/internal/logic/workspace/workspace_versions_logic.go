// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"gorm.io/gorm"
)

type WorkspaceVersionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Workspace 版本列表
func NewWorkspaceVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceVersionsLogic {
	return &WorkspaceVersionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceVersionsLogic) WorkspaceVersions(req *types.WorkspaceVersionsRequest) (resp *types.WorkspaceVersionsResponse, err error) {
	maps, err := l.List(req.ObjectKey, req.From, req.To)
	if err != nil {
		return nil, err
	}

	// Convert []map[string]interface{} to []WorkspaceVersionRecord
	records := make([]types.WorkspaceVersionRecord, 0, len(maps))
	for _, m := range maps {
		records = append(records, types.WorkspaceVersionRecord{
			Id:                 fmt.Sprintf("%v", m["id"]),
			ObjectKey:          fmt.Sprintf("%v", m["objectKey"]),
			Version:            int(m["version"].(int64)),
			Config:             m["config"],
			IsCurrentDraft:     m["is_current_draft"].(bool),
			IsCurrentPublished: m["is_current_published"].(bool),
			CreatedAt:          fmt.Sprintf("%v", m["created_at"]),
			CreatedBy:          fmt.Sprintf("%v", m["created_by"]),
			Comment:            fmt.Sprintf("%v", m["comment"]),
		})
	}

	return &types.WorkspaceVersionsResponse{
		Items: records,
	}, nil
}

func (l *WorkspaceVersionsLogic) List(objectKey, from, to string) ([]map[string]interface{}, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	fromAt, toAt, err := parseWorkspaceVersionTimeRange(from, to)
	if err != nil {
		return nil, err
	}
	versionKey := workspaceVersionKey(objectKey)
	records, err := l.svcCtx.ConfigVersionModel.List(l.ctx, versionKey)
	if err != nil {
		return nil, err
	}
	currentDraftVersion := 0
	if latest, latestErr := l.svcCtx.ConfigVersionModel.FindLatest(l.ctx, versionKey); latestErr == nil {
		currentDraftVersion = latest.Version
	} else if latestErr != nil && !errors.Is(latestErr, gorm.ErrRecordNotFound) {
		return nil, latestErr
	}

	currentPublishedVersion := 0
	if latestCfg, cfgErr := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, objectKey); cfgErr == nil {
		if latestCfg.Published && latestCfg.PublishedAt != nil {
			// Find the version that matches the published_at timestamp
			publishedTime := latestCfg.PublishedAt.Unix()
			for _, rec := range records {
				if rec.CreatedAt.Unix() == publishedTime {
					currentPublishedVersion = rec.Version
					break
				}
			}
		}
	}

	result := make([]map[string]interface{}, 0, len(records))
	for _, record := range records {
		// Apply time range filter if specified
		if !fromAt.IsZero() && record.CreatedAt.Before(fromAt) {
			continue
		}
		if !toAt.IsZero() && record.CreatedAt.After(toAt) {
			continue
		}

		var configData interface{}
		if record.Value != "" {
			if err := json.Unmarshal([]byte(record.Value), &configData); err != nil {
				configData = record.Value // fallback to raw string
			}
		}

		result = append(result, map[string]interface{}{
			"id":                   strconv.FormatUint(uint64(record.ID), 10),
			"objectKey":            record.Key,
			"version":              record.Version,
			"config":               configData,
			"is_current_draft":     record.Version == currentDraftVersion,
			"is_current_published": record.Version == currentPublishedVersion,
			"created_at":           record.CreatedAt.Format(time.RFC3339),
			"created_by":           record.CreatedBy,
			"comment":              record.Message,
		})
	}

	return result, nil
}
